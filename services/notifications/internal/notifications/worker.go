package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"notifications/internal/delivery"
	"time"
)

type Worker struct {
	repository      Repository
	senders         map[string]delivery.Sender
	logger          *slog.Logger
	workerID        string
	batchSize       int
	pollInterval    time.Duration
	lease           time.Duration
	maxAttempts     int
	deliveryTimeout time.Duration
}

func NewWorker(
	repository Repository,
	senders []delivery.Sender,
	logger *slog.Logger,
	workerID string,
	batchSize int,
	pollInterval time.Duration,
	lease time.Duration,
	maxAttempts int,
	deliveryTimeout time.Duration,
) (*Worker, error) {
	if repository == nil {
		return nil, errors.New("notification repository is nil")
	}
	if logger == nil {
		return nil, errors.New("worker logger is nil")
	}
	if workerID == "" {
		return nil, errors.New("worker ID is empty")
	}
	if batchSize <= 0 || pollInterval <= 0 || lease <= 0 {
		return nil, errors.New("invalid worker batch or lease configuration")
	}
	if maxAttempts <= 0 || deliveryTimeout <= 0 {
		return nil, errors.New("invalid worker retry configuration")
	}

	registry := make(map[string]delivery.Sender, len(senders))
	for _, sender := range senders {
		channel := sender.Channel()
		if channel == "" {
			return nil, errors.New("sender channel is empty")
		}
		if _, exists := registry[channel]; exists {
			return nil, fmt.Errorf("duplicate sender for channel %s", channel)
		}
		registry[channel] = sender
	}

	return &Worker{
		repository:      repository,
		senders:         registry,
		logger:          logger.With(slog.String("component", "delivery_worker")),
		workerID:        workerID,
		batchSize:       batchSize,
		pollInterval:    pollInterval,
		lease:           lease,
		maxAttempts:     maxAttempts,
		deliveryTimeout: deliveryTimeout,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		if err := w.processBatch(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("process notification batch", slog.Any("error", err))
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) error {
	jobs, err := w.repository.ClaimBatch(
		ctx,
		w.workerID,
		w.batchSize,
		w.lease,
	)
	if err != nil {
		return fmt.Errorf("claim notification batch: %w", err)
	}

	var batchErr error
	for _, job := range jobs {
		if err := w.processNotification(ctx, job); err != nil {
			if ctx.Err() != nil {
				return errors.Join(batchErr, ctx.Err())
			}
			batchErr = errors.Join(batchErr, err)
		}
	}

	return batchErr
}

func (w *Worker) processNotification(ctx context.Context, job Notification) error {
	startedAt := time.Now().UTC()
	sender, exists := w.senders[job.Channel]
	if !exists {
		reason := "unsupported notification channel: " + job.Channel
		finishedAt := time.Now().UTC()

		if err := w.repository.MarkFailed(
			ctx,
			job,
			w.workerID,
			StatusDead,
			finishedAt,
			"",
			reason,
			startedAt,
			finishedAt,
		); err != nil {
			return err
		}

		w.logger.Error(
			"notification moved to dead",
			slog.String("notification_id", job.ID),
			slog.String("reason", reason),
		)
		return nil
	}

	message := delivery.Message{
		NotificationID: job.ID,
		IdempotencyKey: job.ID,
		Recipient:      job.Recipient,
		Template:       job.Template,
		Payload:        json.RawMessage(job.Payload),
	}

	sendCtx, cancel := context.WithTimeout(ctx, w.deliveryTimeout)
	providerMessageID, sendErr := sender.Send(sendCtx, message)
	cancel()
	finishedAt := time.Now().UTC()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if sendErr == nil {
		if err := w.repository.MarkSent(
			ctx,
			job,
			w.workerID,
			providerMessageID,
			startedAt,
			finishedAt,
		); err != nil {
			return err
		}

		w.logger.Info(
			"notification completed",
			slog.String("notification_id", job.ID),
			slog.Int("attempt", job.Attempts),
		)
		return nil
	}

	outcome := StatusRetry
	availableAt := finishedAt.Add(notificationRetryDelay(job.Attempts))
	if job.Attempts >= w.maxAttempts {
		outcome = StatusDead
		availableAt = finishedAt
	}

	if err := w.repository.MarkFailed(
		ctx,
		job,
		w.workerID,
		outcome,
		availableAt,
		providerMessageID,
		sendErr.Error(),
		startedAt,
		finishedAt,
	); err != nil {
		return err
	}

	w.logger.Warn(
		"notification delivery failed",
		slog.String("notification_id", job.ID),
		slog.String("outcome", outcome),
		slog.Int("attempt", job.Attempts),
		slog.Any("error", sendErr),
	)

	return nil
}

func notificationRetryDelay(attempt int) time.Duration {
	const maxDelay = 30 * time.Minute

	if attempt < 1 {
		attempt = 1
	}

	shift := min(attempt-1, 10)
	delay := time.Second * time.Duration(1<<shift)
	if delay >= maxDelay {
		return maxDelay
	}

	jitterLimit := delay / 5
	jitter := time.Duration(rand.Int64N(int64(jitterLimit) + 1))
	return min(delay+jitter, maxDelay)
}
