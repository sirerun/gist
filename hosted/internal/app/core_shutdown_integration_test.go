//go:build integration

package app

import (
	"context"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/storage"
)

func TestCoreShutdownCancelsBlockedMaintenanceQueryBeforeClosingPool(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	cfg.RequestTimeout = 30 * time.Second
	a, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.Shutdown(ctx); err != nil {
			t.Errorf("cleanup shutdown: %v", err)
		}
	})
	// Replace the idle periodic clock with a controlled tick; both use the
	// same production maintenance loop and real restricted PostgreSQL pool.
	a.janitorCancel()
	select {
	case <-a.janitorDone:
	case <-time.After(5 * time.Second):
		t.Fatal("idle maintenance loop did not stop")
	}
	catalog, err := storage.NewPostgres(a.pool)
	if err != nil {
		t.Fatal(err)
	}
	maintenanceCtx, cancelMaintenance := context.WithCancel(context.Background())
	a.janitorCancel = cancelMaintenance
	a.janitorDone = make(chan struct{})
	ticks := make(chan time.Time, 1)
	go func() {
		defer close(a.janitorDone)
		a.runEventJanitor(maintenanceCtx, catalog, ticks)
	}()
	lockCtx, cancelLock := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelLock()
	lock, err := fixture.adminPool.Begin(lockCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Rollback(context.Background()); err != nil {
			t.Errorf("release owned maintenance lock: %v", err)
		}
	}()
	if _, err := lock.Exec(lockCtx, "LOCK TABLE workspace_memberships IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	ticks <- time.Now()
	observeCtx, cancelObserve := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelObserve()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		var blocked bool
		if err := fixture.adminPool.QueryRow(observeCtx, `SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
			AND wait_event_type='Lock' AND query LIKE '%workspace_memberships%'
		)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case <-poll.C:
		case <-observeCtx.Done():
			t.Fatal("maintenance query never waited for the owned table lock")
		}
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), time.Second)
	defer cancelShutdown()
	if err := a.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown while maintenance query was locked: %v", err)
	}
	// The table lock remains held: success cannot come from unblocking the query.
	select {
	case <-a.janitorDone:
	default:
		t.Fatal("pool was closed before maintenance loop finished")
	}
	if a.pool.Stat().AcquiredConns() != 0 {
		t.Fatal("shutdown retained an acquired maintenance connection")
	}
	if conn, err := a.pool.Acquire(shutdownCtx); err == nil {
		conn.Release()
		t.Fatal("closed application pool accepted a connection")
	}
}
