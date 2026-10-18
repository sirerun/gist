package migrate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var runtimeTenantTables = []string{
	"directory_bookmarks", "private_directory_entries", "catalog_versions", "catalog_payloads", "namespace_reservations", "review_records",
	"taxonomy_editions", "taxonomy_nodes", "workload_identities",
	"oauth_authorization_codes", "oauth_refresh_families", "oauth_refresh_tokens",
	"workspace_memberships", "policy_decisions", "cursors", "resolutions", "connections",
	"event_outbox", "event_cursors", "event_retention_floors", "workspace_invitations",
}

// RuntimeAttestation reports only successful observations. Row-hiding probes may
// be vacuous on an empty catalog; seeded integration is separate evidence.
type RuntimeAttestation struct {
	User                          string `json:"user"`
	ReadOnly                      bool   `json:"read_only"`
	Authenticated                 bool   `json:"authenticated"`
	RestrictedAttributes          bool   `json:"restricted_attributes"`
	NoMemberships                 bool   `json:"no_memberships"`
	NonOwner                      bool   `json:"non_owner"`
	ReviewedPrivilegesOnly        bool   `json:"reviewed_privileges_only"`
	RequiredGrants                bool   `json:"required_grants"`
	ForceRLS                      bool   `json:"force_rls"`
	UnscopedRowsHidden            bool   `json:"unscoped_rows_hidden"`
	MismatchedWorkspaceRowsHidden bool   `json:"mismatched_workspace_rows_hidden"`
}

// VerifyRuntimeRole performs no DDL or row writes. It authenticates as the actual
// application identity instead of relying on privileged role metadata alone.
func (r Runner) VerifyRuntimeRole(ctx context.Context, owner *pgx.ConnConfig, raw string) (RuntimeAttestation, error) {
	cfg, err := runtimeConfig(owner, raw)
	if err != nil {
		return RuntimeAttestation{}, err
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return RuntimeAttestation{}, errors.New("cannot authenticate runtime database role")
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return RuntimeAttestation{}, errors.New("cannot begin read-only runtime verification")
	}
	defer rollback(tx)
	var current, session string
	var readOnly bool
	if err := tx.QueryRow(ctx, `SELECT current_user,session_user,current_setting('transaction_read_only')='on'`).Scan(&current, &session, &readOnly); err != nil || current != cfg.User || session != cfg.User || !readOnly {
		return RuntimeAttestation{}, errors.New("runtime identity or read-only transaction mismatch")
	}
	var unsafe bool
	var oid uint32
	if err := tx.QueryRow(ctx, `SELECT oid,NOT rolcanlogin OR rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole OR rolinherit OR rolreplication OR coalesce(array_length(rolconfig,1),0)>0 FROM pg_roles WHERE rolname=current_user`).Scan(&oid, &unsafe); err != nil || unsafe {
		return RuntimeAttestation{}, errors.New("runtime role attributes are unsafe")
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_auth_members WHERE member=$1) OR EXISTS(SELECT 1 FROM pg_shdepend WHERE refclassid='pg_authid'::regclass AND refobjid=$1 AND deptype='o' AND dbid IN (0,(SELECT oid FROM pg_database WHERE datname=current_database())))`, oid).Scan(&unsafe); err != nil || unsafe {
		return RuntimeAttestation{}, errors.New("runtime role has unsafe memberships or ownership")
	}
	if err := rejectExtraRuntimePrivileges(ctx, tx, current); err != nil {
		return RuntimeAttestation{}, err
	}
	// Check required rights as well as forbidden rights: a powerless identity is
	// not a working runtime. These are privilege inspections, never write probes.
	for table, privileges := range runtimeTables {
		for _, privilege := range strings.Split(privileges, ", ") {
			var allowed bool
			if err := tx.QueryRow(ctx, `SELECT has_table_privilege(current_user,$1,$2)`, "public."+table, privilege).Scan(&allowed); err != nil || !allowed {
				return RuntimeAttestation{}, errors.New("runtime role lacks reviewed required table privileges")
			}
		}
	}
	for _, function := range runtimeFunctions {
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT has_function_privilege(current_user,$1,'EXECUTE')`, "public."+function).Scan(&allowed); err != nil || !allowed {
			return RuntimeAttestation{}, errors.New("runtime role lacks reviewed required function privileges")
		}
	}
	var sequenceAllowed bool
	if err := tx.QueryRow(ctx, `SELECT has_sequence_privilege(current_user,'public.event_outbox_event_id_seq','USAGE') AND has_sequence_privilege(current_user,'public.event_outbox_event_id_seq','SELECT')`).Scan(&sequenceAllowed); err != nil || !sequenceAllowed {
		return RuntimeAttestation{}, errors.New("runtime role lacks reviewed required sequence privileges")
	}
	for _, table := range runtimeTenantTables {
		var forced bool
		if err := tx.QueryRow(ctx, `SELECT c.relrowsecurity AND c.relforcerowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=$1`, table).Scan(&forced); err != nil || !forced {
			return RuntimeAttestation{}, errors.New("runtime tenant relation lacks forced row security")
		}
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return RuntimeAttestation{}, errors.New("cannot generate read-only verification scope")
	}
	mismatch := "gist-verification-" + hex.EncodeToString(nonce)
	var collision bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.workspaces WHERE id=$1)`, mismatch).Scan(&collision); err != nil || collision {
		return RuntimeAttestation{}, errors.New("cannot qualify mismatched verification scope")
	}
	for _, scope := range []string{"", mismatch} {
		if _, err := tx.Exec(ctx, `SELECT set_config('registry.workspace_id',$1,true)`, scope); err != nil {
			return RuntimeAttestation{}, errors.New("cannot set read-only verification scope")
		}
		for _, table := range runtimeTenantTables {
			var visible bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM public."+pgx.Identifier{table}.Sanitize()+")").Scan(&visible); err != nil || visible {
				return RuntimeAttestation{}, errors.New("runtime row-security isolation probe failed")
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return RuntimeAttestation{}, errors.New("cannot complete read-only runtime verification")
	}
	return RuntimeAttestation{User: current, ReadOnly: true, Authenticated: true, RestrictedAttributes: true, NoMemberships: true, NonOwner: true, ReviewedPrivilegesOnly: true, RequiredGrants: true, ForceRLS: true, UnscopedRowsHidden: true, MismatchedWorkspaceRowsHidden: true}, nil
}
