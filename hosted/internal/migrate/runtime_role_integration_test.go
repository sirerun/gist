//go:build integration

package migrate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/migrations"
)

func runtimeFixture(t *testing.T, owner *pgx.Conn) (string, string) {
	t.Helper()
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	role := "gist_runtime_" + hex.EncodeToString(id)
	cfg := owner.Config()
	u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, User: url.UserPassword(role, strings.Repeat("x", 48))}
	query := u.Query()
	query.Set("sslmode", "disable")
	u.RawQuery = query.Encode()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var exists bool
		if err := owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", role).Scan(&exists); err != nil {
			t.Error(err)
			return
		}
		if !exists {
			return
		}
		if _, err := owner.Exec(ctx, "DROP OWNED BY "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Error(err)
		}
		if _, err := owner.Exec(ctx, "DROP ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Error(err)
		}
	})
	return role, u.String()
}
func TestProvisionRuntimeRoleRestrictedRLSAndNoDDLOwnership(t *testing.T) {
	owner, _ := migrationDatabase(t)
	ctx := context.Background()
	r, err := Load(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	role, raw := runtimeFixture(t, owner)
	if r.ProvisionRuntimeRole(ctx, owner, raw) == nil {
		t.Fatal("provisioned before audited migrations")
	}
	if err := r.Apply(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if err := r.ProvisionRuntimeRole(ctx, owner, raw); err != nil {
		t.Fatal(err)
	}
	// A second explicit run is safe and rotates the verifier without widening rights.
	if err := r.ProvisionRuntimeRole(ctx, owner, raw); err != nil {
		t.Fatal(err)
	}
	var unsafe bool
	if err := owner.QueryRow(ctx, `SELECT rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole OR rolinherit OR rolreplication FROM pg_roles WHERE rolname=$1`, role).Scan(&unsafe); err != nil || unsafe {
		t.Fatalf("unsafe runtime attributes err=%v", err)
	}
	runtime, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal("runtime authentication failed")
	}
	defer func() { _ = runtime.Close(context.Background()) }()
	for _, statement := range []string{"CREATE TABLE public.forbidden(id integer)", "CREATE TEMP TABLE forbidden_temp(id integer)", "SELECT * FROM public.registry_schema_migrations", "ALTER TABLE public.workspaces ADD COLUMN forbidden integer", "INSERT INTO public.review_records DEFAULT VALUES", "DELETE FROM public.namespace_reservations"} {
		if _, err := runtime.Exec(ctx, statement); err == nil {
			t.Fatalf("runtime accepted forbidden operation %s", statement)
		}
	}
	if _, err := owner.Exec(ctx, `INSERT INTO public.event_outbox(workspace_id,event_type,artifact_kind,artifact_id,artifact_version,policy_generation) VALUES('tenant-a','version_published','skill','a','1.0.0',1),('tenant-b','version_published','skill','b','1.0.0',1)`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := runtime.QueryRow(ctx, "SELECT count(*) FROM public.event_outbox").Scan(&count); err != nil || count != 0 {
		t.Fatalf("unscoped rows=%d err=%v", count, err)
	}
	if _, err := runtime.Exec(ctx, `SELECT set_config('registry.workspace_id','tenant-a',false)`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.QueryRow(ctx, "SELECT count(*) FROM public.event_outbox").Scan(&count); err != nil || count != 1 {
		t.Fatalf("scoped rows=%d err=%v", count, err)
	}
	if _, err := runtime.Exec(ctx, `INSERT INTO public.event_outbox(workspace_id,event_type,artifact_kind,artifact_id,artifact_version,policy_generation) VALUES('tenant-b','version_published','skill','x','1.0.0',1)`); err == nil {
		t.Fatal("cross-tenant write succeeded")
	}
	if _, err := runtime.Exec(ctx, `INSERT INTO public.event_outbox(workspace_id,event_type,artifact_kind,artifact_id,artifact_version,policy_generation) VALUES('tenant-a','version_published','skill','x','1.0.0',1)`); err != nil {
		t.Fatal("same-tenant write failed")
	}
}
func TestProvisionRuntimeRoleRefusesExistingPrivilegeEscalation(t *testing.T) {
	for _, kind := range []string{"attributes", "membership", "ownership", "ledger"} {
		t.Run(kind, func(t *testing.T) {
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
			quoted := pgx.Identifier{role}.Sanitize()
			if _, err := owner.Exec(ctx, "CREATE ROLE "+quoted+" LOGIN NOINHERIT"); err != nil {
				t.Fatal(err)
			}
			var statement string
			switch kind {
			case "attributes":
				statement = "ALTER ROLE " + quoted + " CREATEDB"
			case "membership":
				statement = "GRANT pg_read_all_data TO " + quoted
			case "ownership":
				statement = "CREATE TABLE public.owned(id integer); ALTER TABLE public.owned OWNER TO " + quoted
			case "ledger":
				statement = "GRANT SELECT ON public.registry_schema_migrations TO " + quoted
			}
			if _, err := owner.Exec(ctx, statement); err != nil {
				t.Fatal(err)
			}
			if err := r.ProvisionRuntimeRole(ctx, owner, raw); err == nil {
				t.Fatal("accepted unsafe existing runtime role")
			}
			// Rejected role is not silently repaired into a production identity.
		})
	}
}
