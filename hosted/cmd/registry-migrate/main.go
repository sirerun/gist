// registry-migrate is an explicit one-shot release tool. With no flags it only
// inspects schema definitions; the registry server never calls this binary.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/migrate"
	"github.com/sirerun/gist/hosted/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	status := flag.Bool("status", false, "read-only schema status (default)")
	apply := flag.Bool("apply", false, "apply pending embedded forward migrations")
	provision := flag.Bool("provision-runtime-role", false, "provision the reviewed restricted runtime role after migration apply")
	adopt := flag.Int("adopt-through", 0, "record an operator-audited existing migration prefix; never runs its SQL")
	manifest := flag.String("expected-manifest-sha256", "", "required migration manifest SHA256 for adoption")
	schema := flag.String("expected-schema-sha256", "", "required current schema SHA256 for adoption")
	timeout := flag.Duration("timeout", 5*time.Minute, "overall deadline, at most 15 minutes")
	flag.Parse()
	if flag.NArg() != 0 || (*status && (*apply || *adopt != 0 || *provision)) || (*apply && (*adopt != 0 || *provision)) || (*provision && *adopt != 0) || *adopt < 0 || (*adopt == 0 && (*manifest != "" || *schema != "")) || *timeout <= 0 || *timeout > 15*time.Minute {
		return fmt.Errorf("invalid migration flags; choose status, apply, or explicit audited adoption")
	}
	if os.Getenv("GIST_DATABASE_URL") == "" {
		return fmt.Errorf("GIST_DATABASE_URL is required")
	}
	runner, err := migrate.Load(migrations.Files)
	if err != nil {
		return err
	}
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, *timeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("GIST_DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("cannot connect to migration database")
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	switch {
	case *provision:
		err = runner.ProvisionRuntimeRole(ctx, conn, os.Getenv("GIST_RUNTIME_DATABASE_URL"))
	case *apply:
		err = runner.Apply(ctx, conn)
	case *adopt > 0:
		err = runner.Adopt(ctx, conn, *adopt, *manifest, *schema)
	}
	if err != nil {
		return err
	}
	report, err := runner.Status(ctx, conn)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return fmt.Errorf("cannot write migration status")
	}
	return nil
}
