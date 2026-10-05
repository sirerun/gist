//go:build integration

package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

const (
	coreMaintenanceRowsAtBound = maxEventMaintenanceBatches * 1000
	coreMaintenanceBacklogRows = coreMaintenanceRowsAtBound + 1
)

func seedExpiredMaintenanceEvents(t *testing.T, fixture compositionPostgres, workspace string, count int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := fixture.adminPool.Exec(ctx, `INSERT INTO event_outbox (workspace_id,event_type,artifact_kind,artifact_id,artifact_version,policy_generation,occurred_at)
		SELECT $1,'version_published','skill','maintenance-fixture-'||g::text,'1.0.0',1,clock_timestamp()-interval '8 days'
		FROM generate_series(1,$2) AS g`, workspace, count)
	if err != nil {
		t.Fatalf("seed expired maintenance events: %v", err)
	}
}

func countExpiredMaintenanceEvents(t *testing.T, fixture compositionPostgres, workspace string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var count int64
	err := fixture.adminPool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1 AND occurred_at<clock_timestamp()-interval '7 days'`, workspace).Scan(&count)
	if err != nil {
		t.Fatalf("count expired maintenance events: %v", err)
	}
	return count
}

func TestCoreMaintenanceBoundsBacklogAndCatchesUp(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	seedExpiredMaintenanceEvents(t, fixture, compositionWorkspace, coreMaintenanceBacklogRows)

	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	first, err := New(ctx, cfg)
	if err == nil || first != nil || !strings.Contains(err.Error(), "bounded catch-up") {
		if first != nil {
			if shutdownErr := first.Shutdown(context.Background()); shutdownErr != nil {
				t.Errorf("shutdown unexpectedly started app: %v", shutdownErr)
			}
		}
		t.Fatalf("startup did not fail closed at the bounded backlog: app=%v err=%v", first, err)
	}
	if got := countExpiredMaintenanceEvents(t, fixture, compositionWorkspace); got != 1 {
		t.Fatalf("first startup exceeded its bounded catch-up or left an unexpected backlog: remaining=%d want=1", got)
	}

	second, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("startup did not catch up the final bounded remainder: %v", err)
	}
	if err := second.Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown caught-up app: %v", err)
	}
	if got := countExpiredMaintenanceEvents(t, fixture, compositionWorkspace); got != 0 {
		t.Fatalf("successful catch-up left expired events: %d", got)
	}
}

func TestCoreMaintenanceAcceptsBacklogAtBound(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	seedExpiredMaintenanceEvents(t, fixture, compositionWorkspace, coreMaintenanceRowsAtBound)

	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	instance, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("startup rejected backlog exactly at the bounded purge limit: %v", err)
	}
	if instance == nil {
		t.Fatal("startup returned no app at the bounded purge limit")
	}
	if err := instance.Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown after exact-limit catch-up: %v", err)
	}
	if got := countExpiredMaintenanceEvents(t, fixture, compositionWorkspace); got != 0 {
		t.Fatalf("successful exact-limit catch-up left expired events: %d", got)
	}
}

func TestCoreMaintenanceContinuesAfterTargetFailure(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	seedExpiredMaintenanceEvents(t, fixture, compositionWorkspace, 1)
	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	cfg.EventMaintenanceTargets = []MaintenanceTarget{
		{WorkspaceID: "missing-maintenance-workspace", Subject: "missing-maintainer"},
		{WorkspaceID: compositionWorkspace, Subject: compositionSubject},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	instance, err := New(ctx, cfg)
	if err == nil || instance != nil || !strings.Contains(err.Error(), "maintenance membership unavailable") {
		if instance != nil {
			if shutdownErr := instance.Shutdown(context.Background()); shutdownErr != nil {
				t.Errorf("shutdown unexpectedly started app: %v", shutdownErr)
			}
		}
		t.Fatalf("startup did not report the invalid maintenance target: app=%v err=%v", instance, err)
	}
	if got := countExpiredMaintenanceEvents(t, fixture, compositionWorkspace); got != 0 {
		t.Fatalf("valid target was starved by an earlier failed target; remaining=%d", got)
	}
}

func TestCoreEventJanitorStopsWithParentAndShutdownClosesPool(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	parent, cancelParent := context.WithCancel(context.Background())
	instance, err := New(parent, cfg)
	if err != nil {
		cancelParent()
		t.Fatalf("start app with valid maintenance target: %v", err)
	}
	t.Cleanup(func() {
		if err := instance.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown app during cleanup: %v", err)
		}
	})

	cancelParent()
	select {
	case <-instance.janitorDone:
	case <-time.After(5 * time.Second):
		t.Fatal("event janitor did not stop after parent cancellation")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := instance.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown after parent cancellation: %v", err)
	}
	poolCtx, cancelPool := context.WithTimeout(context.Background(), time.Second)
	defer cancelPool()
	conn, err := instance.pool.Acquire(poolCtx)
	if err == nil {
		conn.Release()
		t.Fatal("application pool accepted a connection after shutdown")
	}
}

func TestCoreMaintenanceStartupUsesConfiguredDeadline(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	cfg.RequestTimeout = 250 * time.Millisecond

	lockCtx, cancelLock := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelLock()
	lockTx, err := fixture.adminPool.Begin(lockCtx)
	if err != nil {
		t.Fatalf("begin maintenance table lock: %v", err)
	}
	lockHeld := true
	releaseLock := func() {
		if !lockHeld {
			return
		}
		lockHeld = false
		if err := lockTx.Rollback(context.Background()); err != nil {
			t.Errorf("release maintenance table lock: %v", err)
		}
	}
	type startupResult struct {
		instance *App
		err      error
	}
	result := make(chan startupResult, 1)
	resultReceived := false
	t.Cleanup(func() {
		releaseLock()
		if resultReceived {
			return
		}
		select {
		case got := <-result:
			if got.instance != nil {
				if shutdownErr := got.instance.Shutdown(context.Background()); shutdownErr != nil {
					t.Errorf("shutdown app returned during test cleanup: %v", shutdownErr)
				}
			}
		case <-time.After(5 * time.Second):
			t.Error("startup did not return after test cleanup released the lock")
		}
	})
	if _, err := lockTx.Exec(lockCtx, "LOCK TABLE workspace_memberships IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatalf("lock maintenance target table: %v", err)
	}

	start := time.Now()
	startupCtx := context.Background()
	if _, hasDeadline := startupCtx.Deadline(); hasDeadline {
		t.Fatal("startup test caller context unexpectedly has a deadline")
	}
	go func() {
		instance, err := New(startupCtx, cfg)
		result <- startupResult{instance: instance, err: err}
	}()

	waitCtx, cancelWait := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelWait()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	waitingForLock := false
	for !waitingForLock {
		if err := fixture.adminPool.QueryRow(waitCtx, `SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity
			WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%workspace_memberships%'
		)`).Scan(&waitingForLock); err != nil {
			releaseLock()
			t.Fatalf("observe startup waiting on maintenance target lock: %v", err)
		}
		if waitingForLock {
			break
		}
		select {
		case <-ticker.C:
		case <-waitCtx.Done():
			releaseLock()
			t.Fatalf("startup never reached the locked maintenance target query: %v", waitCtx.Err())
		}
	}

	var got startupResult
	select {
	case got = <-result:
		resultReceived = true
	case <-time.After(3 * time.Second):
		releaseLock()
		select {
		case got = <-result:
			resultReceived = true
			if got.instance != nil {
				if shutdownErr := got.instance.Shutdown(context.Background()); shutdownErr != nil {
					t.Errorf("shutdown app after outer timeout: %v", shutdownErr)
				}
			}
		case <-time.After(5 * time.Second):
			t.Fatal("startup remained blocked after releasing the fixture lock")
		}
		t.Fatal("startup ignored configured maintenance timeout")
	}
	releaseLock()
	if got.instance != nil || got.err == nil || !strings.Contains(got.err.Error(), "maintenance membership unavailable") {
		if got.instance != nil {
			if shutdownErr := got.instance.Shutdown(context.Background()); shutdownErr != nil {
				t.Errorf("shutdown unexpectedly started app: %v", shutdownErr)
			}
		}
		t.Fatalf("startup did not fail on its configured timeout: app=%v err=%v", got.instance, got.err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("configured startup timeout returned too slowly: %s", elapsed)
	}
}
