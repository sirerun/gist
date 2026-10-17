//go:build integration

package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/migrations"
)

func TestReadOnlyRuntimeVerificationAndSchemaAudit(t *testing.T) {
	owner, _ := migrationDatabase(t)
	ctx := context.Background()
	r, err := Load(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(ctx, owner); err != nil {
		t.Fatal(err)
	}
	role, raw := runtimeFixture(t, owner)
	if err := r.ProvisionRuntimeRole(ctx, owner, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO public.workspaces(id,name) VALUES('tenant-a','private_row_marker_740921'); INSERT INTO public.event_outbox(workspace_id,event_type,artifact_kind,artifact_id,artifact_version,policy_generation) VALUES('tenant-a','version_published','skill','private_row_marker_740921','1.0.0',1),('tenant-b','version_published','skill','private_row_marker_740921','1.0.0',1)`); err != nil {
		t.Fatal(err)
	}
	before, err := r.Status(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	report, err := r.VerifyRuntimeRole(ctx, owner.Config(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if report.User != role || !report.ReadOnly || !report.Authenticated || !report.ForceRLS || !report.UnscopedRowsHidden || !report.MismatchedWorkspaceRowsHidden {
		t.Fatal("incomplete actual runtime attestation")
	}
	audit, err := r.SchemaDefinitions(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if !sort.StringsAreSorted(audit.Definitions) || len(audit.Definitions) == 0 {
		t.Fatal("schema definitions not sorted or empty")
	}
	encoded, err := json.Marshal(audit.Definitions)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(encoded)
	if hex.EncodeToString(hash[:]) != audit.SchemaSHA256 || audit.SchemaSHA256 != before.SchemaSHA256 {
		t.Fatal("definition audit does not agree with status fingerprint")
	}
	if strings.Contains(string(encoded), "private_row_marker_740921") {
		t.Fatal("row contents escaped into DDL audit")
	}
	after, err := r.Status(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if after.SchemaSHA256 != before.SchemaSHA256 {
		t.Fatal("read-only verification changed definitions")
	}
	var count int
	if err := owner.QueryRow(ctx, "SELECT count(*) FROM public.event_outbox").Scan(&count); err != nil || count != 2 {
		t.Fatal("read-only verification changed fixture rows")
	}
	quoted := pgx.Identifier{role}.Sanitize()
	if _, err := owner.Exec(ctx, "GRANT SELECT ON public.registry_schema_migrations TO "+quoted); err != nil {
		t.Fatal(err)
	}
	if _, err := r.VerifyRuntimeRole(ctx, owner.Config(), raw); err == nil {
		t.Fatal("runtime verification accepted unsafe ledger grant")
	}
	if _, err := owner.Exec(ctx, "REVOKE SELECT ON public.registry_schema_migrations FROM "+quoted+"; ALTER TABLE public.event_outbox NO FORCE ROW LEVEL SECURITY"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.VerifyRuntimeRole(ctx, owner.Config(), raw); err == nil {
		t.Fatal("runtime verification accepted missing FORCE RLS")
	}
	if _, err := owner.Exec(ctx, "ALTER TABLE public.event_outbox FORCE ROW LEVEL SECURITY; REVOKE DELETE ON public.github_login_flows FROM "+quoted); err != nil {
		t.Fatal(err)
	}
	if _, err := r.VerifyRuntimeRole(ctx, owner.Config(), raw); err == nil {
		t.Fatal("runtime verification accepted missing required grant")
	}
}
