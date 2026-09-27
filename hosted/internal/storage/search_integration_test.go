//go:build integration

package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// searchTestPool creates a throwaway, fully migrated database. A missing
// Postgres is a failure, not a skip.
func searchTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := os.Getenv("GIST_DATABASE_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" {
		base = "postgres://dndungu@127.0.0.1:5432/postgres?sslmode=disable"
	}
	adminCfg, err := pgx.ParseConfig(base)
	if err != nil {
		t.Fatalf("parse postgres URL: %v", err)
	}
	adminCfg.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("connect to postgres (set GIST_DATABASE_URL): %v", err)
	}
	defer admin.Close(context.Background())
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	dbName := "gist_registry_search_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+dbName+`"`); err != nil {
		t.Fatalf("create database: %v", err)
	}
	cfg, err := pgxpool.ParseConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		drop, err := pgx.ConnectConfig(context.Background(), adminCfg)
		if err != nil {
			return
		}
		defer drop.Close(context.Background())
		_, _ = drop.Exec(context.Background(), `DROP DATABASE IF EXISTS "`+dbName+`" WITH (FORCE)`)
	})
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find migrations: %v (%d files)", err, len(files))
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(file), err)
		}
	}
	return pool
}

func TestSearchRanksTokenizedMatchesWithinWorkspace(t *testing.T) {
	pool := searchTestPool(t)
	ctx := context.Background()
	type row struct{ workspace, kind, id, state, metadata string }
	rows := []row{
		{"ws-a", "skill", "gist/skill/asset-skill", "published", `{"logical_name":"asset-skill","summary":"Offline document parsing fixture.","tags":["document","fixture"],"required_capabilities":["gist/document/parse@1.0.0"]}`},
		{"ws-a", "capability", "gist/document/parse", "published", `{"logical_name":"document parse","summary":"Parse a document into text.","tags":["document"]}`},
		{"ws-a", "binding", "gist/slack-web-api/document-parse", "deprecated", `{"logical_name":"public document parser binding","summary":"Document parser binding.","tags":["document"],"capability_ref":"gist/document/parse@1.0.0"}`},
		{"ws-a", "capability", "gist/identity/user.create", "published", `{"logical_name":"identity user create","summary":"Create a user.","tags":["identity"]}`},
		// Sorts ahead of every real record alphabetically; a pre-ranking LIMIT
		// would return it instead of the better matches.
		{"ws-a", "binding", "aaa/unrelated", "published", `{"logical_name":"unrelated","summary":"Nothing relevant."}`},
		// A revoked copy with every query term must never be a candidate.
		{"ws-a", "skill", "zz/offline-document-parsing-fixture", "revoked", `{"logical_name":"offline document parsing fixture","summary":"offline document parsing fixture"}`},
		// Another workspace's perfect match must be invisible under RLS.
		{"ws-b", "skill", "gist/offline/document-parsing-fixture", "published", `{"logical_name":"offline document parsing fixture","summary":"offline document parsing fixture"}`},
	}
	for _, ws := range []string{"ws-a", "ws-b"} {
		if err := WithTenant(ctx, pool, ws, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1)`, ws); err != nil {
				return err
			}
			for _, r := range rows {
				if r.workspace != ws {
					continue
				}
				if _, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,owner_id) VALUES($1,$2,$3,'1.0.0',$4,'sha256',$5,'sha256',$5,$6,'owner')`, ws, r.kind, r.id, r.state, strings.Repeat("a", 64), r.metadata); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("seed %s: %v", ws, err)
		}
	}
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	search := func(text string, limit int) []string {
		page, err := store.Search(ctx, ports.SearchQuery{Principal: ports.Principal{Subject: "p", WorkspaceID: "ws-a"}, Text: text, Limit: limit})
		if err != nil {
			t.Fatalf("search %q: %v", text, err)
		}
		out := make([]string, 0, len(page.Records))
		for _, r := range page.Records {
			if r.Ref.WorkspaceID != "ws-a" {
				t.Fatalf("record escaped the workspace: %+v", r.Ref)
			}
			out = append(out, r.Ref.ID)
		}
		return out
	}
	if got := search("offline document parsing fixture", 1); len(got) != 1 || got[0] != "gist/skill/asset-skill" {
		t.Fatalf("limit must apply after ranking: %v", got)
	}
	got := search("offline document parsing fixture", 10)
	// The binding and capability tie on terms and weight; kind breaks the tie.
	want := []string{"gist/skill/asset-skill", "gist/slack-web-api/document-parse", "gist/document/parse"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ranking = %v, want %v", got, want)
	}
	if got := search("weather forecast lookup", 10); len(got) != 0 {
		t.Fatalf("no-match must be empty: %v", got)
	}
	if got := search("the of", 10); len(got) != 0 {
		t.Fatalf("stopword-only query must be empty: %v", got)
	}
}
