//go:build integration

package wiring

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/storage"
)

func (f *fixture) countVersions(t *testing.T, workspace string) int {
	t.Helper()
	var n int
	if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM catalog_versions WHERE workspace_id=$1`, workspace).Scan(&n)
	}); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	return n
}

// Regression: POST /v1/publish/revocations went through the package publisher
// and inserted an ordinary catalog_versions row. It must revoke the named
// version in place: no new catalog version, the version is revoked for exact
// reads, selection and resolution, a version_revoked event is emitted once,
// and a repeat returns the original notice.
func TestPublishRevocationRevokesExistingVersion(t *testing.T) {
	f := requireFixture(t)
	const workspace = "q3-revocation"
	if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1) ON CONFLICT (id) DO NOTHING`, workspace)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	subject := f.memberSubject(t, "revoker", workspace)
	digest := strings.Repeat("b", 64)
	if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		for _, v := range []string{"1.0.0", "1.1.0"} {
			if _, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,package_key,owner_id) VALUES($1,'skill','rev-skill',$2,'published','sha256',$3,'sha256',$3,'{"id":"rev-skill","required_capabilities":[]}'::jsonb,$3,$4)`, workspace, v, digest, subject); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	token := f.token(t, workspace, subject, "catalog:read", "catalog:publish")
	// Open the feed cursor first so the revocation event lands after it.
	feed := f.do(t, "GET", "/v1/events", token, "")
	var opened struct {
		NextCursor string `json:"next_cursor"`
	}
	raw, _ := io.ReadAll(feed.Body)
	_ = feed.Body.Close()
	if err := json.Unmarshal(raw, &opened); err != nil || opened.NextCursor == "" {
		t.Fatalf("open events: %v %s", err, raw)
	}
	before := f.countVersions(t, workspace)

	body := `{"artifact":{"kind":"skill","id":"rev-skill","version":"1.0.0","reason":"compromised"},"max_bytes":4096,"idempotency_key":"rev-1"}`
	first := f.do(t, "POST", "/v1/publish/revocations", token, body)
	firstRaw, _ := io.ReadAll(first.Body)
	_ = first.Body.Close()
	if first.StatusCode != 201 {
		t.Fatalf("revoke status=%d body=%s", first.StatusCode, firstRaw)
	}
	if after := f.countVersions(t, workspace); after != before {
		t.Fatalf("catalog versions %d -> %d; a revocation must not add a version", before, after)
	}
	var state string
	if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT state FROM catalog_versions WHERE workspace_id=$1 AND artifact_id='rev-skill' AND version='1.0.0'`, workspace).Scan(&state)
	}); err != nil || state != "revoked" {
		t.Fatalf("state=%q err=%v", state, err)
	}

	second := f.do(t, "POST", "/v1/publish/revocations", token, body)
	secondRaw, _ := io.ReadAll(second.Body)
	_ = second.Body.Close()
	if second.StatusCode != 201 || string(secondRaw) != string(firstRaw) {
		t.Fatalf("repeat status=%d body=%s, want original notice %s", second.StatusCode, secondRaw, firstRaw)
	}

	exact := f.do(t, "GET", "/v1/skills/rev-skill/versions/1.0.0", token, "")
	exactRaw, _ := io.ReadAll(exact.Body)
	_ = exact.Body.Close()
	if exact.StatusCode != 409 || !strings.Contains(string(exactRaw), `"artifact_revoked"`) {
		t.Fatalf("exact read status=%d body=%s", exact.StatusCode, exactRaw)
	}
	list := f.do(t, "GET", "/v1/skills/rev-skill/versions", token, "")
	listRaw, _ := io.ReadAll(list.Body)
	_ = list.Body.Close()
	if list.StatusCode != 200 || strings.Contains(string(listRaw), `"1.0.0"`) || !strings.Contains(string(listRaw), `"1.1.0"`) {
		t.Fatalf("versions list status=%d body=%s", list.StatusCode, listRaw)
	}
	resolve := f.do(t, "POST", "/v1/resolve", token, `{"skill_ref":"rev-skill@1.0.0","runtime":{"id":"go","owned_connections":false},"max_bytes":4096}`)
	resolveRaw, _ := io.ReadAll(resolve.Body)
	_ = resolve.Body.Close()
	if resolve.StatusCode != 409 || !strings.Contains(string(resolveRaw), `"artifact_revoked"`) {
		t.Fatalf("resolve status=%d body=%s", resolve.StatusCode, resolveRaw)
	}

	events := f.do(t, "GET", "/v1/events?cursor="+opened.NextCursor, token, "")
	eventsRaw, _ := io.ReadAll(events.Body)
	_ = events.Body.Close()
	if events.StatusCode != 200 || strings.Count(string(eventsRaw), `"version_revoked"`) != 1 {
		t.Fatalf("events status=%d body=%s, want exactly one version_revoked", events.StatusCode, eventsRaw)
	}

	missing := f.do(t, "POST", "/v1/publish/revocations", token, `{"artifact":{"kind":"skill","id":"rev-skill","version":"9.9.9"},"max_bytes":4096,"idempotency_key":"rev-2"}`)
	_ = missing.Body.Close()
	if missing.StatusCode != 404 {
		t.Fatalf("missing target status=%d want 404", missing.StatusCode)
	}
}
