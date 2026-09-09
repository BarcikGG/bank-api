package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"
)

type Relay struct {
	repository   Repository
	publisher    Publisher
	logger       *slog.Logger
	workerID     string
	batchSize    int
	pollInterval time.Duration
	lease        time.Duration
}

func NewRelay(
	repository Repository,
	publisher Publisher,
	logger *slog.Logger,
	workerID string,
	batchSize int,
	pollInterval time.Duration,
	lease time.Duration,
) (*Relay, error) {
	if repository == nil {
		return nil, errors.New("outbox repository is nil")
	}
	if publisher == nil {
		return nil, errors.New("outbox publisher is nil")
	}
	if workerID == "" {
		return nil, errors.New("worker ID is empty")
	}
	if batchSize <= 0 {
		return nil, errors.New("batch size must be positive")
	}
	if pollInterval <= 0 {
		return nil, errors.New("poll interval must be positive")
	}
	if lease <= 0 {
		return nil, errors.New("lease must be positive")
	}

	return &Relay{
		repository:   repository,
		publisher:    publisher,
		logger:       logger.With(slog.String("component", "outbox_relay")),
		workerID:     workerID,
		batchSize:    batchSize,
		pollInterval: pollInterval,
		lease:        lease,
	}, nil
}

func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		if err := r.publishBatch(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			r.logger.Error("publish outbox batch", slog.Any("error", err))
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Relay) publishBatch(ctx context.Context) error {
	events, err := r.repository.ClaimBatch(
		ctx,
		r.workerID,
		r.batchSize,
		r.lease,
	)
	if err != nil {
		return fmt.Errorf("claim batch: %w", err)
	}

	var batchErr error

	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return errors.Join(batchErr, err)
		}

		if err := r.publisher.Publish(ctx, event); err != nil {
			if ctx.Err() != nil {
				return errors.Join(batchErr, ctx.Err())
			}

			publishErr := fmt.Errorf("publish event %s: %w", event.ID, err)
			availableAt := time.Now().UTC().Add(retryDelay(event.Attempts))

			if rescheduleErr := r.repository.Reschedule(
				ctx,
				event.ID,
				r.workerID,
				availableAt,
				publishErr.Error(),
			); rescheduleErr != nil {
				return errors.Join(
					batchErr,
					publishErr,
					fmt.Errorf("reschedule event %s: %w", event.ID, rescheduleErr),
				)
			}

			batchErr = errors.Join(batchErr, publishErr)
			continue
		}

		if err := r.repository.MarkPublished(
			ctx,
			event.ID,
			r.workerID,
			time.Now().UTC(),
		); err != nil {
			return errors.Join(
				batchErr,
				fmt.Errorf("mark event %s published: %w", event.ID, err),
			)
		}
	}

	return batchErr
}

func retryDelay(attempt int) time.Duration {
	const maxDelay = 5 * time.Minute

	if attempt < 1 {
		attempt = 1
	}

	shift := min(attempt-1, 8)
	delay := time.Second * time.Duration(1<<shift)

	if delay > maxDelay {
		return maxDelay
	}

	jitterLimit := delay / 5
	jitter := time.Duration(rand.Int64N(int64(jitterLimit) + 1))

	return min(delay+jitter, maxDelay)
}
