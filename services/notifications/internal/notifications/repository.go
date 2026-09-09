package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	ClaimBatch(
		ctx context.Context,
		workerID string,
		limit int,
		lease time.Duration,
	) ([]Notification, error)

	MarkSent(
		ctx context.Context,
		notification Notification,
		workerID string,
		providerMessageID string,
		startedAt time.Time,
		finishedAt time.Time,
	) error

	MarkFailed(
		ctx context.Context,
		notification Notification,
		workerID string,
		outcome string,
		availableAt time.Time,
		providerMessageID string,
		reason string,
		startedAt time.Time,
		finishedAt time.Time,
	) error
}

type PostgresRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) ClaimBatch(
	ctx context.Context,
	workerID string,
	limit int,
	lease time.Duration,
) ([]Notification, error) {
	const query = `
WITH candidates AS (
    SELECT id
    FROM notifications
    WHERE (
        (
            status IN ('pending', 'retry')
            AND available_at <= now()
            AND (locked_until IS NULL OR locked_until < now())
        )
        OR (
            status = 'sending'
            AND locked_until < now()
        )
    )
    ORDER BY available_at, created_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT @batch_size
)
UPDATE notifications AS notification
SET status = 'sending',
    locked_by = @worker_id,
    locked_until = now() + make_interval(secs => @lease_seconds),
    attempts = attempts + 1,
    updated_at = now()
FROM candidates
WHERE notification.id = candidates.id
RETURNING notification.*;
`

	var jobs []Notification
	result := r.db.WithContext(ctx).Raw(
		query,
		sql.Named("batch_size", limit),
		sql.Named("worker_id", workerID),
		sql.Named("lease_seconds", lease.Seconds()),
	).Scan(&jobs)
	if result.Error != nil {
		return nil, fmt.Errorf("claim notifications: %w", result.Error)
	}

	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].AvailableAt.Equal(jobs[j].AvailableAt) {
			if jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
				return jobs[i].ID < jobs[j].ID
			}
			return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
		}
		return jobs[i].AvailableAt.Before(jobs[j].AvailableAt)
	})

	return jobs, nil
}

func (r *PostgresRepository) MarkSent(
	ctx context.Context,
	notification Notification,
	workerID string,
	providerMessageID string,
	startedAt time.Time,
	finishedAt time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		const updateQuery = `
UPDATE notifications
SET status = 'sent',
    provider_message_id = @provider_message_id,
    sent_at = @finished_at,
    last_error = NULL,
    locked_by = NULL,
    locked_until = NULL,
    updated_at = @finished_at
WHERE id = @notification_id
  AND status = 'sending'
  AND locked_by = @worker_id
`

		result := tx.Exec(
			updateQuery,
			sql.Named("provider_message_id", providerMessageID),
			sql.Named("finished_at", finishedAt),
			sql.Named("notification_id", notification.ID),
			sql.Named("worker_id", workerID),
		)
		if result.Error != nil {
			return fmt.Errorf("mark notification sent: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf(
				"mark notification %s sent: affected %d rows",
				notification.ID,
				result.RowsAffected,
			)
		}

		attempt := NotificationAttempt{
			NotificationID:    notification.ID,
			AttemptNumber:     notification.Attempts,
			WorkerID:          workerID,
			StartedAt:         startedAt,
			FinishedAt:        finishedAt,
			Outcome:           StatusSent,
			ProviderMessageID: stringPointer(providerMessageID),
		}
		if err := tx.Create(&attempt).Error; err != nil {
			return fmt.Errorf("insert sent notification attempt: %w", err)
		}

		return nil
	})
}

func (r *PostgresRepository) MarkFailed(
	ctx context.Context,
	notification Notification,
	workerID string,
	outcome string,
	availableAt time.Time,
	providerMessageID string,
	reason string,
	startedAt time.Time,
	finishedAt time.Time,
) error {
	if outcome != StatusRetry && outcome != StatusDead {
		return fmt.Errorf("invalid failure outcome: %s", outcome)
	}
	if reason == "" {
		return errors.New("notification failure reason is empty")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		const updateQuery = `
UPDATE notifications
SET status = @outcome,
    available_at = @available_at,
    provider_message_id = @provider_message_id,
    last_error = @reason,
    locked_by = NULL,
    locked_until = NULL,
    updated_at = @finished_at
WHERE id = @notification_id
  AND status = 'sending'
  AND locked_by = @worker_id
`

		result := tx.Exec(
			updateQuery,
			sql.Named("outcome", outcome),
			sql.Named("available_at", availableAt),
			sql.Named("provider_message_id", nullableString(providerMessageID)),
			sql.Named("reason", reason),
			sql.Named("finished_at", finishedAt),
			sql.Named("notification_id", notification.ID),
			sql.Named("worker_id", workerID),
		)
		if result.Error != nil {
			return fmt.Errorf("mark notification failed: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf(
				"mark notification %s failed: affected %d rows",
				notification.ID,
				result.RowsAffected,
			)
		}

		attempt := NotificationAttempt{
			NotificationID:    notification.ID,
			AttemptNumber:     notification.Attempts,
			WorkerID:          workerID,
			StartedAt:         startedAt,
			FinishedAt:        finishedAt,
			Outcome:           outcome,
			ProviderMessageID: stringPointer(providerMessageID),
			Error:             stringPointer(reason),
		}
		if err := tx.Create(&attempt).Error; err != nil {
			return fmt.Errorf("insert failed notification attempt: %w", err)
		}

		return nil
	})
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
