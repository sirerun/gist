package storage

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/events"
	"github.com/sirerun/gist/hosted/internal/ports"
)

const (
	EventRetention   = 7 * 24 * time.Hour
	EventCursorTTL   = 5 * time.Minute
	MaxEventCursors  = 16
	MaxEventsPerPage = 100
	// MaxEventPurgeBatch bounds work and lock duration for every janitor call.
	MaxEventPurgeBatch = 1000
)

var (
	ErrEventCursorExpired           = events.ErrCursorExpired
	ErrEventCursorBinding           = events.ErrCursorBinding
	ErrEventBudget            error = eventBudgetError{}
	ErrEventPrincipalRequired       = errors.New("storage: authenticated principal required for event reads")
)

type eventBudgetError struct{}

func (eventBudgetError) Error() string        { return "storage: event page exceeds response budget" }
func (eventBudgetError) BudgetExceeded() bool { return true }

// OpenEventPageForPrincipal atomically qualifies and opens a cursorless HTTP
// event page. Budget denial happens before pruning or eviction, so a client
// retry cannot lose an older usable cursor when the first response is too
// large for its requested budget.
func (s *Postgres) OpenEventPageForPrincipal(ctx context.Context, principal ports.Principal, maxBytes int) (ports.EventPage, error) {
	if maxBytes <= 0 {
		return ports.EventPage{}, ErrEventBudget
	}
	if s == nil || s.pool == nil || principal.WorkspaceID == "" || principal.Issuer == "" || principal.Subject == "" {
		return ports.EventPage{}, errors.New("storage: incomplete event cursor principal")
	}
	id, err := newEventCursorID()
	if err != nil {
		return ports.EventPage{}, err
	}
	cursor := ports.Cursor{ID: id, PrincipalHash: ports.PrincipalHash(principal), WorkspaceID: principal.WorkspaceID}
	page := ports.EventPage{Events: []ports.Event{}, Next: cursor}
	err = WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(principal), func(ctx context.Context, tx pgx.Tx) error {
		if err := lockCursorPrincipal(ctx, tx, cursor.PrincipalHash); err != nil {
			return err
		}
		body, err := marshalEventPage(page.Events, cursor.ID)
		if err != nil {
			return err
		}
		if len(body) > maxBytes {
			return ErrEventBudget
		}
		var dbNow time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow); err != nil {
			return err
		}
		if err := pruneEventCursors(ctx, tx, dbNow); err != nil {
			return err
		}
		var latest, floor int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(event_id),0) FROM event_outbox WHERE workspace_id=$1`, principal.WorkspaceID).Scan(&latest); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(through_event_id),0) FROM event_retention_floors WHERE workspace_id=$1`, principal.WorkspaceID).Scan(&floor); err != nil {
			return err
		}
		position := latest
		if floor > position {
			position = floor
		}
		if err := makeCursorRoom(ctx, tx, cursor.PrincipalHash); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO event_cursors
			(id, workspace_id, issuer, subject, principal_hash, policy_generation, last_event_id, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,clock_timestamp()+($8 * interval '1 second'))
			RETURNING extract(epoch from expires_at)::bigint`, cursor.ID, principal.WorkspaceID, principal.Issuer, principal.Subject, cursor.PrincipalHash, principal.PolicyGeneration, position, EventCursorTTL.Seconds()).Scan(&cursor.ExpiresAt)
	})
	if err != nil {
		return ports.EventPage{}, fmt.Errorf("open event page: %w", err)
	}
	page.Next = cursor
	return page, nil
}

// NewEventCursor creates a durable cursor bound to the complete caller
// identity. A new subscription begins after the latest event currently
// visible to its workspace; subsequent events are delivered by Read.
func (s *Postgres) NewEventCursor(ctx context.Context, principal ports.Principal, ttl time.Duration) (ports.Cursor, error) {
	if s == nil || s.pool == nil || principal.WorkspaceID == "" || principal.Issuer == "" || principal.Subject == "" {
		return ports.Cursor{}, errors.New("storage: incomplete event cursor principal")
	}
	if ttl <= 0 || ttl > EventCursorTTL {
		ttl = EventCursorTTL
	}
	id, err := newEventCursorID()
	if err != nil {
		return ports.Cursor{}, err
	}
	cursor := ports.Cursor{ID: id, PrincipalHash: ports.PrincipalHash(principal), WorkspaceID: principal.WorkspaceID}
	err = WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(principal), func(ctx context.Context, tx pgx.Tx) error {
		if err := lockCursorPrincipal(ctx, tx, cursor.PrincipalHash); err != nil {
			return err
		}
		var dbNow time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow); err != nil {
			return err
		}
		if err := pruneEventCursors(ctx, tx, dbNow); err != nil {
			return err
		}
		var latest int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(event_id), 0) FROM event_outbox WHERE workspace_id=$1`, principal.WorkspaceID).Scan(&latest); err != nil {
			return err
		}
		var floor int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(through_event_id), 0) FROM event_retention_floors WHERE workspace_id=$1`, principal.WorkspaceID).Scan(&floor); err != nil {
			return err
		}
		position := latest
		if floor > position {
			position = floor
		}
		if err := makeCursorRoom(ctx, tx, cursor.PrincipalHash); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO event_cursors
			(id, workspace_id, issuer, subject, principal_hash, policy_generation, last_event_id, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,clock_timestamp()+($8 * interval '1 second'))
			RETURNING extract(epoch from expires_at)::bigint`, cursor.ID, principal.WorkspaceID, principal.Issuer, principal.Subject, cursor.PrincipalHash, principal.PolicyGeneration, position, ttl.Seconds()).Scan(&cursor.ExpiresAt)
	})
	if err != nil {
		return ports.Cursor{}, fmt.Errorf("create event cursor: %w", err)
	}
	return cursor, nil
}

// Append persists one event. RevokeVersion uses the same insert helper inside
// its catalog mutation transaction so revocation state and visibility commit
// together.
func (s *Postgres) Append(ctx context.Context, event ports.Event) error {
	if s == nil || s.pool == nil {
		return errors.New("storage: unavailable postgres event store")
	}
	if err := validateEvent(event); err != nil {
		return err
	}
	err := WithTenant(ctx, s.pool, event.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return AppendEventInTransaction(ctx, tx, event)
	})
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

// AppendEventInTransaction writes an event using a caller-owned PostgreSQL
// transaction. Publication adapters use it inside the same tenant transaction
// that inserts the catalog row, preserving atomic publish-to-feed visibility.
// The caller must commit or roll back the transaction.
func AppendEventInTransaction(ctx context.Context, tx pgx.Tx, event ports.Event) error {
	if tx == nil {
		return errors.New("storage: nil event transaction")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateEvent(event); err != nil {
		return err
	}
	return insertEvent(ctx, tx, event)
}

// Read preserves ports.EventStore compatibility but fails closed because a
// cursor alone cannot prove the current policy generation.
func (s *Postgres) Read(ctx context.Context, cursor ports.Cursor) (ports.EventPage, error) {
	return ports.EventPage{}, ErrEventPrincipalRequired
}

// ReadEventPage preserves the legacy signature but fails closed without an
// authenticated principal. Use ReadEventPageForPrincipal for HTTP reads.
func (s *Postgres) ReadEventPage(ctx context.Context, cursor ports.Cursor, maxBytes int) (ports.EventPage, error) {
	return ports.EventPage{}, ErrEventPrincipalRequired
}

// ReadEventPageForPrincipal additionally binds the durable cursor to the
// currently authenticated principal, including its policy generation.
func (s *Postgres) ReadEventPageForPrincipal(ctx context.Context, principal ports.Principal, cursor ports.Cursor, maxBytes int) (ports.EventPage, error) {
	return s.readEventPage(ctx, principal, cursor, maxBytes)
}

func (s *Postgres) readEventPage(ctx context.Context, principal ports.Principal, cursor ports.Cursor, maxBytes int) (ports.EventPage, error) {
	if maxBytes <= 0 {
		return ports.EventPage{}, ErrEventBudget
	}
	if s == nil || s.pool == nil {
		return ports.EventPage{}, errors.New("storage: unavailable postgres event store")
	}
	if cursor.ID == "" || cursor.WorkspaceID == "" || cursor.PrincipalHash == "" {
		return ports.EventPage{}, ErrEventCursorBinding
	}
	page := ports.EventPage{Events: []ports.Event{}}
	withTenant := func(ctx context.Context, tx pgx.Tx) error {
		// Keep the lock order consistent with NewEventCursor so independent
		// reads and opens cannot deadlock while enforcing the principal cap.
		if err := lockCursorPrincipal(ctx, tx, cursor.PrincipalHash); err != nil {
			return err
		}
		var issuer, subject, storedHash string
		var generation uint64
		var position int64
		var expiresAt time.Time
		err := tx.QueryRow(ctx, `SELECT issuer, subject, principal_hash, policy_generation, last_event_id, expires_at
			FROM event_cursors WHERE id=$1 AND workspace_id=$2 FOR UPDATE`, cursor.ID, cursor.WorkspaceID).
			Scan(&issuer, &subject, &storedHash, &generation, &position, &expiresAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEventCursorBinding
		}
		if err != nil {
			return err
		}
		var dbNow time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow); err != nil {
			return err
		}
		if storedHash != cursor.PrincipalHash {
			return ErrEventCursorBinding
		}
		if principal.WorkspaceID != cursor.WorkspaceID || principal.Issuer != issuer || principal.Subject != subject || principal.PolicyGeneration != generation || ports.PrincipalHash(principal) != storedHash {
			return ErrEventCursorBinding
		}
		if !expiresAt.After(dbNow) {
			return ErrEventCursorExpired
		}
		var floor int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(through_event_id),0) FROM event_retention_floors WHERE workspace_id=$1`, cursor.WorkspaceID).Scan(&floor); err != nil {
			return err
		}
		if position < floor {
			page.RetentionGap = true
			return ErrEventCursorExpired
		}
		rows, err := tx.Query(ctx, `SELECT event_id, event_type, artifact_kind, artifact_id, artifact_version,
			extract(epoch from occurred_at)::bigint, policy_generation
			FROM event_outbox WHERE workspace_id=$1 AND event_id>$2 ORDER BY event_id LIMIT $3`, cursor.WorkspaceID, position, MaxEventsPerPage)
		if err != nil {
			return err
		}
		last := position
		for rows.Next() {
			var id, occurred int64
			var typ, kind, artifactID, version string
			var eventGeneration uint64
			if err := rows.Scan(&id, &typ, &kind, &artifactID, &version, &occurred, &eventGeneration); err != nil {
				rows.Close()
				return err
			}
			page.Events = append(page.Events, ports.Event{ID: fmt.Sprint(id), WorkspaceID: cursor.WorkspaceID, Type: ports.EventType(typ), Subject: ports.ArtifactRef{WorkspaceID: cursor.WorkspaceID, Kind: ports.ArtifactKind(kind), ID: artifactID, Version: version}, OccurredAt: occurred, PolicyGeneration: eventGeneration})
			last = id
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		next := cursor
		if len(page.Events) > 0 {
			next.ID, err = newEventCursorID()
			if err != nil {
				return err
			}
		}
		body, err := marshalEventPage(page.Events, next.ID)
		if err != nil {
			return err
		}
		if len(body) > maxBytes {
			return ErrEventBudget
		}

		// Sliding expiry and oldest-first pruning happen only after the payload
		// fits. A continuation is a new immutable position; the input remains
		// usable when a client never receives this page.
		if _, err := tx.Exec(ctx, `UPDATE event_cursors SET expires_at=clock_timestamp()+($2 * interval '1 second') WHERE id=$1`, cursor.ID, EventCursorTTL.Seconds()); err != nil {
			return err
		}
		if len(page.Events) == 0 {
			page.Next = next
			return nil
		}
		if err := pruneEventCursors(ctx, tx, dbNow); err != nil {
			return err
		}
		if err := makeCursorRoom(ctx, tx, cursor.PrincipalHash); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `INSERT INTO event_cursors
			(id, workspace_id, issuer, subject, principal_hash, policy_generation, last_event_id, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,clock_timestamp()+($8 * interval '1 second'))
			RETURNING extract(epoch from expires_at)::bigint`, next.ID, cursor.WorkspaceID, issuer, subject, storedHash, generation, last, EventCursorTTL.Seconds()).Scan(&next.ExpiresAt)
		if err != nil {
			return err
		}
		page.Next = next
		return nil
	}
	err := WithTenantPrincipal(ctx, s.pool, tenantFromPrincipal(principal), withTenant)
	if errors.Is(err, ErrEventCursorExpired) {
		return page, err
	}
	if errors.Is(err, ErrEventCursorBinding) || errors.Is(err, ErrEventBudget) {
		return ports.EventPage{}, err
	}
	if err != nil {
		return ports.EventPage{}, fmt.Errorf("read events: %w", err)
	}
	return page, nil
}

func marshalEventPage(events []ports.Event, nextCursor string) ([]byte, error) {
	// REST uses this same map with json.Marshal. Keep bytes identical so the
	// database budget check includes the canonical event envelope and fields.
	return json.Marshal(map[string]any{"events": events, "next_cursor": nextCursor})
}

func validateEvent(event ports.Event) error {
	if event.WorkspaceID == "" || event.Subject.WorkspaceID != event.WorkspaceID || event.Subject.Kind == "" || event.Subject.ID == "" || event.Subject.Version == "" {
		return errors.New("storage: incomplete event")
	}
	if event.Type != ports.EventVersionPublished && event.Type != ports.EventVersionRevoked {
		return errors.New("storage: invalid event type")
	}
	return nil
}

func insertEvent(ctx context.Context, tx pgx.Tx, event ports.Event) error {
	if event.OccurredAt == 0 {
		return tx.QueryRow(ctx, `INSERT INTO event_outbox (workspace_id,event_type,artifact_kind,artifact_id,artifact_version,policy_generation,occurred_at)
			VALUES ($1,$2,$3,$4,$5,$6,clock_timestamp()) RETURNING event_id`, event.WorkspaceID, event.Type, event.Subject.Kind, event.Subject.ID, event.Subject.Version, event.PolicyGeneration).Scan(new(int64))
	}
	return tx.QueryRow(ctx, `INSERT INTO event_outbox (workspace_id,event_type,artifact_kind,artifact_id,artifact_version,policy_generation,occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,to_timestamp($7)) RETURNING event_id`, event.WorkspaceID, event.Type, event.Subject.Kind, event.Subject.ID, event.Subject.Version, event.PolicyGeneration, event.OccurredAt).Scan(new(int64))
}

func tenantFromPrincipal(p ports.Principal) Tenant {
	return Tenant{Issuer: p.Issuer, Subject: p.Subject, Audience: p.Audience, WorkspaceID: p.WorkspaceID, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration}
}

func lockCursorPrincipal(ctx context.Context, tx pgx.Tx, hash string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, hash)
	return err
}

func pruneEventCursors(ctx context.Context, tx pgx.Tx, dbNow time.Time) error {
	_, err := tx.Exec(ctx, `DELETE FROM event_cursors WHERE expires_at <= $1`, dbNow)
	return err
}

// PurgeEvents removes at most limit expired outbox rows from one tenant and
// advances that tenant's retention floor in the same transaction. An external
// janitor should call this periodically for each workspace it is authorized to
// maintain; this storage API does not start or claim to install a scheduler.
func (s *Postgres) PurgeEvents(ctx context.Context, tenant Tenant, limit int) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, errors.New("storage: unavailable postgres event store")
	}
	if tenant.WorkspaceID == "" || limit <= 0 || limit > MaxEventPurgeBatch {
		return 0, errors.New("storage: invalid event purge scope or batch limit")
	}
	var removed int64
	err := WithTenantPrincipal(ctx, s.pool, tenant, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `WITH eligible AS (
			SELECT event_id FROM event_outbox
			WHERE workspace_id=$1 AND occurred_at < clock_timestamp() - interval '7 days'
			ORDER BY event_id LIMIT $2 FOR UPDATE SKIP LOCKED
		), purged AS (
			DELETE FROM event_outbox e USING eligible x
			WHERE e.event_id=x.event_id AND e.workspace_id=$1
			RETURNING e.workspace_id, e.event_id
		), floors AS (
			SELECT workspace_id, max(event_id) AS through_event_id FROM purged GROUP BY workspace_id
		), saved AS (
		INSERT INTO event_retention_floors(workspace_id, through_event_id)
		SELECT workspace_id, through_event_id FROM floors
		ON CONFLICT (workspace_id) DO UPDATE SET
			through_event_id=GREATEST(event_retention_floors.through_event_id, EXCLUDED.through_event_id),
			updated_at=clock_timestamp()
		RETURNING workspace_id
		)
		SELECT count(*) FROM purged CROSS JOIN (SELECT count(*) FROM saved) AS applied`, tenant.WorkspaceID, limit).Scan(&removed)
	})
	if err != nil {
		return 0, fmt.Errorf("purge tenant events: %w", err)
	}
	return removed, nil
}

func makeCursorRoom(ctx context.Context, tx pgx.Tx, hash string) error {
	_, err := tx.Exec(ctx, `DELETE FROM event_cursors WHERE id IN (
		SELECT id FROM event_cursors WHERE principal_hash=$1 ORDER BY cursor_order DESC
		OFFSET $2
	)`, hash, MaxEventCursors-1)
	return err
}

func newEventCursorID() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate event cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

var _ ports.EventStore = (*Postgres)(nil)
