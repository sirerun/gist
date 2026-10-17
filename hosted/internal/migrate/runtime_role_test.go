package migrate

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRuntimeURLRequiresSeparateRoleSameDatabaseAndStrongCredential(t *testing.T) {
	owner, err := pgx.ParseConfig("postgres://owner@database.invalid:5432/gist?sslmode=require")
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"", "postgres://owner:" + strings.Repeat("a", 32) + "@database.invalid/gist", "postgres://runtime:short@database.invalid/gist", "postgres://runtime:" + strings.Repeat("a", 32) + "@other.invalid/gist", "postgres://runtime:" + strings.Repeat("a", 32) + "@database.invalid/other"} {
		if _, err := runtimeConfig(owner, url); err == nil {
			t.Fatal("accepted unsafe runtime URL")
		}
	}
	if _, err := runtimeConfig(owner, "postgres://runtime:"+strings.Repeat("a", 32)+"@database.invalid/gist?sslmode=require"); err != nil {
		t.Fatal(err)
	}
	verifier, err := scramVerifier(strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(verifier, "SCRAM-SHA-256$4096:") || strings.Contains(verifier, strings.Repeat("a", 32)) {
		t.Fatal("invalid or plaintext verifier")
	}
}
