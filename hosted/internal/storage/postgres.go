package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func (s *Postgres) Search(ctx context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	if q.Principal.WorkspaceID == "" {
		return ports.SearchPage{}, errors.New("storage: search principal has no workspace")
	}
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	args := []any{q.Principal.WorkspaceID, limit}
	where := []string{"workspace_id=$1", "state IN ('published','deprecated')"}
	if q.Text != "" {
		args = append(args, "%"+strings.ToLower(q.Text)+"%")
		where = append(where, "lower(artifact_id) LIKE $"+fmt.Sprint(len(args)))
	}
	if len(q.Kinds) > 0 {
		vals := make([]string, len(q.Kinds))
		for i, kind := range q.Kinds {
			vals[i] = string(kind)
		}
		args = append(args, vals)
		where = append(where, "kind = ANY($"+fmt.Sprint(len(args))+"::text[])")
	}
	query := `SELECT kind, artifact_id, version, state, digest_algorithm, digest_value, manifest_digest_algorithm, manifest_digest_value, metadata FROM catalog_versions WHERE ` + strings.Join(where, " AND ") + ` ORDER BY artifact_id, version LIMIT $2`
	page := ports.SearchPage{}
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
			page.Records = append(page.Records, ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: q.Principal.WorkspaceID, Kind: ports.ArtifactKind(kind), ID: id, Version: version}, State: state, Digest: ports.Digest{Algorithm: da, Value: dv}, ManifestDigest: ports.Digest{Algorithm: mda, Value: mdv}, Metadata: append([]byte(nil), metadata...)})
		}
		return rows.Err()
	})
	if err != nil {
		return ports.SearchPage{}, fmt.Errorf("search catalog: %w", err)
	}
	return page, nil
}

func (s *Postgres) Lookup(ctx context.Context, issuer, subject string) (ports.IdentityRecord, error) {
	var r ports.IdentityRecord
	var scopes []string
	err := s.pool.QueryRow(ctx, `SELECT issuer, subject, workspace_id, subject_type, scopes, policy_generation, extract(epoch from expires_at)::bigint FROM workload_identities WHERE issuer=$1 AND subject=$2 AND revoked_at IS NULL`, issuer, subject).Scan(&r.Issuer, &r.Subject, &r.WorkspaceID, &r.SubjectType, &scopes, &r.PolicyGeneration, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.IdentityRecord{}, ErrNotFound
	}
	if err != nil {
		return ports.IdentityRecord{}, fmt.Errorf("lookup identity: %w", err)
	}
	r.Scopes = append([]string(nil), scopes...)
	return r, nil
}

func (s *Postgres) Revoke(ctx context.Context, issuer, subject string) error {
	command, err := s.pool.Exec(ctx, `UPDATE workload_identities SET revoked_at=now() WHERE issuer=$1 AND subject=$2`, issuer, subject)
	if err != nil {
		return fmt.Errorf("revoke identity: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func validateRef(ref ports.ArtifactRef) error {
	if ref.WorkspaceID == "" || ref.Kind == "" || ref.ID == "" || ref.Version == "" {
		return errors.New("storage: incomplete artifact reference")
	}
	return nil
}
func decodeMetadata(raw []byte) (map[string]any, error) {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}
	return v, nil
}

var _ ports.CatalogStore = (*Postgres)(nil)
var _ ports.IdentityStore = (*Postgres)(nil)
var _ io.Reader
