//go:build integration

package wiring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// Regression: listVersions filtered one search page capped at MaxResults (50
// in this fixture), so versions that sorted beyond the cap were dropped. The
// dedicated workspace holds more unrelated skills than the cap, all sorting
// ahead of the wanted id.
func TestListVersionsReturnsEveryVersionBeyondResultCap(t *testing.T) {
	f := requireFixture(t)
	const workspace = "q3-versions"
	subject := f.memberSubject(t, "versions")
	if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1) ON CONFLICT (id) DO NOTHING`, workspace); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,'maintainer',$4,1)`, workspace, f.baseURL, subject, []string{"catalog:read", "catalog:publish"}); err != nil {
			return err
		}
		insert := func(id, version string) error {
			sum := sha256.Sum256([]byte(id + "@" + version))
			digest := hex.EncodeToString(sum[:])
			_, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,package_key,owner_id) VALUES($1,$2,$3,$4,'published','sha256',$5,'sha256',$5,'{}'::jsonb,$5,$6) ON CONFLICT DO NOTHING`, workspace, string(ports.KindSkill), id, version, digest, subject)
			return err
		}
		for i := 0; i < 55; i++ {
			if err := insert(fmt.Sprintf("aaa-filler-%02d", i), "1.0.0"); err != nil {
				return err
			}
		}
		for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
			if err := insert("zzz-wanted", v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed versions workspace: %v", err)
	}
	token := f.token(t, workspace, subject, "catalog:read")
	resp := f.do(t, "GET", "/v1/skills/zzz-wanted/versions", token, "")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Items []ports.CatalogRecord `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v: %s", err, body)
	}
	got := map[string]bool{}
	for _, it := range out.Items {
		if it.Ref.ID != "zzz-wanted" {
			t.Fatalf("leaked item %+v", it.Ref)
		}
		got[it.Ref.Version] = true
	}
	if len(out.Items) != 3 || !got["1.0.0"] || !got["1.1.0"] || !got["2.0.0"] {
		t.Fatalf("versions=%v want 1.0.0, 1.1.0, 2.0.0", got)
	}
}

// Regression: ListVersions ordered by the version text, so 1.10.0 sorted
// before 1.9.0 and a pre-release after its release. It also returned every
// version in one page. Versions must come back in SemVer precedence order and
// page stably with the keyset cursor.
func TestListVersionsPagesInSemverOrder(t *testing.T) {
	f := requireFixture(t)
	const workspace = "q3-semver"
	subject := f.memberSubject(t, "semver")
	want := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0", "1.2.0", "1.9.0", "1.10.0", "2.0.0"}
	if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1) ON CONFLICT (id) DO NOTHING`, workspace); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,'maintainer',$4,1)`, workspace, f.baseURL, subject, []string{"catalog:read"}); err != nil {
			return err
		}
		// Insert in reverse text order so neither insertion nor text order
		// accidentally matches precedence.
		for i := len(want) - 1; i >= 0; i-- {
			sum := sha256.Sum256([]byte(want[i]))
			digest := hex.EncodeToString(sum[:])
			if _, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,package_key,owner_id) VALUES($1,'skill','semver-skill',$2,'published','sha256',$3,'sha256',$3,'{}'::jsonb,$3,$4)`, workspace, want[i], digest, subject); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed semver workspace: %v", err)
	}
	token := f.token(t, workspace, subject, "catalog:read")
	var got []string
	path := "/v1/skills/semver-skill/versions?limit=4"
	for pages := 0; ; pages++ {
		if pages > len(want) {
			t.Fatal("paging did not terminate")
		}
		resp := f.do(t, "GET", path, token, "")
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		var out struct {
			Items      []ports.CatalogRecord `json:"items"`
			NextCursor string                `json:"next_cursor"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("decode: %v: %s", err, body)
		}
		if len(out.Items) > 4 {
			t.Fatalf("page of %d exceeds limit 4", len(out.Items))
		}
		for _, it := range out.Items {
			got = append(got, it.Ref.Version)
		}
		if out.NextCursor == "" {
			break
		}
		path = "/v1/skills/semver-skill/versions?limit=4&cursor=" + out.NextCursor
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("versions = %v\nwant       %v", got, want)
	}
}
