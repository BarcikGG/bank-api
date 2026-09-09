package main

import (
	"bank/internal/config"
	"bank/internal/outbox"
	"bank/internal/storage/postgres"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	applogger "bank/internal/logger"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	cfg := config.MustLoadRelay()

	logger := applogger.New(cfg.Env).With(slog.String("process", "outbox-relay"))

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("outbox relay stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	storage, err := postgres.Connect(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}

	sqlDB, err := storage.DB.DB()
	if err != nil {
		return fmt.Errorf("get underlying database connection: %w", err)
	}
	defer sqlDB.Close()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.ZstdCompression()),
		kgo.RecordDeliveryTimeout(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create Kafka producer: %w", err)
	}

	publisher := outbox.NewKafkaPublisher(client)
	defer publisher.Close()

	repository := outbox.NewRepository(storage.DB)
	workerID := uuid.NewString()

	relay, err := outbox.NewRelay(
		repository,
		publisher,
		logger,
		workerID,
		cfg.OutboxBatchSize,
		cfg.OutboxPollInterval,
		cfg.OutboxLease,
	)
	if err != nil {
		return fmt.Errorf("create outbox relay: %w", err)
	}

	logger.Info(
		"outbox relay started",
		slog.String("worker_id", workerID),
		slog.Int("batch_size", cfg.OutboxBatchSize),
	)

	return relay.Run(ctx)
}
