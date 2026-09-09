package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	ClaimBatch(ctx context.Context, workerID string, limit int, lease time.Duration) ([]Event, error)
	MarkPublished(ctx context.Context, eventID, workerID string, at time.Time) error
	Reschedule(ctx context.Context, eventID, workerID string, availableAt time.Time, reason string) error
}

type PostgresRepository struct {
	db *gorm.DB
}

type Writer struct{}

func NewRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (Writer) Add(tx *gorm.DB, event *Event) error {
	if err := tx.Create(event).Error; err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ClaimBatch(
	ctx context.Context,
	workerID string,
	limit int,
	lease time.Duration,
) ([]Event, error) {
	const query = `
WITH candidates AS (
  SELECT id
  FROM outbox_events
  WHERE published_at IS NULL
	AND available_at <= now()
	AND (locked_until IS NULL OR locked_until < now())
  ORDER BY occurred_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT @batch_size
)
UPDATE outbox_events AS event
SET locked_by = @worker_id,
  locked_until = now() + make_interval(secs => @lease_seconds),
  attempts = attempts + 1
FROM candidates
WHERE event.id = candidates.id
RETURNING event.*;
`

	var events []Event

	result := r.db.WithContext(ctx).Raw(
		query,
		sql.Named("batch_size", limit),
		sql.Named("worker_id", workerID),
		sql.Named("lease_seconds", lease.Seconds()),
	).Scan(&events)

	if result.Error != nil {
		return nil, fmt.Errorf("claim outbox batch: %w", result.Error)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].ID < events[j].ID
		}
		return events[i].OccurredAt.Before(events[j].OccurredAt)
	})

	return events, nil
}

func (r *PostgresRepository) MarkPublished(
	ctx context.Context,
	eventID string,
	workerID string,
	at time.Time,
) error {
	const query = `
UPDATE outbox_events
SET published_at = @published_at,
  locked_until = NULL,
  locked_by = NULL,
  last_error = NULL
WHERE id = @event_id
AND locked_by = @worker_id
AND published_at IS NULL
`

	result := r.db.WithContext(ctx).Exec(
		query,
		sql.Named("published_at", at),
		sql.Named("event_id", eventID),
		sql.Named("worker_id", workerID),
	)
	if result.Error != nil {
		return fmt.Errorf("mark outbox event published: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf(
			"mark outbox event %s published: affected %d rows",
			eventID,
			result.RowsAffected,
		)
	}

	return nil
}

func (r *PostgresRepository) Reschedule(
	ctx context.Context,
	eventID string,
	workerID string,
	availableAt time.Time,
	reason string,
) error {
	const query = `
UPDATE outbox_events
SET available_at = @available_at,
  locked_until = NULL,
  locked_by = NULL,
  last_error = @reason
WHERE id = @event_id
AND locked_by = @worker_id
AND published_at IS NULL
`

	result := r.db.WithContext(ctx).Exec(
		query,
		sql.Named("available_at", availableAt),
		sql.Named("reason", reason),
		sql.Named("event_id", eventID),
		sql.Named("worker_id", workerID),
	)
	if result.Error != nil {
		return fmt.Errorf("reschedule outbox event: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf(
			"reschedule outbox event %s: affected %d rows",
			eventID,
			result.RowsAffected,
		)
	}

	return nil
}
