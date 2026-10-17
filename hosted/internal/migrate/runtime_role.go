package migrate

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Explicit grants are reviewed alongside application changes. Publisher review
// evidence and namespace ownership are readable, never writable by the service.
var runtimeTables = map[string]string{
	"catalog_versions":          "SELECT, INSERT, UPDATE",
	"catalog_payloads":          "SELECT, INSERT, UPDATE",
	"namespace_reservations":    "SELECT",
	"review_records":            "SELECT",
	"taxonomy_editions":         "SELECT, INSERT, UPDATE",
	"taxonomy_nodes":            "SELECT, INSERT, UPDATE",
	"workload_identities":       "SELECT, INSERT, UPDATE",
	"oauth_clients":             "SELECT, INSERT, UPDATE, DELETE",
	"oauth_authorization_codes": "SELECT, INSERT, UPDATE, DELETE",
	"oauth_refresh_families":    "SELECT, INSERT, UPDATE, DELETE",
	"oauth_refresh_tokens":      "SELECT, INSERT, UPDATE, DELETE",
	"workspaces":                "SELECT, INSERT",
	"workspace_memberships":     "SELECT, INSERT, UPDATE",
	"policy_decisions":          "SELECT, INSERT",
	"cursors":                   "SELECT, INSERT, UPDATE, DELETE",
	"resolutions":               "SELECT, INSERT, UPDATE, DELETE",
	"connections":               "SELECT, INSERT, UPDATE, DELETE",
	"event_outbox":              "SELECT, INSERT, UPDATE, DELETE",
	"event_cursors":             "SELECT, INSERT, UPDATE, DELETE",
	"event_retention_floors":    "SELECT, INSERT, UPDATE, DELETE",
	"github_login_flows":        "SELECT, INSERT, UPDATE",
	"workspace_invitations":     "SELECT, INSERT, UPDATE",
	"founder_bootstrap":         "SELECT, INSERT",
}
var runtimeFunctions = []string{"registry_workspace_visible(text)", "registry_semver_key(text)", "purge_registry_events(interval)"}
var runtimeRoleName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

func runtimeConfig(owner *pgx.ConnConfig, raw string) (*pgx.ConnConfig, error) {
	cfg, err := pgx.ParseConfig(raw)
	if err != nil || raw == "" || cfg.Host != owner.Host || cfg.Port != owner.Port || cfg.Database != owner.Database || (cfg.User == owner.User || (owner.Password != "" && cfg.Password == owner.Password)) || !runtimeRoleName.MatchString(cfg.User) || len(cfg.Password) < 32 {
		return nil, errors.New("runtime URL must identify the same host, port and database with a distinct role and strong password")
	}
	for _, b := range []byte(cfg.Password) {
		if b < 33 || b > 126 {
			return nil, errors.New("runtime password must use printable ASCII without spaces")
		}
	}
	// Do not accept a URL that silently switches to another host on failure.
	for _, fallback := range cfg.Fallbacks {
		if fallback.Host != owner.Host || fallback.Port != owner.Port {
			return nil, errors.New("runtime URL contains an alternate database host")
		}
	}
	return cfg, nil
}

// ProvisionRuntimeRole is a separate explicit post-migration release operation.
// It does not change ownership or create a workspace, membership or identity.
func (r Runner) ProvisionRuntimeRole(ctx context.Context, conn *pgx.Conn, raw string) error {
	cfg, err := runtimeConfig(conn.Config(), raw)
	if err != nil {
		return err
	}
	verifier, err := scramVerifier(cfg.Password)
	if err != nil {
		return errors.New("cannot prepare runtime role credential")
	}
	return r.locked(ctx, conn, func(status Status) error {
		if !status.LedgerExists || len(status.Pending) > 0 {
			return errors.New("runtime role provisioning requires a fully applied audited migration ledger")
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return errors.New("cannot start runtime role provisioning")
		}
		defer rollback(tx)
		var oid uint32
		var unsafe bool
		err = tx.QueryRow(ctx, `SELECT oid,rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole OR rolinherit OR rolreplication OR coalesce(array_length(rolconfig,1),0)>0 FROM pg_roles WHERE rolname=$1`, cfg.User).Scan(&oid, &unsafe)
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return errors.New("cannot inspect runtime role")
		}
		if exists {
			if unsafe {
				return errors.New("existing runtime role has unsafe attributes or settings")
			}
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_auth_members WHERE member=$1) OR EXISTS(SELECT 1 FROM pg_shdepend WHERE refclassid='pg_authid'::regclass AND refobjid=$1 AND deptype='o' AND dbid IN (0,(SELECT oid FROM pg_database WHERE datname=current_database())))`, oid).Scan(&unsafe); err != nil {
				return errors.New("cannot inspect runtime role membership and ownership")
			}
			if unsafe {
				return errors.New("existing runtime role has memberships or ownership")
			}
		} else {
			if _, err := tx.Exec(ctx, "CREATE ROLE "+pgx.Identifier{cfg.User}.Sanitize()+" LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION"); err != nil {
				return errors.New("cannot create restricted runtime role")
			}
		}
		role := pgx.Identifier{cfg.User}.Sanitize()
		database := pgx.Identifier{cfg.Database}.Sanitize()
		// This dedicated registry database must not inherit CREATE/TEMP or arbitrary
		// public-function EXECUTE through PUBLIC. The migration owner retains ownership.
		for _, statement := range []string{"REVOKE CREATE, TEMPORARY ON DATABASE " + database + " FROM PUBLIC", "REVOKE CREATE ON SCHEMA public FROM PUBLIC", "REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC"} {
			if _, err := tx.Exec(ctx, statement); err != nil {
				return errors.New("cannot restrict database public defaults")
			}
		}
		if err := rejectExtraRuntimePrivileges(ctx, tx, cfg.User); err != nil {
			return err
		}
		// Only the salted SCRAM verifier enters SQL, never the plaintext secret.
		if _, err := tx.Exec(ctx, "ALTER ROLE "+role+" LOGIN PASSWORD '"+verifier+"'"); err != nil {
			return errors.New("cannot install runtime role credential")
		}
		if _, err := tx.Exec(ctx, "GRANT CONNECT ON DATABASE "+database+" TO "+role); err != nil {
			return errors.New("cannot grant runtime database connection")
		}
		if _, err := tx.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+role); err != nil {
			return errors.New("cannot grant runtime schema usage")
		}
		for table, privileges := range runtimeTables {
			if _, err := tx.Exec(ctx, "GRANT "+privileges+" ON TABLE public."+pgx.Identifier{table}.Sanitize()+" TO "+role); err != nil {
				return errors.New("cannot grant reviewed runtime table privileges")
			}
		}
		if _, err := tx.Exec(ctx, "GRANT USAGE, SELECT ON SEQUENCE public.event_outbox_event_id_seq TO "+role); err != nil {
			return errors.New("cannot grant reviewed runtime sequence privileges")
		}
		for _, function := range runtimeFunctions {
			if _, err := tx.Exec(ctx, "GRANT EXECUTE ON FUNCTION public."+function+" TO "+role); err != nil {
				return errors.New("cannot grant reviewed runtime function privileges")
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return errors.New("cannot commit runtime role provisioning; inspect before retry")
		}
		return nil
	})
}

func rejectExtraRuntimePrivileges(ctx context.Context, tx pgx.Tx, role string) error {
	var unsafe bool
	if err := tx.QueryRow(ctx, `SELECT has_database_privilege($1,current_database(),'CREATE') OR has_database_privilege($1,current_database(),'TEMPORARY') OR EXISTS(SELECT 1 FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname<>'information_schema' AND has_schema_privilege($1,oid,'CREATE'))`, role).Scan(&unsafe); err != nil {
		return errors.New("cannot inspect runtime DDL privileges")
	}
	if unsafe {
		return errors.New("existing runtime role has DDL privileges")
	}
	rows, err := tx.Query(ctx, `SELECT n.nspname,c.relname,p.privilege,has_table_privilege($1,c.oid,p.privilege),has_table_privilege($1,c.oid,p.privilege||' WITH GRANT OPTION') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN (VALUES('SELECT'),('INSERT'),('UPDATE'),('DELETE'),('TRUNCATE'),('REFERENCES'),('TRIGGER')) AS p(privilege) WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f')`, role)
	if err != nil {
		return errors.New("cannot inspect runtime table privileges")
	}
	defer rows.Close()
	for rows.Next() {
		var schema, table, privilege string
		var allowed, grantOption bool
		if rows.Scan(&schema, &table, &privilege, &allowed, &grantOption) != nil {
			return errors.New("cannot inspect runtime table privileges")
		}
		if grantOption || (allowed && (schema != "public" || !strings.Contains(", "+runtimeTables[table]+",", ", "+privilege+","))) {
			return errors.New("existing runtime role has unreviewed table privileges")
		}
	}
	if rows.Err() != nil {
		return errors.New("cannot inspect runtime table privileges")
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT n.nspname,p.proname||'('||oidvectortypes(p.proargtypes)||')',has_function_privilege($1,p.oid,'EXECUTE WITH GRANT OPTION') FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND has_function_privilege($1,p.oid,'EXECUTE')`, role)
	if err != nil {
		return errors.New("cannot inspect runtime function privileges")
	}
	defer rows.Close()
	for rows.Next() {
		var schema, function string
		var grantOption bool
		if rows.Scan(&schema, &function, &grantOption) != nil {
			return errors.New("cannot inspect runtime function privileges")
		}
		allowed := false
		for _, expected := range runtimeFunctions {
			allowed = allowed || (schema == "public" && function == expected)
		}
		if !allowed || grantOption {
			return errors.New("existing runtime role has unreviewed function privileges")
		}
	}
	if rows.Err() != nil {
		return errors.New("cannot inspect runtime function privileges")
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT n.nspname,c.relname,has_sequence_privilege($1,c.oid,'USAGE'),has_sequence_privilege($1,c.oid,'SELECT'),has_sequence_privilege($1,c.oid,'UPDATE'),has_sequence_privilege($1,c.oid,'USAGE WITH GRANT OPTION') OR has_sequence_privilege($1,c.oid,'SELECT WITH GRANT OPTION') OR has_sequence_privilege($1,c.oid,'UPDATE WITH GRANT OPTION') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='S' AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'`, role)
	if err != nil {
		return errors.New("cannot inspect runtime sequence privileges")
	}
	defer rows.Close()
	for rows.Next() {
		var schema, name string
		var usage, selection, update, grantOption bool
		if rows.Scan(&schema, &name, &usage, &selection, &update, &grantOption) != nil {
			return errors.New("cannot inspect runtime sequence privileges")
		}
		if update || grantOption || ((usage || selection) && (schema != "public" || name != "event_outbox_event_id_seq")) {
			return errors.New("existing runtime role has unreviewed sequence privileges")
		}
	}
	if rows.Err() != nil {
		return errors.New("cannot inspect runtime sequence privileges")
	}
	return nil
}

func scramVerifier(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	salted, err := pbkdf2.Key(sha256.New, password, salt, 4096, 32)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, salted)
	_, _ = mac.Write([]byte("Client Key"))
	stored := sha256.Sum256(mac.Sum(nil))
	mac = hmac.New(sha256.New, salted)
	_, _ = mac.Write([]byte("Server Key"))
	server := mac.Sum(nil)
	b64 := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("SCRAM-SHA-256$4096:%s$%s:%s", b64(salt), b64(stored[:]), b64(server)), nil
}
