package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/lexical"
	"github.com/sirerun/gist/hosted/internal/ports"
)

var (
	ErrNotFound          = errors.New("registry record not found")
	ErrConflict          = errors.New("registry version conflict")
	ErrPolicyUnavailable = errors.New("registry policy unavailable")
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) (*Postgres, error) {
	if pool == nil {
		return nil, errors.New("storage: nil postgres pool")
	}
	return &Postgres{pool: pool}, nil
}

func (s *Postgres) Pool() *pgxpool.Pool { return s.pool }

func (s *Postgres) Get(ctx context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	if err := validateRef(ref); err != nil {
		return ports.CatalogRecord{}, err
	}
	var record ports.CatalogRecord
	var metadata []byte
	var state, da, dv, mda, mdv string
	err := WithTenant(ctx, s.pool, ref.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT state, digest_algorithm, digest_value, manifest_digest_algorithm, manifest_digest_value, metadata FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&state, &da, &dv, &mda, &mdv, &metadata)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.CatalogRecord{}, ErrNotFound
	}
	if err != nil {
		return ports.CatalogRecord{}, fmt.Errorf("get catalog record: %w", err)
	}
	record.Ref, record.State = ref, state
	record.Digest = ports.Digest{Algorithm: da, Value: dv}
	record.ManifestDigest = ports.Digest{Algorithm: mda, Value: mdv}
	record.Metadata = append([]byte(nil), metadata...)
	return record, nil
}

// Search generates lexical discovery candidates inside the principal's
// workspace. The read runs under WithTenant, so row-level security confines
// both the scanned rows and every term statistic to that workspace; revoked
// and draft versions never become candidates. With query text, every live
// record in the workspace (bounded by the catalog-entry limit) is scored with
// the shared lexical model, non-matching records are dropped, and the rest are
// ranked before the limit is applied, so a record can never be cut by an
// alphabetical LIMIT ahead of a better match. Ties break on kind, artifact ID
// and version, giving a total, deterministic order.
func (s *Postgres) Search(ctx context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	if q.Principal.WorkspaceID == "" {
		return ports.SearchPage{}, errors.New("storage: search principal has no workspace")
	}
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	terms := lexical.Query(q.Text)
	if strings.TrimSpace(q.Text) != "" && len(terms) == 0 {
		return ports.SearchPage{}, nil // stopwords only: nothing to match, never forced similarity
	}
	args := []any{q.Principal.WorkspaceID}
	where := []string{"workspace_id=$1", "state IN ('published','deprecated')"}
	if len(q.Kinds) > 0 {
		vals := make([]string, len(q.Kinds))
		for i, kind := range q.Kinds {
			vals[i] = string(kind)
		}
		args = append(args, vals)
		where = append(where, "kind = ANY($"+fmt.Sprint(len(args))+"::text[])")
	}
	scan := limit
	if len(terms) > 0 {
		scan = maxSearchScan
	}
	args = append(args, scan)
	query := `SELECT kind, artifact_id, version, state, digest_algorithm, digest_value, manifest_digest_algorithm, manifest_digest_value, metadata FROM catalog_versions WHERE ` + strings.Join(where, " AND ") + ` ORDER BY kind, artifact_id, version LIMIT $` + fmt.Sprint(len(args))
	type ranked struct {
		record ports.CatalogRecord
		match  lexical.Match
	}
	var hits []ranked
	err := WithTenant(ctx, s.pool, q.Principal.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var kind, id, version, state, da, dv, mda, mdv string
			var metadata []byte
			if err := rows.Scan(&kind, &id, &version, &state, &da, &dv, &mda, &mdv, &metadata); err != nil {
				return err
			}
			record := ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: q.Principal.WorkspaceID, Kind: ports.ArtifactKind(kind), ID: id, Version: version}, State: state, Digest: ports.Digest{Algorithm: da, Value: dv}, ManifestDigest: ports.Digest{Algorithm: mda, Value: mdv}, Metadata: append([]byte(nil), metadata...)}
			var match lexical.Match
			if len(terms) > 0 {
				doc, err := lexical.DocumentFromMetadata(kind, id, metadata)
				if err != nil {
					return fmt.Errorf("decode search metadata %s: %w", id, err)
				}
				if match = lexical.NewIndex(doc).Score(terms); !match.Matched() {
					continue
				}
			}
			hits = append(hits, ranked{record: record, match: match})
		}
		return rows.Err()
	})
	if err != nil {
		return ports.SearchPage{}, fmt.Errorf("search catalog: %w", err)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if c := hits[i].match.Better(hits[j].match); c != 0 {
			return c > 0
		}
		return refLess(hits[i].record.Ref, hits[j].record.Ref)
	})
	page := ports.SearchPage{}
	for i := 0; i < len(hits) && i < limit; i++ {
		page.Records = append(page.Records, hits[i].record)
	}
	return page, nil
}

// maxSearchScan bounds the rows one text search scores, so a single query
// has a fixed worst-case cost. A workspace's live catalog up to this size is
// ranked in full rather than as a lexicographic prefix.
const maxSearchScan = 10000

func refLess(a, b ports.ArtifactRef) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	return a.Version < b.Version
}

// ListVersions returns one page of the published or deprecated versions of
// one artifact in the reference's workspace, in SemVer precedence order
// (registry_semver_key, migration 006; ties on build metadata break on the
// version text). after is the last version of the previous page, or "" for
// the first page, and limit caps the page. It is not capped by the
// catalog-wide search limit, so an artifact's versions are never truncated by
// unrelated records.
func (s *Postgres) ListVersions(ctx context.Context, ref ports.ArtifactRef, after string, limit int) ([]ports.CatalogRecord, error) {
	if ref.WorkspaceID == "" || ref.Kind == "" || ref.ID == "" {
		return nil, errors.New("storage: incomplete artifact reference")
	}
	if limit <= 0 {
		return nil, errors.New("storage: version page limit must be positive")
	}
	var out []ports.CatalogRecord
	err := WithTenant(ctx, s.pool, ref.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT version, state, digest_algorithm, digest_value, manifest_digest_algorithm, manifest_digest_value, metadata FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND state IN ('published','deprecated') AND ($4 = '' OR (version_key, version COLLATE "C") > (registry_semver_key($4), $4 COLLATE "C")) ORDER BY version_key, version COLLATE "C" LIMIT $5`, ref.WorkspaceID, ref.Kind, ref.ID, after, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var version, state, da, dv, mda, mdv string
			var metadata []byte
			if err := rows.Scan(&version, &state, &da, &dv, &mda, &mdv, &metadata); err != nil {
				return err
			}
			out = append(out, ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: ref.WorkspaceID, Kind: ref.Kind, ID: ref.ID, Version: version}, State: state, Digest: ports.Digest{Algorithm: da, Value: dv}, ManifestDigest: ports.Digest{Algorithm: mda, Value: mdv}, Metadata: append([]byte(nil), metadata...)})
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list artifact versions: %w", err)
	}
	return out, nil
}

// Lookup reads an unrevoked workload identity. workload_identities is under
// FORCE ROW LEVEL SECURITY keyed on registry.workspace_id, so the read runs
// inside WithTenant for the caller's workspace; without that scope every row is
// invisible and the lookup always reports ErrNotFound.
func (s *Postgres) Lookup(ctx context.Context, workspaceID, issuer, subject string) (ports.IdentityRecord, error) {
	if err := validateIdentityKey(workspaceID, issuer, subject); err != nil {
		return ports.IdentityRecord{}, err
	}
	var r ports.IdentityRecord
	var scopes []string
	err := WithTenant(ctx, s.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT issuer, subject, workspace_id, subject_type, scopes, policy_generation, extract(epoch from expires_at)::bigint FROM workload_identities WHERE workspace_id=$1 AND issuer=$2 AND subject=$3 AND revoked_at IS NULL`, workspaceID, issuer, subject).Scan(&r.Issuer, &r.Subject, &r.WorkspaceID, &r.SubjectType, &scopes, &r.PolicyGeneration, &r.ExpiresAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.IdentityRecord{}, ErrNotFound
	}
	if err != nil {
		return ports.IdentityRecord{}, fmt.Errorf("lookup identity: %w", err)
	}
	r.Scopes = append([]string(nil), scopes...)
	return r, nil
}

// Revoke marks a workload identity revoked. Like Lookup it must run under the
// workspace's RLS scope, or the UPDATE matches zero rows.
func (s *Postgres) Revoke(ctx context.Context, workspaceID, issuer, subject string) error {
	if err := validateIdentityKey(workspaceID, issuer, subject); err != nil {
		return err
	}
	var affected int64
	err := WithTenant(ctx, s.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE workload_identities SET revoked_at=COALESCE(revoked_at, now()) WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, workspaceID, issuer, subject)
		affected = command.RowsAffected()
		return err
	})
	if err != nil {
		return fmt.Errorf("revoke identity: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrIdentityRevoked reports that an issuance was refused because the stored
// identity for that workspace, issuer and subject has been revoked.
var ErrIdentityRevoked = errors.New("registry identity revoked")

// RecordIssued persists or refreshes the stored identity behind a freshly
// minted workload token, under the token workspace's RLS scope. The upsert
// never clears revoked_at: when the existing row is revoked the conflict
// update matches nothing and RecordIssued returns ErrIdentityRevoked, so the
// caller must discard the token. expires_at only moves forward, so a
// shorter-lived mint never shortens the record of an outstanding token.
//
// Revocation is permanent for the (workspace, issuer, subject) key: once
// revoked, that subject can never be minted again in that workspace, even
// after its old tokens expire. This is the deliberate safe default (ADR 005
// amendment, 2026-09-26); there is no reinstatement API. A retired or
// compromised workload that needs access again gets a new subject.
func (s *Postgres) RecordIssued(ctx context.Context, r ports.IdentityRecord) error {
	if err := validateIdentityKey(r.WorkspaceID, r.Issuer, r.Subject); err != nil {
		return err
	}
	if r.SubjectType == "" || r.ExpiresAt <= 0 {
		return errors.New("storage: incomplete identity record")
	}
	scopes := append([]string{}, r.Scopes...)
	tenant := Tenant{Issuer: r.Issuer, Subject: r.Subject, WorkspaceID: r.WorkspaceID, Scopes: scopes, PolicyGeneration: r.PolicyGeneration}
	var affected int64
	err := WithTenantPrincipal(ctx, s.pool, tenant, func(ctx context.Context, tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `INSERT INTO workload_identities(issuer,subject,subject_type,workspace_id,scopes,policy_generation,expires_at)
VALUES($1,$2,$3,$4,$5,$6,to_timestamp($7))
ON CONFLICT (workspace_id,issuer,subject) DO UPDATE SET
    subject_type=EXCLUDED.subject_type,
    scopes=EXCLUDED.scopes,
    policy_generation=EXCLUDED.policy_generation,
    expires_at=GREATEST(workload_identities.expires_at, EXCLUDED.expires_at)
WHERE workload_identities.revoked_at IS NULL`, r.Issuer, r.Subject, r.SubjectType, r.WorkspaceID, scopes, int64(r.PolicyGeneration), r.ExpiresAt)
		affected = command.RowsAffected()
		return err
	})
	if err != nil {
		return fmt.Errorf("record issued identity: %w", err)
	}
	if affected == 0 {
		return ErrIdentityRevoked
	}
	return nil
}

func validateIdentityKey(workspaceID, issuer, subject string) error {
	if workspaceID == "" || issuer == "" || subject == "" {
		return errors.New("storage: incomplete identity key")
	}
	return nil
}

func validateRef(ref ports.ArtifactRef) error {
	if ref.WorkspaceID == "" || ref.Kind == "" || ref.ID == "" || ref.Version == "" {
		return errors.New("storage: incomplete artifact reference")
	}
	return nil
}

var _ ports.CatalogStore = (*Postgres)(nil)
var _ io.Reader
