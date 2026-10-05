//go:build integration

package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
)

const (
	compositionIssuer    = "https://composition.example.invalid"
	compositionSubject   = "composition-agent"
	compositionWorkspace = "composition-workspace"
)

type compositionPostgres struct {
	adminPool  *pgxpool.Pool
	runtimeURL string
	role       string
}

// compositionPostgres creates a private throwaway database and a restricted
// login role. Migrations and the controlled tenant seed use the fixture admin;
// every App.New pool uses runtimeURL and is checked as the restricted role.
func newCompositionPostgres(t *testing.T) compositionPostgres {
	t.Helper()
	base := os.Getenv("GIST_DATABASE_URL")
	if base == "" {
		t.Fatal("GIST_DATABASE_URL is required for PostgreSQL integration tests")
	}

	adminCfg, err := pgx.ParseConfig(base)
	if err != nil {
		t.Fatalf("parse fixture PostgreSQL URL: %v", err)
	}
	adminCfg.Database = "postgres"
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("connect to fixture PostgreSQL: %v", err)
	}
	var adminPool *pgxpool.Pool
	var database, role string
	databaseCreated, roleCreated := false, false
	t.Cleanup(func() {
		if adminPool != nil {
			adminPool.Close()
		}
		if admin != nil {
			if err := admin.Close(context.Background()); err != nil {
				t.Errorf("close fixture setup connection: %v", err)
			}
		}
		if !databaseCreated && !roleCreated {
			return
		}
		dropCfg, err := pgx.ParseConfig(base)
		if err != nil {
			t.Errorf("parse fixture cleanup connection config: %v", err)
			return
		}
		dropCfg.Database = "postgres"
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		drop, err := pgx.ConnectConfig(dropCtx, dropCfg)
		if err != nil {
			t.Errorf("connect for fixture cleanup: %v", err)
			return
		}
		if databaseCreated {
			if _, err := drop.Exec(dropCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)"); err != nil {
				t.Errorf("drop owned composition database: %v", err)
			}
		}
		if roleCreated {
			if _, err := drop.Exec(dropCtx, "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Errorf("drop owned restricted application role: %v", err)
			}
		}
		if err := drop.Close(context.Background()); err != nil {
			t.Errorf("close fixture cleanup connection: %v", err)
		}
	})

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal("generate fixture identifiers")
	}
	short := hex.EncodeToString(suffix)
	database = "gist_composition_" + short
	role = "gist_app_" + short
	passwordBytes := make([]byte, 24)
	if _, err := rand.Read(passwordBytes); err != nil {
		t.Fatal("generate restricted role password")
	}
	password := hex.EncodeToString(passwordBytes)
	quotedDatabase := pgx.Identifier{database}.Sanitize()
	quotedRole := pgx.Identifier{role}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE ROLE "+quotedRole+" LOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS PASSWORD '"+password+"'"); err != nil {
		t.Fatalf("create restricted application role: %v", err)
	}
	roleCreated = true
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quotedDatabase); err != nil {
		t.Fatalf("create owned composition database: %v", err)
	}
	databaseCreated = true
	if _, err := admin.Exec(ctx, "GRANT CONNECT ON DATABASE "+quotedDatabase+" TO "+quotedRole); err != nil {
		t.Fatalf("grant fixture database connection: %v", err)
	}

	adminDBCfg, err := pgxpool.ParseConfig(base)
	if err != nil {
		t.Fatalf("parse fixture pool config: %v", err)
	}
	adminDBCfg.ConnConfig.Database = database
	adminPool, err = pgxpool.NewWithConfig(ctx, adminDBCfg)
	if err != nil {
		t.Fatalf("open fixture migration pool: %v", err)
	}

	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		adminPool.Close()
		t.Fatalf("find hosted migrations: %v (%d files)", err, len(files))
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			adminPool.Close()
			t.Fatalf("read migration %s: %v", filepath.Base(file), err)
		}
		if _, err := adminPool.Exec(ctx, string(raw)); err != nil {
			adminPool.Close()
			t.Fatalf("apply migration %s: %v", filepath.Base(file), err)
		}
	}
	for _, grant := range []string{
		"GRANT USAGE ON SCHEMA public TO " + quotedRole,
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO " + quotedRole,
		"GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO " + quotedRole,
		"GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO " + quotedRole,
	} {
		if _, err := adminPool.Exec(ctx, grant); err != nil {
			adminPool.Close()
			t.Fatalf("grant restricted application permissions: %v", err)
		}
	}
	setupConn := admin
	admin = nil
	if err := setupConn.Close(context.Background()); err != nil {
		t.Fatalf("close fixture administration connection: %v", err)
	}

	runtimeURL, err := databaseURL(base, database, role, password)
	if err != nil {
		adminPool.Close()
		t.Fatalf("build restricted runtime URL: %v", err)
	}
	return compositionPostgres{adminPool: adminPool, runtimeURL: runtimeURL, role: role}
}

func databaseURL(base, database, user, password string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", fmt.Errorf("fixture database URL must be a postgres URI")
	}
	u.Path = "/" + database
	u.RawPath = ""
	if user != "" {
		u.User = url.UserPassword(user, password)
	}
	return u.String(), nil
}

func (f compositionPostgres) seedTenant(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := f.adminPool.Exec(ctx, `INSERT INTO workspaces(id,name,policy_generation) VALUES($1,$1,1)`, compositionWorkspace)
	if err != nil {
		t.Fatalf("seed controlled fixture workspace: %v", err)
	}
	_, err = f.adminPool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,active,policy_generation) VALUES($1,$2,$3,'maintainer',$4,true,1)`, compositionWorkspace, compositionIssuer, compositionSubject, []string{"catalog:read", "catalog:publish", "catalog:resolve"})
	if err != nil {
		t.Fatalf("seed controlled fixture membership: %v", err)
	}
}

func compositionSigningConfig(t *testing.T) []byte {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("generate test-only workload signing key")
	}
	config, err := json.Marshal(identity.KeySetConfig{Current: identity.KeyConfig{
		KID:       "composition-fixture",
		Algorithm: "EdDSA",
		Private:   private,
		Public:    private.Public().(ed25519.PublicKey),
	}})
	if err != nil {
		t.Fatalf("marshal test-only signing config: %v", err)
	}
	return config
}

func compositionConfig(databaseURL, objectRoot string, signingConfig []byte) Config {
	return Config{
		ListenAddress:         "127.0.0.1:0",
		PublicOrigin:          compositionIssuer,
		ResourceAudience:      compositionIssuer,
		DatabaseURL:           databaseURL,
		ObjectStoreRoot:       objectRoot,
		RequestTimeout:        3 * time.Second,
		MaxPackageBytes:       1 << 20,
		MaxExpandedBytes:      4 << 20,
		MaxRequestBytes:       1 << 20,
		MaxResponseBytes:      1 << 20,
		MaxCatalogEntries:     100,
		MaxConcurrentRequests: 8,
		MaxDiscoveryResults:   8,
		RetryAfter:            1,
		OAuthConsentSecret:    []byte(strings.Repeat("c", MinOAuthConsentSecretBytes)),
		SigningKeyConfig:      append([]byte(nil), signingConfig...),
		EventMaintenanceTargets: []MaintenanceTarget{{
			WorkspaceID: compositionWorkspace,
			Subject:     compositionSubject,
		}},
	}
}

func compositionHTTP(t *testing.T, handler http.Handler, method, target, bearer string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(string(body)))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func assertCompositionRuntimeRole(t *testing.T, instance *App, wantRole string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := instance.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire application pool connection: %v", err)
	}
	defer conn.Release()
	var currentUser, sessionUser string
	var superuser, bypassRLS bool
	err = conn.QueryRow(ctx, `SELECT current_user, session_user, r.rolsuper, r.rolbypassrls FROM pg_roles r WHERE r.rolname=current_user`).Scan(&currentUser, &sessionUser, &superuser, &bypassRLS)
	if err != nil {
		t.Fatalf("read runtime role attributes: %v", err)
	}
	if currentUser != wantRole || sessionUser != wantRole || superuser || bypassRLS {
		t.Fatalf("App pool is not constrained to the fixture role: current=%q session=%q superuser=%t bypassrls=%t", currentUser, sessionUser, superuser, bypassRLS)
	}
}

func TestComposedAppsShareWorkloadKeysAndDurableEventCursor(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	objectRoot, err := os.MkdirTemp("", "gist-composition-app-objects-")
	if err != nil {
		t.Fatalf("create owned object-store directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(objectRoot); err != nil {
			t.Errorf("remove owned object-store fixture: %v", err)
		}
	})

	signing := compositionSigningConfig(t)
	cfg := compositionConfig(fixture.runtimeURL, objectRoot, signing)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	appA, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("start app A with the restricted PostgreSQL role: %v", err)
	}
	t.Cleanup(func() {
		if err := appA.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown app A: %v", err)
		}
	})
	appB, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("start independent app B with the restricted PostgreSQL role: %v", err)
	}
	t.Cleanup(func() {
		if err := appB.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown app B: %v", err)
		}
	})
	assertCompositionRuntimeRole(t, appA, fixture.role)
	assertCompositionRuntimeRole(t, appB, fixture.role)

	jwksA := compositionHTTP(t, appA.Handler(), http.MethodGet, "/oauth/jwks", "", nil)
	jwksB := compositionHTTP(t, appB.Handler(), http.MethodGet, "/oauth/jwks", "", nil)
	if jwksA.Code != http.StatusOK || jwksB.Code != http.StatusOK || !strings.EqualFold(jwksA.Body.String(), jwksB.Body.String()) {
		t.Fatalf("independent App startups published different workload JWKS: A=%d B=%d", jwksA.Code, jwksB.Code)
	}
	if strings.Contains(jwksA.Body.String(), `"d"`) {
		t.Fatal("public JWKS exposed private key material")
	}

	workloadToken, err := appA.MintWorkloadToken(ctx, identity.WorkloadRequest{
		Subject:     compositionSubject,
		WorkspaceID: compositionWorkspace,
		Scopes:      []string{"catalog:read", "catalog:publish"},
	})
	if err != nil {
		t.Fatalf("mint workload token through app A: %v", err)
	}
	first := compositionHTTP(t, appA.Handler(), http.MethodGet, "/v1/events", workloadToken, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("open event cursor through app A: status=%d body=%s", first.Code, first.Body.String())
	}
	var firstPage struct {
		Events     []json.RawMessage `json:"events"`
		NextCursor string            `json:"next_cursor"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstPage); err != nil {
		t.Fatalf("decode initial event page: %v", err)
	}
	if len(firstPage.Events) != 0 || firstPage.NextCursor == "" {
		t.Fatalf("expected empty first page with a durable cursor, got %s", first.Body.String())
	}

	principal := ports.Principal{
		Issuer: compositionIssuer, Subject: compositionSubject, Audience: compositionIssuer,
		WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read", "catalog:publish"},
		PolicyGeneration: 1, SubjectType: "workload",
	}
	ref := ports.ArtifactRef{WorkspaceID: compositionWorkspace, Kind: ports.KindSkill, ID: "fixture-skill", Version: "1.0.0"}
	if err := appA.SeedAcceptanceArtifact(ctx, principal, ref, []byte("fixture package"), []byte(`{"id":"fixture-skill","version":"1.0.0"}`)); err != nil {
		t.Fatalf("seed controlled catalog artifact: %v", err)
	}
	revokeBody, err := json.Marshal(map[string]any{
		"artifact":  map[string]string{"kind": "skill", "id": ref.ID, "version": ref.Version, "reason": "composition integration fixture"},
		"max_bytes": 4096, "idempotency_key": "composition-fixture-revoke",
	})
	if err != nil {
		t.Fatalf("marshal fixture revocation: %v", err)
	}
	revoked := compositionHTTP(t, appA.Handler(), http.MethodPost, "/v1/publish/revocations", workloadToken, revokeBody)
	if revoked.Code != http.StatusCreated {
		t.Fatalf("revoke fixture artifact through app A: status=%d body=%s", revoked.Code, revoked.Body.String())
	}

	resume := compositionHTTP(t, appB.Handler(), http.MethodGet, "/v1/events?cursor="+url.QueryEscape(firstPage.NextCursor), workloadToken, nil)
	if resume.Code != http.StatusOK {
		t.Fatalf("resume app A cursor through app B: status=%d body=%s", resume.Code, resume.Body.String())
	}
	var resumed struct {
		Events     []json.RawMessage `json:"events"`
		NextCursor string            `json:"next_cursor"`
	}
	if err := json.Unmarshal(resume.Body.Bytes(), &resumed); err != nil {
		t.Fatalf("decode resumed event page: %v", err)
	}
	if len(resumed.Events) != 1 || resumed.NextCursor == "" {
		t.Fatalf("expected one durable revocation event on app B, got %s", resume.Body.String())
	}
	var event map[string]json.RawMessage
	if err := json.Unmarshal(resumed.Events[0], &event); err != nil {
		t.Fatalf("decode canonical event wire object: %v", err)
	}
	wantFields := []string{"event_id", "event_type", "subject_ref", "occurred_at", "policy_generation"}
	if len(event) != len(wantFields) {
		t.Fatalf("event wire fields differ from frozen v1 schema: %s", resumed.Events[0])
	}
	for _, field := range wantFields {
		if _, ok := event[field]; !ok {
			t.Fatalf("event missing frozen v1 field %q: %s", field, resumed.Events[0])
		}
	}
	var eventID, eventType, subjectRef, occurredAt string
	var generation uint64
	for field, dst := range map[string]*string{"event_id": &eventID, "event_type": &eventType, "subject_ref": &subjectRef, "occurred_at": &occurredAt} {
		if err := json.Unmarshal(event[field], dst); err != nil {
			t.Fatalf("decode event field %s: %v", field, err)
		}
	}
	if err := json.Unmarshal(event["policy_generation"], &generation); err != nil {
		t.Fatalf("decode event policy generation: %v", err)
	}
	if eventID == "" || eventType != "version_revoked" || subjectRef != ref.ID+"@"+ref.Version || generation != principal.PolicyGeneration {
		t.Fatalf("revocation event lost canonical reference or generation: id=%q type=%q ref=%q generation=%d", eventID, eventType, subjectRef, generation)
	}
	if _, err := time.Parse(time.RFC3339, occurredAt); err != nil {
		t.Fatalf("event timestamp is not frozen RFC3339 wire text: %q", occurredAt)
	}

	if err := appA.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown app A before restart check: %v", err)
	}
	appC, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("start app C after app A shutdown: %v", err)
	}
	t.Cleanup(func() {
		if err := appC.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown app C: %v", err)
		}
	})
	assertCompositionRuntimeRole(t, appC, fixture.role)
	jwksC := compositionHTTP(t, appC.Handler(), http.MethodGet, "/oauth/jwks", "", nil)
	if jwksC.Code != http.StatusOK || jwksC.Body.String() != jwksA.Body.String() {
		t.Fatalf("restart app C changed workload JWKS: status=%d", jwksC.Code)
	}
	accepted := compositionHTTP(t, appC.Handler(), http.MethodGet, "/v1/events?cursor="+url.QueryEscape(resumed.NextCursor), workloadToken, nil)
	if accepted.Code != http.StatusOK {
		t.Fatalf("app C did not accept app A token and app B cursor after restart: status=%d body=%s", accepted.Code, accepted.Body.String())
	}
}

func TestComposedStartupRejectsMissingOrMalformedKeysBeforeDatabase(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys []byte
	}{
		{name: "missing"},
		{name: "malformed", keys: []byte(`{"current":`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := compositionConfig("postgres://unreachable@127.0.0.1:1/no_database?sslmode=disable", "/unused", compositionSigningConfig(t))
			cfg.SigningKeyConfig = tc.keys
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			instance, err := New(ctx, cfg)
			if instance != nil {
				if shutdownErr := instance.Shutdown(context.Background()); shutdownErr != nil {
					t.Errorf("shutdown unexpectedly started app: %v", shutdownErr)
				}
				t.Fatal("invalid key config unexpectedly started an app")
			}
			if err == nil || !strings.Contains(err.Error(), "GIST_SIGNING_KEY_CONFIG") || strings.Contains(err.Error(), "connect to postgres") {
				t.Fatalf("startup did not reject signing config before database access: %v", err)
			}
		})
	}
}
