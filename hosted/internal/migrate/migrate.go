// Package migrate implements explicit, forward-only registry schema delivery.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const ledger = "registry_schema_migrations"

// Every migrator uses the same session lock; closing a failed connection also
// releases it. Application request connections never acquire this lock.
const lockID int64 = 0x474953544d494752

type Migration struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	SHA256  string `json:"sha256"`
	SQL     string `json:"-"`
}
type Applied struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	SHA256  string `json:"sha256"`
	Adopted bool   `json:"adopted"`
}
type Status struct {
	ManifestSHA256  string      `json:"manifest_sha256"`
	SchemaSHA256    string      `json:"schema_sha256"`
	LedgerExists    bool        `json:"ledger_exists"`
	Relations       []string    `json:"relations"`
	ExistingObjects int         `json:"existing_objects"`
	Applied         []Applied   `json:"applied"`
	Pending         []Migration `json:"pending"`
}
type Runner struct{ Migrations []Migration }

var filename = regexp.MustCompile(`^([0-9]{3})_[a-z0-9_]+\.sql$`)

func Load(files fs.FS) (Runner, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return Runner{}, errors.New("migration assets unavailable")
	}
	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := filename.FindStringSubmatch(entry.Name())
		if match == nil {
			return Runner{}, errors.New("invalid migration asset name")
		}
		raw, err := fs.ReadFile(files, entry.Name())
		if err != nil || len(raw) == 0 {
			return Runner{}, errors.New("migration asset unavailable or empty")
		}
		version, _ := strconv.Atoi(match[1])
		hash := sha256.Sum256(raw)
		migrations = append(migrations, Migration{Version: version, Name: entry.Name(), SHA256: hex.EncodeToString(hash[:]), SQL: string(raw)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	if len(migrations) == 0 {
		return Runner{}, errors.New("no migration assets")
	}
	for i, m := range migrations {
		if m.Version != i+1 {
			return Runner{}, errors.New("migration versions must form a unique contiguous prefix")
		}
	}
	return Runner{Migrations: migrations}, nil
}

func (r Runner) ManifestSHA256() string {
	raw, _ := json.Marshal(r.Migrations)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r Runner) Status(ctx context.Context, conn *pgx.Conn) (Status, error) {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Status{}, errors.New("cannot start read-only migration status")
	}
	defer rollback(tx)
	status, err := r.status(ctx, tx)
	if err != nil {
		return Status{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Status{}, errors.New("cannot complete read-only migration status")
	}
	return status, nil
}

func (r Runner) status(ctx context.Context, q queryer) (Status, error) {
	status := Status{ManifestSHA256: r.ManifestSHA256(), Applied: []Applied{}, Relations: []string{}, Pending: []Migration{}}
	var exists bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('public.registry_schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return Status{}, errors.New("cannot inspect migration ledger")
	}
	status.LedgerExists = exists
	if exists {
		rows, err := q.Query(ctx, `SELECT version,name,sha256,adopted FROM public.registry_schema_migrations ORDER BY version`)
		if err != nil {
			return Status{}, errors.New("cannot read migration ledger")
		}
		for rows.Next() {
			var applied Applied
			if err := rows.Scan(&applied.Version, &applied.Name, &applied.SHA256, &applied.Adopted); err != nil {
				rows.Close()
				return Status{}, errors.New("invalid migration ledger")
			}
			status.Applied = append(status.Applied, applied)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return Status{}, errors.New("cannot read migration ledger")
		}
	}
	if err := r.validate(status.Applied); err != nil {
		return Status{}, err
	}
	status.Pending = append(status.Pending, r.Migrations[len(status.Applied):]...)
	rows, err := q.Query(ctx, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations' AND c.relkind IN ('r','p','v','m','S','f') ORDER BY c.relname COLLATE "C"`)
	if err != nil {
		return Status{}, errors.New("cannot inspect schema relations")
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return Status{}, errors.New("cannot inspect schema relations")
		}
		status.Relations = append(status.Relations, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Status{}, errors.New("cannot inspect schema relations")
	}
	if err := q.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations' AND c.relkind IN ('r','p','v','m','S','f')) +
 (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public') +
 (SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typrelid=0 AND t.typelem=0)
 `).Scan(&status.ExistingObjects); err != nil {
		return Status{}, errors.New("cannot inspect existing schema objects")
	}
	status.SchemaSHA256, err = schemaFingerprint(ctx, q)
	if err != nil {
		return Status{}, err
	}
	return status, nil
}
func (r Runner) validate(applied []Applied) error {
	if len(applied) > len(r.Migrations) {
		return errors.New("unknown applied migration")
	}
	for i, a := range applied {
		expected := r.Migrations[i]
		if a.Version != expected.Version {
			return errors.New("missing or unknown applied migration")
		}
		if a.Name != expected.Name || a.SHA256 != expected.SHA256 {
			return errors.New("applied migration checksum or name drift")
		}
	}
	return nil
}

const createLedger = `CREATE TABLE public.registry_schema_migrations (version integer PRIMARY KEY CHECK(version>0), name text NOT NULL UNIQUE, sha256 text NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'), adopted boolean NOT NULL DEFAULT false, applied_at timestamptz NOT NULL DEFAULT now())`

func (r Runner) locked(ctx context.Context, conn *pgx.Conn, fn func(Status) error) error {
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		return errors.New("cannot acquire migration lock")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, lockID); err != nil {
			_ = conn.Close(unlockCtx)
		}
	}()
	status, err := r.Status(ctx, conn)
	if err != nil {
		return err
	}
	return fn(status)
}

func (r Runner) Apply(ctx context.Context, conn *pgx.Conn) error {
	return r.locked(ctx, conn, func(status Status) error {
		if len(status.Applied) == 0 && status.ExistingObjects > 0 {
			return errors.New("existing schema requires explicit audited adoption")
		}
		ledgerExists := status.LedgerExists
		for _, m := range status.Pending {
			tx, err := conn.Begin(ctx)
			if err != nil {
				return errors.New("cannot start migration transaction")
			}
			failed := func() error {
				if !ledgerExists {
					if _, err := tx.Exec(ctx, createLedger); err != nil {
						return errors.New("cannot create migration ledger")
					}
				}
				if _, err := tx.Exec(ctx, `SET LOCAL search_path = public, pg_catalog`); err != nil {
					return errors.New("cannot set migration schema")
				}
				if _, err := tx.Exec(ctx, m.SQL); err != nil {
					return fmt.Errorf("migration %03d failed; transaction rolled back", m.Version)
				}
				if _, err := tx.Exec(ctx, `INSERT INTO public.registry_schema_migrations(version,name,sha256,adopted) VALUES($1,$2,$3,false)`, m.Version, m.Name, m.SHA256); err != nil {
					return errors.New("cannot record applied migration")
				}
				if err := tx.Commit(ctx); err != nil {
					return errors.New("cannot commit migration; inspect status before retry")
				}
				return nil
			}()
			if failed != nil {
				rollback(tx)
				return failed
			}
			ledgerExists = true
		}
		return nil
	})
}

// Adopt records a prefix only after the operator has audited that baseline and
// obtained the exact current status fingerprints. Matching fingerprints detect
// drift; they do not prove schema equivalence, authorization or a valid backup.
func (r Runner) Adopt(ctx context.Context, conn *pgx.Conn, through int, manifest, schema string) error {
	if through < 1 || through > len(r.Migrations) || !validHash(manifest) || !validHash(schema) {
		return errors.New("adoption requires a valid prefix and both expected SHA256 fingerprints")
	}
	return r.locked(ctx, conn, func(status Status) error {
		if len(status.Applied) > 0 {
			return errors.New("cannot adopt an already recorded baseline")
		}
		if status.ExistingObjects == 0 {
			return errors.New("cannot adopt an empty schema")
		}
		if manifest != status.ManifestSHA256 || schema != status.SchemaSHA256 {
			return errors.New("adoption fingerprint mismatch")
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return errors.New("cannot start adoption transaction")
		}
		defer rollback(tx)
		if !status.LedgerExists {
			if _, err := tx.Exec(ctx, createLedger); err != nil {
				return errors.New("cannot create migration ledger")
			}
		}
		for _, m := range r.Migrations[:through] {
			if _, err := tx.Exec(ctx, `INSERT INTO public.registry_schema_migrations(version,name,sha256,adopted) VALUES($1,$2,$3,true)`, m.Version, m.Name, m.SHA256); err != nil {
				return errors.New("cannot record adopted baseline")
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return errors.New("cannot commit adoption; inspect status before retry")
		}
		return nil
	})
}
func validHash(raw string) bool {
	decoded, err := hex.DecodeString(raw)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == raw
}

// Cleanup uses an independent bounded deadline after a cancelled release task.
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
