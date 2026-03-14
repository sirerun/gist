package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
)

// CheckResult holds the outcome of a single diagnostic check.
type CheckResult struct {
	Name    string
	OK      bool
	Message string
	Skipped bool
}

func checkGoRuntime() CheckResult {
	return CheckResult{
		Name:    "Go runtime",
		OK:      true,
		Message: runtime.Version(),
	}
}

func checkBuildInfo() CheckResult {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return CheckResult{
			Name:    "Build info",
			OK:      false,
			Message: "unavailable",
		}
	}
	version := info.Main.Version
	if version == "" || version == "(devel)" {
		version = "(devel)"
	}
	return CheckResult{
		Name:    "Build info",
		OK:      true,
		Message: fmt.Sprintf("%s %s", info.Main.Path, version),
	}
}

func checkPlatform() CheckResult {
	return CheckResult{
		Name:    "Platform",
		OK:      true,
		Message: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

func checkPostgres(ctx context.Context, dsnVal string) CheckResult {
	if dsnVal == "" {
		return CheckResult{
			Name:    "PostgreSQL",
			Skipped: true,
			Message: "not configured (use --dsn or GIST_DSN)",
		}
	}

	start := time.Now()
	conn, err := pgx.Connect(ctx, dsnVal)
	if err != nil {
		return CheckResult{
			Name:    "PostgreSQL",
			OK:      false,
			Message: fmt.Sprintf("connection failed: %v", err),
		}
	}
	defer conn.Close(ctx)

	if err := conn.Ping(ctx); err != nil {
		return CheckResult{
			Name:    "PostgreSQL",
			OK:      false,
			Message: fmt.Sprintf("ping failed: %v", err),
		}
	}
	latency := time.Since(start)

	return CheckResult{
		Name:    "PostgreSQL",
		OK:      true,
		Message: fmt.Sprintf("connected (%s latency)", latency.Round(time.Millisecond)),
	}
}

func checkPgTrgm(ctx context.Context, dsnVal string) CheckResult {
	if dsnVal == "" {
		return CheckResult{
			Name:    "pg_trgm extension",
			Skipped: true,
			Message: "skipped (no database connection)",
		}
	}

	conn, err := pgx.Connect(ctx, dsnVal)
	if err != nil {
		return CheckResult{
			Name:    "pg_trgm extension",
			OK:      false,
			Message: fmt.Sprintf("connection failed: %v", err),
		}
	}
	defer conn.Close(ctx)

	var name string
	err = conn.QueryRow(ctx,
		"SELECT name FROM pg_available_extensions WHERE name = 'pg_trgm'",
	).Scan(&name)
	if err != nil {
		return CheckResult{
			Name:    "pg_trgm extension",
			OK:      false,
			Message: "not available",
		}
	}

	return CheckResult{
		Name:    "pg_trgm extension",
		OK:      true,
		Message: "available",
	}
}

func formatResult(r CheckResult) string {
	var marker string
	switch {
	case r.Skipped:
		marker = "[-]"
	case r.OK:
		marker = "[✓]"
	default:
		marker = "[✗]"
	}
	return fmt.Sprintf("%s %s: %s", marker, r.Name, r.Message)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check runtime environment and dependencies",
	// Override PersistentPreRunE from root so DSN is not required.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		fmt.Println("Gist Doctor")
		fmt.Println("===========")

		dsnVal := dsn
		if dsnVal == "" {
			dsnVal = os.Getenv("GIST_DSN")
		}

		checks := []CheckResult{
			checkGoRuntime(),
			checkBuildInfo(),
			checkPlatform(),
			checkPostgres(ctx, dsnVal),
			checkPgTrgm(ctx, dsnVal),
		}

		hasFailed := false
		for _, c := range checks {
			fmt.Println(formatResult(c))
			if !c.OK && !c.Skipped {
				hasFailed = true
			}
		}

		if hasFailed {
			return fmt.Errorf("one or more checks failed")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
