package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"notifications/internal/config"
	"notifications/internal/delivery"
	logdelivery "notifications/internal/delivery/log"
	applogger "notifications/internal/logger"
	domain "notifications/internal/notifications"
	"notifications/internal/storage/postgres"

	"github.com/google/uuid"
)

func main() {
	cfg := config.MustLoadWorker()
	logger := applogger.New(cfg.Env, "worker")

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("notifications worker stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	storage, err := postgres.Connect(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	defer storage.Close()

	repository := domain.NewRepository(storage.DB)
	sender := logdelivery.New(logger)
	workerID := uuid.NewString()

	worker, err := domain.NewWorker(
		repository,
		[]delivery.Sender{sender},
		logger,
		workerID,
		cfg.NotificationBatchSize,
		cfg.NotificationPollInterval,
		cfg.NotificationLease,
		cfg.NotificationMaxAttempts,
		cfg.DeliveryTimeout,
	)
	if err != nil {
		return fmt.Errorf("create notification worker: %w", err)
	}

	logger.Info(
		"notifications worker started",
		slog.String("worker_id", workerID),
		slog.Int("batch_size", cfg.NotificationBatchSize),
	)

	return worker.Run(ctx)
}
