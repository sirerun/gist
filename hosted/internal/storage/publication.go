package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// ErrPublicationBudget identifies bounded publication quota exhaustion.
var ErrPublicationBudget = errors.New("storage: publication quota exceeded")

// PublicationFence is executed inside every principal-bound publication
// transaction, including idempotent replay and reads.
type PublicationFence func(context.Context, pgx.Tx) error

type PublicationStore struct {
	pool    *pgxpool.Pool
	objects *ObjectStore
}

func NewPublicationStore(pool *pgxpool.Pool, objects *ObjectStore) (*PublicationStore, error) {
	if pool == nil || objects == nil {
		return nil, errors.New("storage: publication store requires pool and object store")
	}
	return &PublicationStore{pool: pool, objects: objects}, nil
}

func (s *PublicationStore) Publish(ctx context.Context, p ports.Principal, prepared ports.PreparedPublication, receipt []byte, fence PublicationFence) (ports.PublicationResult, error) {
	if s == nil || s.pool == nil || s.objects == nil || fence == nil {
		return ports.PublicationResult{}, errors.New("storage: publication fence and store required")
	}
	if p.WorkspaceID == "" || p.Issuer == "" || p.Subject == "" || prepared.Ref.Kind == "" || prepared.Ref.ID == "" || prepared.Ref.Version == "" || prepared.IdempotencyKey == "" || len(receipt) == 0 || len(prepared.Artifact) == 0 {
		return ports.PublicationResult{}, errors.New("storage: incomplete publication")
	}
	sum := sha256.Sum256(prepared.Artifact)
	artifactHex := hex.EncodeToString(sum[:])
	if prepared.ArtifactDigest.Algorithm != "sha256" || prepared.ArtifactDigest.Value != artifactHex {
		return ports.PublicationResult{}, errors.New("storage: prepared artifact digest mismatch")
	}
	if len(prepared.Metadata) == 0 {
		return ports.PublicationResult{}, errors.New("storage: publication metadata is empty")
	}
	var saved ports.PublicationReceipt
	if err := json.Unmarshal(receipt, &saved); err != nil {
		return ports.PublicationResult{}, fmt.Errorf("storage: invalid publication receipt: %w", err)
	}
	if saved.Kind != prepared.Ref.Kind || saved.ID != prepared.Ref.ID || saved.Version != prepared.Ref.Version || saved.ArtifactDigest != "sha256:"+artifactHex {
		return ports.PublicationResult{}, errors.New("storage: publication receipt identity mismatch")
	}
	if prepared.MaxBytes <= 0 || int64(len(receipt)) > prepared.MaxBytes {
		return ports.PublicationResult{}, ErrPublicationBudget
	}
	if saved.ManifestDigest != digestString(prepared.ManifestDigest) || saved.PackageDigest != digestString(prepared.PackageDigest) {
		return ports.PublicationResult{}, errors.New("storage: publication receipt digest mismatch")
	}
	metadataSum := sha256.Sum256(prepared.Metadata)
	metadataHex := hex.EncodeToString(metadataSum[:])
	if prepared.Ref.Kind == ports.KindSkill {
		if prepared.ManifestDigest.Algorithm != "sha256" || prepared.ManifestDigest.Value != metadataHex {
			return ports.PublicationResult{}, errors.New("storage: prepared manifest digest mismatch")
		}
	} else if prepared.DocumentDigest.Algorithm != "sha256" || prepared.DocumentDigest.Value != metadataHex {
		return ports.PublicationResult{}, errors.New("storage: prepared document digest mismatch")
	} else if !bytes.Equal(prepared.Artifact, prepared.Metadata) {
		return ports.PublicationResult{}, errors.New("storage: typed artifact and metadata bytes differ")
	}
	ref := prepared.Ref
	ref.WorkspaceID = p.WorkspaceID
	var attemptID, objectKey string
	var replay, alias bool
	err := WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		if err := fence(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, p.WorkspaceID+"/publication-quota"); err != nil {
			return err
		}
		// Serialize this key before checking permanent identity and limits.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, p.WorkspaceID+"/"+string(ref.Kind)+"/"+prepared.IdempotencyKey); err != nil {
			return err
		}
		var oldID, oldVersion, oldDigest string
		err := tx.QueryRow(ctx, `SELECT artifact_id,version,artifact_digest,attempt_id,response_body FROM publication_idempotency WHERE workspace_id=$1 AND kind=$2 AND idempotency_key=$3`, p.WorkspaceID, ref.Kind, prepared.IdempotencyKey).Scan(&oldID, &oldVersion, &oldDigest, &attemptID, &receipt)
		if err == nil {
			if oldID != ref.ID || oldVersion != ref.Version || oldDigest != artifactHex {
				return ErrConflict
			}
			var state string
			if err := tx.QueryRow(ctx, `SELECT state,object_key FROM publication_attempts WHERE workspace_id=$1 AND attempt_id=$2 FOR UPDATE`, p.WorkspaceID, attemptID).Scan(&state, &objectKey); err != nil {
				return err
			}
			if state == "deleting" {
				return ErrConflict
			}
			if state == "committed" || state == "retired" {
				var liveDigest, liveState, liveKey string
				catalogErr := tx.QueryRow(ctx, `SELECT digest_value,state,COALESCE(object_key,'') FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&liveDigest, &liveState, &liveKey)
				if state == "committed" || catalogErr == nil {
					if catalogErr != nil || liveDigest != artifactHex || liveKey != objectKey || (liveState != "published" && liveState != "deprecated") {
						return ErrConflict
					}
				} else if !errors.Is(catalogErr, pgx.ErrNoRows) {
					return catalogErr
				}
			}
			if state == "retired" {
				// The old physical key remains a cleanup tombstone. Reassign this
				// permanent identity to a fresh attempt/key only after deletion.
				var random [16]byte
				if _, err := rand.Read(random[:]); err != nil {
					return err
				}
				newID := hex.EncodeToString(random[:])
				newKey := "v2/" + newID + "/" + artifactHex
				if _, err := tx.Exec(ctx, `INSERT INTO publication_attempts(workspace_id,attempt_id,kind,artifact_id,version,idempotency_key,artifact_digest,object_key,state,lease_until,artifact_size) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'staged',clock_timestamp()+interval '24 hours',$9)`, p.WorkspaceID, newID, ref.Kind, ref.ID, ref.Version, prepared.IdempotencyKey, artifactHex, newKey, len(prepared.Artifact)); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE publication_idempotency SET attempt_id=$4,response_body=$5 WHERE workspace_id=$1 AND kind=$2 AND idempotency_key=$3`, p.WorkspaceID, ref.Kind, prepared.IdempotencyKey, newID, receipt); err != nil {
					return err
				}
				attemptID, objectKey, state = newID, newKey, "staged"
			}
			if state == "staged" {
				_, err = tx.Exec(ctx, `UPDATE publication_attempts SET lease_until=clock_timestamp()+interval '24 hours' WHERE workspace_id=$1 AND attempt_id=$2 AND state='staged'`, p.WorkspaceID, attemptID)
				if err != nil {
					return err
				}
			}
			replay = state == "committed"
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM publication_idempotency WHERE workspace_id=$1`, p.WorkspaceID).Scan(&count); err != nil {
			return err
		}
		if count >= 100000 {
			return ErrPublicationBudget
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1 AND state IN ('staged','deleting')`, p.WorkspaceID).Scan(&count); err != nil {
			return err
		}
		if count >= 64 {
			return ErrPublicationBudget
		}
		var existingDigest, existingState, existingKey string
		err = tx.QueryRow(ctx, `SELECT digest_value,state,COALESCE(object_key,'') FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 FOR UPDATE`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&existingDigest, &existingState, &existingKey)
		if err == nil && existingDigest != artifactHex {
			return ErrConflict
		}
		if err == nil && existingState == "revoked" {
			return ErrConflict
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		state := "staged"
		if existingDigest != "" {
			alias = true
			state = "committed"
			if err := tx.QueryRow(ctx, `SELECT attempt_id FROM publication_attempts WHERE workspace_id=$1 AND object_key=$2 AND state='committed'`, p.WorkspaceID, existingKey).Scan(&attemptID); err != nil {
				return ErrConflict
			}
			if err := tx.QueryRow(ctx, `SELECT publication_receipt FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&receipt); err != nil {
				return err
			}
		} else {
			var random [16]byte
			if _, err := rand.Read(random[:]); err != nil {
				return err
			}
			attemptID = hex.EncodeToString(random[:])
			objectKey = "v2/" + attemptID + "/" + artifactHex
		}
		if !alias {
			_, err = tx.Exec(ctx, `INSERT INTO publication_attempts(workspace_id,attempt_id,kind,artifact_id,version,idempotency_key,artifact_digest,object_key,state,lease_until,artifact_size) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,clock_timestamp()+interval '24 hours',$10)`, p.WorkspaceID, attemptID, ref.Kind, ref.ID, ref.Version, prepared.IdempotencyKey, artifactHex, objectKey, state, len(prepared.Artifact))
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO publication_idempotency(workspace_id,kind,idempotency_key,artifact_id,version,artifact_digest,attempt_id,response_body) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, p.WorkspaceID, ref.Kind, prepared.IdempotencyKey, ref.ID, ref.Version, artifactHex, attemptID, receipt)
		return err
	})
	if err != nil {
		return ports.PublicationResult{}, fmt.Errorf("reserve publication: %w", err)
	}
	if replay {
		var stored []byte
		if err := WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
			if err := fence(ctx, tx); err != nil {
				return err
			}
			return tx.QueryRow(ctx, `SELECT response_body FROM publication_idempotency WHERE workspace_id=$1 AND kind=$2 AND idempotency_key=$3`, p.WorkspaceID, ref.Kind, prepared.IdempotencyKey).Scan(&stored)
		}); err != nil {
			return ports.PublicationResult{}, err
		}
		if int64(len(stored)) > prepared.MaxBytes {
			return ports.PublicationResult{}, ErrPublicationBudget
		}
		var replayReceipt ports.PublicationReceipt
		if err := json.Unmarshal(stored, &replayReceipt); err != nil || replayReceipt.Kind != ref.Kind || replayReceipt.ID != ref.ID || replayReceipt.Version != ref.Version || replayReceipt.ArtifactDigest != "sha256:"+artifactHex {
			return ports.PublicationResult{}, errors.New("storage: stored publication receipt identity mismatch")
		}
		return publicationResult(prepared, stored, false), nil
	}
	if alias {
		return publicationResult(prepared, receipt, false), nil
	}
	if err := s.objects.StageOwned(ctx, objectKey, prepared.Artifact, prepared.ArtifactDigest); err != nil {
		return ports.PublicationResult{}, fmt.Errorf("stage publication object: %w", err)
	}
	var published bool
	err = WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		if err := fence(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, p.WorkspaceID+"/version/"+string(ref.Kind)+"/"+ref.ID+"/"+ref.Version); err != nil {
			return err
		}
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM publication_attempts WHERE workspace_id=$1 AND attempt_id=$2 FOR UPDATE`, p.WorkspaceID, attemptID).Scan(&state); err != nil {
			return err
		}
		if state == "committed" {
			var digest, liveState, liveKey string
			if err := tx.QueryRow(ctx, `SELECT digest_value,state,COALESCE(object_key,'') FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&digest, &liveState, &liveKey); err != nil {
				return err
			}
			if digest != artifactHex || liveKey != objectKey || (liveState != "published" && liveState != "deprecated") {
				return ErrConflict
			}
			alias = true
			return nil
		}
		if state != "staged" {
			return ErrConflict
		}
		var digest string
		getErr := tx.QueryRow(ctx, `SELECT digest_value,state FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 FOR UPDATE`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&digest, &state)
		if getErr == nil {
			if digest != artifactHex || state == "revoked" {
				return ErrConflict
			}
			var canonicalAttempt string
			if err := tx.QueryRow(ctx, `SELECT a.attempt_id FROM catalog_versions c JOIN publication_attempts a ON a.workspace_id=c.workspace_id AND a.object_key=c.object_key AND a.state='committed' WHERE c.workspace_id=$1 AND c.kind=$2 AND c.artifact_id=$3 AND c.version=$4`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&canonicalAttempt); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE publication_idempotency SET attempt_id=$3,response_body=(SELECT publication_receipt FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$4 AND version=$5) WHERE workspace_id=$1 AND kind=$2 AND idempotency_key=$6`, p.WorkspaceID, ref.Kind, canonicalAttempt, ref.ID, ref.Version, prepared.IdempotencyKey)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE publication_attempts SET state='retired',lease_until=clock_timestamp()+interval '24 hours' WHERE workspace_id=$1 AND attempt_id=$2`, p.WorkspaceID, attemptID)
			if err != nil {
				return err
			}
			alias = true
			return nil
		}
		if !errors.Is(getErr, pgx.ErrNoRows) {
			return getErr
		}
		manifestAlg, manifestValue := nullableDigest(prepared.ManifestDigest)
		documentAlg, documentValue := nullableDigest(prepared.DocumentDigest)
		packageAlg, packageValue := nullableDigest(prepared.PackageDigest)
		_, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,package_key,owner_id,document_digest_algorithm,document_digest_value,package_digest_algorithm,package_digest_value,artifact_bytes,metadata_bytes,object_key,publication_receipt,published_at) VALUES($1,$2,$3,$4,'published','sha256',$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,clock_timestamp())`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version, artifactHex, manifestAlg, manifestValue, prepared.Metadata, objectKey, p.Subject, documentAlg, documentValue, packageAlg, packageValue, prepared.Artifact, prepared.Metadata, objectKey, receipt)
		if err != nil {
			return err
		}
		// Taxonomy projection rights are trusted app evidence and are not part
		// of PreparedPublication. The final fence may write the qualified
		// projection in this same transaction; storage does not invent license.
		_, err = tx.Exec(ctx, `UPDATE publication_attempts SET state='committed' WHERE workspace_id=$1 AND attempt_id=$2`, p.WorkspaceID, attemptID)
		if err != nil {
			return err
		}
		return AppendEventInTransaction(ctx, tx, ports.Event{WorkspaceID: p.WorkspaceID, Type: ports.EventVersionPublished, Subject: ref, PolicyGeneration: p.PolicyGeneration})
	})
	if err != nil {
		return ports.PublicationResult{}, fmt.Errorf("commit publication: %w", err)
	}
	published = !alias
	return publicationResult(prepared, receipt, published), nil
}

func nullableDigest(d ports.Digest) (any, any) {
	if d.Algorithm == "" || d.Value == "" {
		return nil, nil
	}
	return d.Algorithm, d.Value
}
func digestString(d ports.Digest) string {
	if d.Algorithm == "" && d.Value == "" {
		return ""
	}
	return d.Algorithm + ":" + d.Value
}
func publicationResult(p ports.PreparedPublication, body []byte, created bool) ports.PublicationResult {
	return ports.PublicationResult{Body: append([]byte(nil), body...), Created: created, ArtifactDigest: p.ArtifactDigest, ManifestDigest: p.ManifestDigest, PackageDigest: p.PackageDigest}
}

func (s *PublicationStore) Read(ctx context.Context, p ports.Principal, ref ports.ArtifactRef, maxBytes int64, packageBody bool, fence PublicationFence) (ports.PublicationRead, error) {
	if s == nil || s.pool == nil || s.objects == nil || fence == nil || maxBytes <= 0 || ref.WorkspaceID != p.WorkspaceID {
		return ports.PublicationRead{}, errors.New("storage: invalid publication read")
	}
	var key, da, dv, dda, ddv, mda, mdv, pda, pdv string
	var size int64
	var metadata, artifact []byte
	var state string
	err := WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		if err := fence(ctx, tx); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT state,digest_algorithm,digest_value,COALESCE(document_digest_algorithm,''),COALESCE(document_digest_value,''),COALESCE(manifest_digest_algorithm,''),COALESCE(manifest_digest_value,''),COALESCE(package_digest_algorithm,''),COALESCE(package_digest_value,''),COALESCE(object_key,''),COALESCE(octet_length(artifact_bytes),-1),COALESCE(metadata_bytes,convert_to(metadata::text,'UTF8')),artifact_bytes FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, p.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&state, &da, &dv, &dda, &ddv, &mda, &mdv, &pda, &pdv, &key, &size, &metadata, &artifact)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.PublicationRead{}, ErrNotFound
	}
	if err != nil {
		return ports.PublicationRead{}, err
	}
	if state != "published" && state != "deprecated" {
		return ports.PublicationRead{}, ErrNotFound
	}
	if packageBody && ref.Kind != ports.KindSkill {
		return ports.PublicationRead{}, ErrNotFound
	}
	if da != "sha256" || !validSHA256Hex(dv) {
		return ports.PublicationRead{}, errors.New("storage: invalid retained artifact digest")
	}
	if ref.Kind == ports.KindSkill {
		if mda != "sha256" || !validSHA256Hex(mdv) || pda != "sha256" || !validSHA256Hex(pdv) {
			return ports.PublicationRead{}, errors.New("storage: missing retained skill digest domain")
		}
	} else if dda != "sha256" || !validSHA256Hex(ddv) {
		return ports.PublicationRead{}, errors.New("storage: missing retained document digest domain")
	}
	artifactSum := sha256.Sum256(artifact)
	if hex.EncodeToString(artifactSum[:]) != dv {
		return ports.PublicationRead{}, errors.New("storage: retained artifact digest mismatch")
	}
	var body []byte
	contentType := "application/json"
	bodyDigest := ports.Digest{Algorithm: "sha256"}
	if packageBody {
		if key == "" || size < 0 {
			return ports.PublicationRead{}, ErrNotFound
		}
		if int64(len(artifact)) > maxBytes {
			return ports.PublicationRead{}, ErrPublicationBudget
		}
		r, e := s.objects.OpenOwned(ctx, key, ports.Digest{Algorithm: da, Value: dv}, size)
		if e != nil {
			return ports.PublicationRead{}, e
		}
		defer r.Close()
		readLimit := maxBytes
		if readLimit < int64(^uint64(0)>>1) {
			readLimit++
		}
		body, e = io.ReadAll(io.LimitReader(r, readLimit))
		if e != nil {
			return ports.PublicationRead{}, e
		}
		contentType = "application/zip"
	} else {
		body = metadata
		bodyDigest = ports.Digest{Algorithm: "sha256", Value: ddv}
		if ddv == "" {
			bodyDigest = ports.Digest{Algorithm: da, Value: dv}
		}
	}
	if int64(len(body)) > maxBytes {
		return ports.PublicationRead{}, ErrPublicationBudget
	}
	sum := sha256.Sum256(body)
	bodyDigest.Value = hex.EncodeToString(sum[:])
	if !packageBody {
		want := ddv
		if ref.Kind == ports.KindSkill {
			want = mdv
		}
		if want != "" && want != bodyDigest.Value {
			return ports.PublicationRead{}, errors.New("storage: retained metadata digest mismatch")
		}
	}
	return ports.PublicationRead{Body: append([]byte(nil), body...), ContentType: contentType, ArtifactDigest: ports.Digest{Algorithm: da, Value: dv}, BodyDigest: bodyDigest, ManifestDigest: ports.Digest{Algorithm: mda, Value: mdv}, PackageDigest: ports.Digest{Algorithm: pda, Value: pdv}}, nil
}

func (s *PublicationStore) Cleanup(ctx context.Context, p ports.Principal, limit int, fence PublicationFence) error {
	if s == nil || s.pool == nil || s.objects == nil || fence == nil || limit <= 0 {
		return errors.New("storage: invalid publication cleanup")
	}
	if limit > 20 {
		limit = 20
	}
	type claim struct {
		id, key    string
		generation int64
	}
	var claims []claim
	err := WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		if err := fence(ctx, tx); err != nil {
			return err
		}
		rows, e := tx.Query(ctx, `SELECT a.attempt_id,a.object_key,a.generation FROM publication_attempts a WHERE a.workspace_id=$1 AND a.state IN ('staged','deleting','retired') AND a.lease_until<=clock_timestamp() AND COALESCE(a.claim_until,'-infinity')<=clock_timestamp() AND NOT EXISTS (SELECT 1 FROM catalog_versions c WHERE c.workspace_id=a.workspace_id AND c.object_key=a.object_key) ORDER BY a.lease_until,a.attempt_id LIMIT $2 FOR UPDATE OF a SKIP LOCKED`, p.WorkspaceID, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var c claim
			if e := rows.Scan(&c.id, &c.key, &c.generation); e != nil {
				return e
			}
			claims = append(claims, c)
		}
		if e := rows.Err(); e != nil {
			return e
		}
		for i := range claims {
			claims[i].generation++
			if _, e := tx.Exec(ctx, `UPDATE publication_attempts SET state='deleting',generation=$3,claim_until=clock_timestamp()+interval '5 minutes' WHERE workspace_id=$1 AND attempt_id=$2`, p.WorkspaceID, claims[i].id, claims[i].generation); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, c := range claims {
		e := s.objects.DeleteOwned(ctx, c.key)
		if e != nil && !errors.Is(e, ErrNotFound) {
			// Rotate failures behind newer tombstones; they remain owned and
			// retryable, while bounded passes continue making progress.
			_ = WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
				if err := fence(ctx, tx); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `UPDATE publication_attempts SET state='retired',claim_until=NULL,lease_until=clock_timestamp()+interval '1 minute' WHERE workspace_id=$1 AND attempt_id=$2 AND state='deleting' AND generation=$3`, p.WorkspaceID, c.id, c.generation)
				return err
			})
			continue
		}
		e = WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
			if err := fence(ctx, tx); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE publication_attempts SET state='retired',claim_until=NULL,lease_until=clock_timestamp()+interval '24 hours' WHERE workspace_id=$1 AND attempt_id=$2 AND state='deleting' AND generation=$3`, p.WorkspaceID, c.id, c.generation)
			return err
		})
		if e != nil {
			return e
		}
	}
	return nil
}
