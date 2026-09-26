package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/packages"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

type postgresPolicy struct{ pool *pgxpool.Pool }

func (p *postgresPolicy) CheckWorkload(ctx context.Context, subject, workspace string, scopes []string, generation uint64) (identity.Policy, error) {
	var got identity.Policy
	var current uint64
	if p == nil || p.pool == nil {
		return got, errors.New("policy database unavailable")
	}
	err := storage.WithTenant(ctx, p.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT wm.policy_generation, wm.scopes, wm.active, w.policy_generation FROM workspace_memberships wm JOIN workspaces w ON w.id=wm.workspace_id WHERE wm.subject=$1 AND wm.workspace_id=$2 ORDER BY wm.created_at DESC LIMIT 1`, subject, workspace).Scan(&got.PolicyGeneration, &got.ParentScopes, &got.Allowed, &current)
	})
	if err != nil {
		return identity.Policy{}, fmt.Errorf("lookup workload policy: %w", err)
	}
	got.Allowed = got.Allowed && got.PolicyGeneration == current && containsScopes(got.ParentScopes, scopes)
	if generation != 0 && generation != got.PolicyGeneration {
		got.Allowed = false
	}
	return got, nil
}

func (p *postgresPolicy) CheckWorkloadForIssuer(ctx context.Context, issuer, subject, workspace string, scopes []string, generation uint64) (identity.Policy, error) {
	var got identity.Policy
	var current uint64
	err := storage.WithTenant(ctx, p.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT wm.policy_generation, wm.scopes, wm.active, w.policy_generation FROM workspace_memberships wm JOIN workspaces w ON w.id=wm.workspace_id WHERE wm.subject=$1 AND wm.workspace_id=$2 AND wm.issuer=$3`, subject, workspace, issuer).Scan(&got.PolicyGeneration, &got.ParentScopes, &got.Allowed, &current)
	})
	if err != nil {
		return identity.Policy{}, fmt.Errorf("lookup workload policy: %w", err)
	}
	got.Allowed = got.Allowed && got.PolicyGeneration == current && containsScopes(got.ParentScopes, scopes)
	if generation != 0 && generation != got.PolicyGeneration {
		got.Allowed = false
	}
	return got, nil
}

func (p *postgresPolicy) Decide(ctx context.Context, principal ports.Principal, action ports.Action, ref *ports.ArtifactRef) (ports.Decision, error) {
	if p == nil || p.pool == nil {
		return ports.Decision{Status: 503}, errors.New("policy database unavailable")
	}
	if principal.WorkspaceID == "" || principal.Subject == "" {
		return ports.Decision{Status: 401}, errors.New("invalid principal")
	}
	if ref != nil && ref.WorkspaceID != "" && ref.WorkspaceID != principal.WorkspaceID {
		return ports.Decision{Status: 404}, nil
	}
	var role string
	var active bool
	var generation uint64
	err := storage.WithTenant(ctx, p.pool, principal.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT role, active, policy_generation FROM workspace_memberships WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, principal.WorkspaceID, principal.Issuer, principal.Subject).Scan(&role, &active, &generation)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Decision{Status: 404}, nil
	}
	if err != nil {
		return ports.Decision{Status: 503}, fmt.Errorf("read authorization policy: %w", err)
	}
	if !active || generation != principal.PolicyGeneration {
		return ports.Decision{Status: 404}, nil
	}
	for _, scope := range principal.Scopes {
		if scope == string(action) {
			if action == ports.ActionPublish && role != "maintainer" {
				return ports.Decision{Status: 403, Reason: "maintainer role required"}, nil
			}
			return ports.Decision{Allowed: true, Status: 200}, nil
		}
	}
	if action == ports.ActionPublish {
		return ports.Decision{Status: 403, Reason: "publish scope or maintainer role required"}, nil
	}
	return ports.Decision{Status: 403, Reason: "scope required"}, nil
}

func containsScopes(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, s := range have {
		set[s] = true
	}
	for _, s := range want {
		if !set[s] {
			return false
		}
	}
	return true
}

type postgresResolutionStore struct{ pool *pgxpool.Pool }

func (s *postgresResolutionStore) Put(ctx context.Context, r ports.Resolution) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return storage.WithTenantPrincipal(ctx, s.pool, storage.Tenant{Issuer: r.Principal.Issuer, Subject: r.Principal.Subject, Audience: r.Principal.Audience, WorkspaceID: r.Principal.WorkspaceID, Scopes: r.Principal.Scopes, PolicyGeneration: r.Principal.PolicyGeneration}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO resolutions (id,workspace_id,issuer,subject,audience,principal_hash,policy_generation,skill_id,skill_version,expires_at,payload) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,to_timestamp($10),$11)`, r.ID, r.Principal.WorkspaceID, r.Principal.Issuer, r.Principal.Subject, r.Principal.Audience, r.Principal.Subject+"\x00"+r.Principal.WorkspaceID, r.Principal.PolicyGeneration, r.Skill.ID, r.Skill.Version, r.ExpiresAt, payload)
		return err
	})
}
func (s *postgresResolutionStore) Get(ctx context.Context, c ports.Cursor) (ports.Resolution, error) {
	var raw []byte
	err := storage.WithTenant(ctx, s.pool, c.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT payload FROM resolutions WHERE id=$1 AND workspace_id=$2 AND expires_at>now()`, c.ID, c.WorkspaceID).Scan(&raw)
	})
	if err != nil {
		return ports.Resolution{}, err
	}
	var r ports.Resolution
	return r, json.Unmarshal(raw, &r)
}

type publisher struct {
	pool    *pgxpool.Pool
	objects *storage.ObjectStore
	limits  Config
}

func (p publisher) Publish(ctx context.Context, principal ports.Principal, kind ports.ArtifactKind, raw []byte) ([]byte, error) {
	archive, err := packages.ReadArchive(raw, packages.Limits{MaxPackageBytes: int(p.limits.MaxPackageBytes), MaxFileBytes: int(p.limits.MaxExpandedBytes), MaxFiles: 256})
	if err != nil {
		return nil, fmt.Errorf("read package: %w", err)
	}
	pack, err := packages.Validate(archive, packages.DefaultLimits())
	if err != nil {
		return nil, err
	}
	review, err := packagesReview(pack, principal.Subject)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	digest := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	ref := ports.ArtifactRef{WorkspaceID: principal.WorkspaceID, Kind: kind, ID: pack.Manifest.ID, Version: pack.Manifest.Version}
	if err := p.objects.Put(ctx, digest, bytesReader(raw), int64(len(raw))); err != nil {
		return nil, err
	}
	err = storage.WithTenantPrincipal(ctx, p.pool, storage.Tenant{Issuer: principal.Issuer, Subject: principal.Subject, Audience: principal.Audience, WorkspaceID: principal.WorkspaceID, Scopes: principal.Scopes, PolicyGeneration: principal.PolicyGeneration}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO catalog_versions (workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,package_key,owner_id) VALUES ($1,$2,$3,$4,'published',$5,$6,'sha256',$7,$8,$6,$9)`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version, digest.Algorithm, digest.Value, pack.ManifestDigest, pack.ManifestBytes, principal.Subject)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := p.objects.Bind(ref, digest); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"ref": ref, "reviewer": review})
}
func packagesReview(p packages.Package, reviewer string) (string, error) {
	if err := packages.ScreenText(p); err != nil {
		return "", err
	}
	if reviewer == "" {
		return "", errors.New("publisher: reviewer is required")
	}
	return reviewer, nil
}

type byteReader struct{ r io.Reader }

func bytesReader(b []byte) io.Reader             { return &byteReader{r: bytes.NewReader(b)} }
func (r *byteReader) Read(p []byte) (int, error) { return r.r.Read(p) }

var _ ports.Authorizer = (*postgresPolicy)(nil)
var _ ports.ResolutionStore = (*postgresResolutionStore)(nil)
