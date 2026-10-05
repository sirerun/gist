//go:build integration

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestPostgresEventCursorBudgetDurabilityAndPolicyBinding(t *testing.T) {
	adminPool := searchTestPool(t)
	pool := restrictedEventPool(t, adminPool)
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := ports.Principal{Issuer: "issuer-a", Subject: "subject-a", Audience: "audience-a", WorkspaceID: "events-a", PolicyGeneration: 7}
	cursor, err := store.NewEventCursor(ctx, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEventPage(ctx, cursor, 1); !errors.Is(err, ErrEventPrincipalRequired) {
		t.Fatalf("legacy reader error=%v, want principal-required fail-closed", err)
	}
	for i := 0; i < MaxEventsPerPage+5; i++ {
		if err := store.Append(ctx, ports.Event{WorkspaceID: p.WorkspaceID, Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: "skill", ID: "item", Version: fmt.Sprintf("1.0.%d", i)}, PolicyGeneration: p.PolicyGeneration}); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}
	var visibleWithoutTenant int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox`).Scan(&visibleWithoutTenant); err != nil {
		t.Fatal(err)
	}
	if visibleWithoutTenant != 0 {
		t.Fatalf("forced RLS exposed %d events without tenant context", visibleWithoutTenant)
	}
	if _, err := store.ReadEventPageForPrincipal(ctx, p, cursor, 128); !errors.Is(err, ErrEventBudget) {
		t.Fatalf("small response budget error=%v, want ErrEventBudget", err)
	}
	if _, err := store.ReadEventPageForPrincipal(ctx, p, cursor, 0); !errors.Is(err, ErrEventBudget) {
		t.Fatalf("zero response budget error=%v, want ErrEventBudget", err)
	}
	page, err := store.ReadEventPageForPrincipal(ctx, p, cursor, 1<<20)
	if err != nil || len(page.Events) != MaxEventsPerPage || page.Next.ID == cursor.ID {
		t.Fatalf("first page count=%d next=%q err=%v", len(page.Events), page.Next.ID, err)
	}
	var eventWire map[string]json.RawMessage
	wireBytes, err := json.Marshal(page.Events[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wireBytes, &eventWire); err != nil {
		t.Fatal(err)
	}
	var subjectRef string
	var occurredAt time.Time
	var generation uint64
	if len(eventWire) != 5 || json.Unmarshal(eventWire["subject_ref"], &subjectRef) != nil ||
		json.Unmarshal(eventWire["occurred_at"], &occurredAt) != nil || json.Unmarshal(eventWire["policy_generation"], &generation) != nil ||
		len(eventWire["event_id"]) == 0 || len(eventWire["event_type"]) == 0 || subjectRef != "item@1.0.0" || occurredAt.IsZero() || generation != p.PolicyGeneration {
		t.Fatalf("event did not match canonical wire envelope: %s", wireBytes)
	}
	encodedPage, err := marshalEventPage(page.Events, page.Next.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEventPageForPrincipal(ctx, p, cursor, len(encodedPage)-1); !errors.Is(err, ErrEventBudget) {
		t.Fatalf("one-byte-short canonical page budget error=%v, want ErrEventBudget", err)
	}
	exactPage, err := store.ReadEventPageForPrincipal(ctx, p, cursor, len(encodedPage))
	if err != nil || len(exactPage.Events) != MaxEventsPerPage {
		t.Fatalf("exact canonical page budget events=%d err=%v", len(exactPage.Events), err)
	}
	// The input token remains an immutable retry point, and therefore the
	// undisplayed tail cannot be lost when a caller rejects an oversized body.
	retry, err := store.ReadEventPageForPrincipal(ctx, p, cursor, 1<<20)
	if err != nil || len(retry.Events) != len(page.Events) || retry.Events[0].ID != page.Events[0].ID {
		t.Fatalf("retry count=%d first=%v err=%v", len(retry.Events), retry.Events[:min(1, len(retry.Events))], err)
	}

	changed := p
	changed.PolicyGeneration++
	if _, err := store.ReadEventPageForPrincipal(ctx, changed, page.Next, 1<<20); !errors.Is(err, ErrEventCursorBinding) {
		t.Fatalf("changed policy generation error=%v, want binding denial", err)
	}
	foreign := p
	foreign.Subject = "foreign-subject"
	if _, err := store.ReadEventPageForPrincipal(ctx, foreign, page.Next, 1<<20); !errors.Is(err, ErrEventCursorBinding) {
		t.Fatalf("foreign principal error=%v, want uniform binding denial", err)
	}
	foreignWorkspace := p
	foreignWorkspace.WorkspaceID = "events-other"
	if _, err := store.ReadEventPageForPrincipal(ctx, foreignWorkspace, page.Next, 1<<20); !errors.Is(err, ErrEventCursorBinding) {
		t.Fatalf("foreign workspace error=%v, want uniform binding denial", err)
	}
	missing := page.Next
	missing.ID = "missing-cursor"
	if _, err := store.ReadEventPageForPrincipal(ctx, p, missing, 1<<20); !errors.Is(err, ErrEventCursorBinding) {
		t.Fatalf("missing cursor error=%v, want uniform binding denial", err)
	}

	// A separately constructed pool/store sees the durable continuation.
	clone := pool.Config()
	clone.ConnConfig = pool.Config().ConnConfig.Copy()
	otherPool, err := pgxpool.NewWithConfig(ctx, clone)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(otherPool.Close)
	otherStore, err := NewPostgres(otherPool)
	if err != nil {
		t.Fatal(err)
	}
	last, err := otherStore.ReadEventPageForPrincipal(ctx, p, page.Next, 1<<20)
	if err != nil || len(last.Events) != 5 {
		t.Fatalf("restarted continuation count=%d err=%v", len(last.Events), err)
	}

	// Budget failures at the 16-cursor cap must not mutate any cursor or evict
	// the oldest retry point, including after repeated failures.
	capPrincipal := ports.Principal{Issuer: "issuer-a", Subject: "cap-subject", Audience: "audience-a", WorkspaceID: "events-cap", PolicyGeneration: 1}
	var oldest ports.Cursor
	for i := 0; i < MaxEventCursors; i++ {
		c, err := store.NewEventCursor(ctx, capPrincipal, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			oldest = c
		}
	}
	if err := store.Append(ctx, ports.Event{WorkspaceID: capPrincipal.WorkspaceID, Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: capPrincipal.WorkspaceID, Kind: "skill", ID: "large-payload", Version: "1.0.0"}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := store.ReadEventPageForPrincipal(ctx, capPrincipal, oldest, 1); !errors.Is(err, ErrEventBudget) {
			t.Fatalf("budget retry %d error=%v, want ErrEventBudget", i, err)
		}
	}
	if err := WithTenant(ctx, pool, capPrincipal.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var count int
		var present bool
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_cursors WHERE principal_hash=$1`, ports.PrincipalHash(capPrincipal)).Scan(&count); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_cursors WHERE id=$1)`, oldest.ID).Scan(&present); err != nil {
			return err
		}
		if count != MaxEventCursors || !present {
			return fmt.Errorf("failed budget reads changed cursor cap: count=%d oldest_present=%v", count, present)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.NewEventCursor(ctx, capPrincipal, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEventPageForPrincipal(ctx, capPrincipal, oldest, 1<<20); !errors.Is(err, ErrEventCursorBinding) {
		t.Fatalf("oldest cursor after seventeenth open error=%v, want denial", err)
	}
}

func TestPostgresOpenEventPageBudgetFailureDoesNotMutateCursorCap(t *testing.T) {
	pool := restrictedEventPool(t, searchTestPool(t))
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := ports.Principal{Issuer: "issuer", Subject: "open-budget", Audience: "audience", WorkspaceID: "events-open-budget", PolicyGeneration: 1}
	cursors := make([]ports.Cursor, MaxEventCursors)
	for i := range cursors {
		cursors[i], err = store.NewEventCursor(ctx, p, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
	}
	oldest := cursors[0]
	second := cursors[1]
	emptyEnvelope, err := marshalEventPage([]ports.Event{}, strings.Repeat("x", 32))
	if err != nil {
		t.Fatal(err)
	}
	emptyBudget := len(emptyEnvelope)
	var beforeExpiry time.Time
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT expires_at FROM event_cursors WHERE id=$1`, oldest.ID).Scan(&beforeExpiry)
	}); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		for _, budget := range []int{emptyBudget - 1, 1, 0, -1} {
			if _, err := store.OpenEventPageForPrincipal(ctx, p, budget); !errors.Is(err, ErrEventBudget) {
				t.Fatalf("open attempt %d budget %d error=%v, want ErrEventBudget", attempt, budget, err)
			}
		}
	}
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var count int
		var present, sameExpiry bool
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_cursors WHERE principal_hash=$1`, ports.PrincipalHash(p)).Scan(&count); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_cursors WHERE id=$1)`, oldest.ID).Scan(&present); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT expires_at=$2 FROM event_cursors WHERE id=$1`, oldest.ID, beforeExpiry).Scan(&sameExpiry); err != nil {
			return err
		}
		if count != MaxEventCursors || !present || !sameExpiry {
			return fmt.Errorf("failed opens changed cursor set: count=%d oldest_present=%v expiry_unchanged=%v", count, present, sameExpiry)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEventPageForPrincipal(ctx, p, oldest, 1<<20); err != nil {
		t.Fatalf("oldest cursor could not be used after budget denial: %v", err)
	}
	page, err := store.OpenEventPageForPrincipal(ctx, p, emptyBudget)
	if err != nil || page.Next.ID == "" || page.Events == nil || len(page.Events) != 0 {
		t.Fatalf("qualified first page=%+v err=%v", page, err)
	}
	encodedEmpty, err := marshalEventPage(page.Events, page.Next.ID)
	if err != nil || len(encodedEmpty) != emptyBudget {
		t.Fatalf("empty page wire bytes=%d expected=%d marshal_err=%v", len(encodedEmpty), emptyBudget, err)
	}
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var count int
		var oldestPresent, secondPresent, openedPresent bool
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_cursors WHERE principal_hash=$1`, ports.PrincipalHash(p)).Scan(&count); err != nil {
			return err
		}
		for _, item := range []struct {
			id string
			to *bool
		}{{oldest.ID, &oldestPresent}, {second.ID, &secondPresent}, {page.Next.ID, &openedPresent}} {
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_cursors WHERE id=$1)`, item.id).Scan(item.to); err != nil {
				return err
			}
		}
		if count != MaxEventCursors || oldestPresent || !secondPresent || !openedPresent {
			return fmt.Errorf("qualified open eviction: count=%d oldest=%v second=%v opened=%v", count, oldestPresent, secondPresent, openedPresent)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresEventCursorSlidingExpiryAndExpiryDenial(t *testing.T) {
	pool := restrictedEventPool(t, searchTestPool(t))
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := ports.Principal{Issuer: "issuer", Subject: "expiry", Audience: "audience", WorkspaceID: "events-expiry", PolicyGeneration: 1}
	active, err := store.NewEventCursor(ctx, p, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE event_cursors SET expires_at=now()+interval '1 second' WHERE id=$1`, active.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEventPageForPrincipal(ctx, p, active, 1<<20); err != nil {
		t.Fatal(err)
	}
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var expires time.Time
		if err := tx.QueryRow(ctx, `SELECT expires_at FROM event_cursors WHERE id=$1`, active.ID).Scan(&expires); err != nil {
			return err
		}
		if time.Until(expires) < 4*time.Minute {
			return fmt.Errorf("read did not slide expiry by five minutes: %s", expires)
		}
		_, err := tx.Exec(ctx, `UPDATE event_cursors SET expires_at=now()-interval '1 second' WHERE id=$1`, active.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEventPageForPrincipal(ctx, p, active, 1<<20); !errors.Is(err, ErrEventCursorExpired) {
		t.Fatalf("expired cursor error=%v, want ErrEventCursorExpired", err)
	}
}

func TestPostgresEventRetentionFloorIsWorkspaceSpecific(t *testing.T) {
	pool := restrictedEventPool(t, searchTestPool(t))
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a := ports.Principal{Issuer: "issuer", Subject: "a", Audience: "audience", WorkspaceID: "floor-a", PolicyGeneration: 1}
	b := ports.Principal{Issuer: "issuer", Subject: "b", Audience: "audience", WorkspaceID: "floor-b", PolicyGeneration: 1}
	oldA, err := store.NewEventCursor(ctx, a, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, ports.Event{WorkspaceID: b.WorkspaceID, Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: b.WorkspaceID, Kind: "skill", ID: "old-b", Version: "1.0.0"}}); err != nil {
		t.Fatal(err)
	}
	markEventOlderThanRetention(t, ctx, pool, b.WorkspaceID, "old-b")
	if removed, err := store.PurgeEvents(ctx, Tenant{WorkspaceID: b.WorkspaceID}, MaxEventPurgeBatch); err != nil || removed != 1 {
		t.Fatalf("purge workspace B removed=%d err=%v", removed, err)
	}
	page, err := store.ReadEventPageForPrincipal(ctx, a, oldA, 1<<20)
	if err != nil || page.RetentionGap {
		t.Fatalf("workspace B sequence gap invalidated workspace A cursor: gap=%v err=%v", page.RetentionGap, err)
	}

	oldSameWorkspace, err := store.NewEventCursor(ctx, a, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, ports.Event{WorkspaceID: a.WorkspaceID, Type: ports.EventVersionRevoked, Subject: ports.ArtifactRef{WorkspaceID: a.WorkspaceID, Kind: "skill", ID: "old-a", Version: "1.0.0"}}); err != nil {
		t.Fatal(err)
	}
	markEventOlderThanRetention(t, ctx, pool, a.WorkspaceID, "old-a")
	if removed, err := store.PurgeEvents(ctx, Tenant{WorkspaceID: a.WorkspaceID}, MaxEventPurgeBatch); err != nil || removed != 1 {
		t.Fatalf("purge workspace A removed=%d err=%v", removed, err)
	}
	page, err = store.ReadEventPageForPrincipal(ctx, a, oldSameWorkspace, 1<<20)
	if !errors.Is(err, ErrEventCursorExpired) || !page.RetentionGap {
		t.Fatalf("same-workspace purge gap=%v err=%v, want retention expiry", page.RetentionGap, err)
	}
}

func TestPostgresEventPurgeIsTenantScopedAndBatchBounded(t *testing.T) {
	pool := restrictedEventPool(t, searchTestPool(t))
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		workspace := "purge-a"
		if i == 2 {
			workspace = "purge-b"
		}
		if err := store.Append(ctx, ports.Event{WorkspaceID: workspace, Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: workspace, Kind: "skill", ID: fmt.Sprintf("old-%d", i), Version: "1.0.0"}}); err != nil {
			t.Fatal(err)
		}
		markEventOlderThanRetention(t, ctx, pool, workspace, fmt.Sprintf("old-%d", i))
	}
	if _, err := store.PurgeEvents(ctx, Tenant{WorkspaceID: "purge-a"}, MaxEventPurgeBatch+1); err == nil {
		t.Fatal("oversized purge batch was accepted")
	}
	removed, err := store.PurgeEvents(ctx, Tenant{WorkspaceID: "purge-a"}, 1)
	if err != nil || removed != 1 {
		t.Fatalf("bounded purge removed=%d err=%v, want one row", removed, err)
	}
	if err := WithTenant(ctx, pool, "purge-a", func(ctx context.Context, tx pgx.Tx) error {
		var remaining, floor int64
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id='purge-a'`).Scan(&remaining); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT through_event_id FROM event_retention_floors WHERE workspace_id='purge-a'`).Scan(&floor); err != nil {
			return err
		}
		if remaining != 1 || floor <= 0 {
			return fmt.Errorf("bounded purge remaining=%d floor=%d", remaining, floor)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if removed, err := store.PurgeEvents(ctx, Tenant{WorkspaceID: "purge-a"}, 100); err != nil || removed != 1 {
		t.Fatalf("second purge removed=%d err=%v", removed, err)
	}
	if err := WithTenant(ctx, pool, "purge-b", func(ctx context.Context, tx pgx.Tx) error {
		var remaining int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id='purge-b'`).Scan(&remaining); err != nil {
			return err
		}
		if remaining != 1 {
			return fmt.Errorf("purge for workspace A removed %d rows from workspace B", 1-remaining)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresEventCursorExpiryUsesDatabaseClockAfterLockWait(t *testing.T) {
	adminPool := searchTestPool(t)
	pool := restrictedEventPool(t, adminPool)
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := ports.Principal{Issuer: "issuer", Subject: "clock-wait", Audience: "audience", WorkspaceID: "events-clock", PolicyGeneration: 1}
	cursor, err := store.NewEventCursor(ctx, p, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE event_cursors SET expires_at=clock_timestamp()+interval '700 milliseconds' WHERE id=$1`, cursor.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	locker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := setTenantContext(ctx, locker, Tenant{WorkspaceID: p.WorkspaceID}); err != nil {
		_ = locker.Rollback(ctx)
		t.Fatal(err)
	}
	if err := lockCursorPrincipal(ctx, locker, cursor.PrincipalHash); err != nil {
		_ = locker.Rollback(ctx)
		t.Fatal(err)
	}
	readResult := make(chan error, 1)
	go func() {
		_, err := store.ReadEventPageForPrincipal(ctx, p, cursor, 1<<20)
		readResult <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var waiting bool
		if err := adminPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory' AND query LIKE '%pg_advisory_xact_lock%')`).Scan(&waiting); err != nil {
			_ = locker.Rollback(ctx)
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			_ = locker.Rollback(ctx)
			t.Fatal("reader did not wait for principal advisory lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(800 * time.Millisecond)
	if err := locker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-readResult; !errors.Is(err, ErrEventCursorExpired) {
		t.Fatalf("read after lock wait error=%v, want expiry by database clock", err)
	}

	// A second connection waits on the same principal lock before opening a
	// cursor. Its expiry must be five minutes from insertion, not five minutes
	// from the transaction's start before the wait.
	writerPrincipal := ports.Principal{Issuer: "issuer", Subject: "clock-write", Audience: "audience", WorkspaceID: "events-clock-write", PolicyGeneration: 1}
	writerHash := ports.PrincipalHash(writerPrincipal)
	writerLock, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := setTenantContext(ctx, writerLock, Tenant{WorkspaceID: writerPrincipal.WorkspaceID}); err != nil {
		_ = writerLock.Rollback(ctx)
		t.Fatal(err)
	}
	if err := lockCursorPrincipal(ctx, writerLock, writerHash); err != nil {
		_ = writerLock.Rollback(ctx)
		t.Fatal(err)
	}
	type cursorResult struct {
		cursor ports.Cursor
		err    error
	}
	created := make(chan cursorResult, 1)
	go func() {
		c, err := store.NewEventCursor(ctx, writerPrincipal, EventCursorTTL)
		created <- cursorResult{cursor: c, err: err}
	}()
	deadline = time.Now().Add(2 * time.Second)
	for {
		var waiting bool
		if err := adminPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory' AND query LIKE '%pg_advisory_xact_lock%')`).Scan(&waiting); err != nil {
			_ = writerLock.Rollback(ctx)
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			_ = writerLock.Rollback(ctx)
			t.Fatal("cursor opener did not wait for principal advisory lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(1100 * time.Millisecond)
	if err := writerLock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	result := <-created
	if result.err != nil {
		t.Fatal(result.err)
	}
	var enoughTTL bool
	if err := WithTenant(ctx, pool, writerPrincipal.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT expires_at > clock_timestamp()+interval '4 minutes 59 seconds' FROM event_cursors WHERE id=$1`, result.cursor.ID).Scan(&enoughTTL)
	}); err != nil {
		t.Fatal(err)
	}
	if !enoughTTL {
		t.Fatal("cursor TTL was shortened by lock wait before insertion")
	}
}

func setTenantContext(ctx context.Context, tx pgx.Tx, tenant Tenant) error {
	settings := []struct{ name, value string }{
		{settingIssuer, tenant.Issuer}, {settingSubject, tenant.Subject}, {settingAudience, tenant.Audience},
		{settingWorkspace, tenant.WorkspaceID}, {settingScopes, joinScopes(tenant.Scopes)},
		{settingPolicyGeneration, fmt.Sprint(tenant.PolicyGeneration)},
	}
	for _, setting := range settings {
		if _, err := tx.Exec(ctx, `SELECT set_config($1,$2,true)`, setting.name, setting.value); err != nil {
			return err
		}
	}
	return nil
}

func markEventOlderThanRetention(t *testing.T, ctx context.Context, pool *pgxpool.Pool, workspace, artifactID string) {
	t.Helper()
	if err := WithTenant(ctx, pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE event_outbox SET occurred_at=clock_timestamp()-interval '8 days' WHERE workspace_id=$1 AND artifact_id=$2`, workspace, artifactID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAppendEventInTransactionCommitAndRollback(t *testing.T) {
	adminPool := searchTestPool(t)
	pool := restrictedEventPool(t, adminPool)
	ctx := context.Background()
	workspace := "append-tx"
	event := ports.Event{WorkspaceID: workspace, Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: workspace, Kind: "skill", ID: "published", Version: "1.0.0"}}
	for i := 0; i < 2; i++ {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := setTenantContext(ctx, tx, Tenant{WorkspaceID: workspace}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if i == 0 {
			// Establish the transaction before the delay. The persisted event
			// timestamp must be insertion time, not PostgreSQL now() at BEGIN.
			time.Sleep(1100 * time.Millisecond)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,owner_id)
			VALUES($1,'skill','published','1.0.0','published','sha256',$2,'sha256',$2,'{}','owner')`, workspace, fmt.Sprintf("%064x", 1)); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err := AppendEventInTransaction(ctx, tx, event); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if i == 0 {
			var current bool
			if err := tx.QueryRow(ctx, `SELECT occurred_at > clock_timestamp()-interval '100 milliseconds' FROM event_outbox WHERE workspace_id=$1`, workspace).Scan(&current); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if !current {
				_ = tx.Rollback(ctx)
				t.Fatal("outbox timestamp reflects transaction start rather than insertion time")
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		} else if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var count, catalogCount int
	if err := WithTenant(ctx, pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1`, workspace).Scan(&count); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM catalog_versions WHERE workspace_id=$1 AND artifact_id='published'`, workspace).Scan(&catalogCount)
	}); err != nil {
		t.Fatal(err)
	}
	if count != 1 || catalogCount != 1 {
		t.Fatalf("caller transaction committed %d events and %d catalog rows after one rollback and one commit", count, catalogCount)
	}
	clone := pool.Config()
	clone.ConnConfig = pool.Config().ConnConfig.Copy()
	otherPool, err := pgxpool.NewWithConfig(ctx, clone)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(otherPool.Close)
	if err := WithTenant(ctx, otherPool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		var persisted, catalogPersisted int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1`, workspace).Scan(&persisted); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM catalog_versions WHERE workspace_id=$1 AND artifact_id='published'`, workspace).Scan(&catalogPersisted); err != nil {
			return err
		}
		if persisted != 1 || catalogPersisted != 1 {
			return fmt.Errorf("second storage instance observed %d committed events and %d catalog rows", persisted, catalogPersisted)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRevocationAndOutboxAreAtomic(t *testing.T) {
	adminPool := searchTestPool(t)
	pool := restrictedEventPool(t, adminPool)
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := ports.Principal{Issuer: "issuer", Subject: "maintainer", Audience: "audience", WorkspaceID: "revoke-a", PolicyGeneration: 42}
	ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: "skill", ID: "revoke-item", Version: "1.0.0"}
	seedCatalogVersion(t, ctx, pool, ref)
	if _, err := adminPool.Exec(ctx, `CREATE FUNCTION fail_revocation_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected outbox failure'; END $$; CREATE TRIGGER fail_revocation_outbox BEFORE INSERT ON event_outbox FOR EACH ROW WHEN (NEW.event_type='version_revoked') EXECUTE FUNCTION fail_revocation_outbox()`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RevokeVersionForPrincipal(ctx, p, ref); err == nil {
		t.Fatal("expected injected outbox failure")
	}
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&state); err != nil {
			return err
		}
		if state != "published" {
			return fmt.Errorf("failed outbox insert committed state %q", state)
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1 AND artifact_id=$2 AND event_type='version_revoked'`, ref.WorkspaceID, ref.ID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("failed revoke left %d outbox rows", count)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `DROP TRIGGER fail_revocation_outbox ON event_outbox; DROP FUNCTION fail_revocation_outbox()`); err != nil {
		t.Fatal(err)
	}
	first, err := store.RevokeVersionForPrincipal(ctx, p, ref)
	if err != nil || !first.Created {
		t.Fatalf("first revoke=%+v err=%v", first, err)
	}
	second, err := store.RevokeVersionForPrincipal(ctx, p, ref)
	if err != nil || second.Created || !second.RevokedAt.Equal(first.RevokedAt) {
		t.Fatalf("repeat revoke=%+v err=%v", second, err)
	}
	if err := WithTenant(ctx, pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var count int
		var generation uint64
		if err := tx.QueryRow(ctx, `SELECT count(*), max(policy_generation) FROM event_outbox WHERE workspace_id=$1 AND artifact_id=$2 AND event_type='version_revoked'`, ref.WorkspaceID, ref.ID).Scan(&count, &generation); err != nil {
			return err
		}
		if count != 1 || generation != p.PolicyGeneration {
			return fmt.Errorf("outbox rows=%d policy_generation=%d", count, generation)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresUnavailableEventStoreFailsClosed(t *testing.T) {
	pool := restrictedEventPool(t, searchTestPool(t))
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	if err := store.Append(context.Background(), ports.Event{WorkspaceID: "offline", Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: "offline", Kind: "skill", ID: "x", Version: "1.0.0"}}); err == nil {
		t.Fatal("append unexpectedly succeeded after the PostgreSQL pool closed")
	}
	if _, err := store.NewEventCursor(context.Background(), ports.Principal{Issuer: "i", Subject: "s", WorkspaceID: "offline"}, time.Minute); err == nil {
		t.Fatal("cursor unexpectedly opened after the PostgreSQL pool closed")
	}
}

func restrictedEventPool(t *testing.T, adminPool *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	role := fmt.Sprintf("gist_evt_runtime_%x", time.Now().UnixNano())
	var currentUser string
	if err := adminPool.QueryRow(ctx, `SELECT current_user`).Scan(&currentUser); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `CREATE ROLE `+role+` NOLOGIN NOSUPERUSER NOBYPASSRLS`); err != nil {
		t.Fatalf("create isolated non-bypass runtime role: %v", err)
	}
	if _, err := adminPool.Exec(ctx, `GRANT SELECT,INSERT,UPDATE,DELETE ON event_outbox,event_cursors,event_retention_floors,catalog_versions TO `+role); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `GRANT USAGE,SELECT ON SEQUENCE event_outbox_event_id_seq,event_cursors_cursor_order_seq TO `+role); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `GRANT `+role+` TO `+pgx.Identifier{currentUser}.Sanitize()); err != nil {
		t.Fatalf("grant isolated runtime role to fixture owner: %v", err)
	}
	config := adminPool.Config()
	config.ConnConfig = adminPool.Config().ConnConfig.Copy()
	priorAfterConnect := config.AfterConnect
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if priorAfterConnect != nil {
			if err := priorAfterConnect(ctx, conn); err != nil {
				return err
			}
		}
		_, err := conn.Exec(ctx, `SET ROLE `+role)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), `DROP OWNED BY `+role)
		_, _ = adminPool.Exec(context.Background(), `DROP ROLE IF EXISTS `+role)
	})
	return pool
}

func seedCatalogVersion(t *testing.T, ctx context.Context, pool *pgxpool.Pool, ref ports.ArtifactRef) {
	t.Helper()
	err := WithTenant(ctx, pool, ref.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,owner_id)
			VALUES($1,$2,$3,$4,'published','sha256',$5,'sha256',$5,'{}','owner')`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version, fmt.Sprintf("%064x", 1))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
