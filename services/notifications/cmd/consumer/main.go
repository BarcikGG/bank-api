package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"notifications/internal/config"
	appevents "notifications/internal/events"
	applogger "notifications/internal/logger"
	kafkatransport "notifications/internal/messaging/kafka"
	"notifications/internal/storage/postgres"
)

func main() {
	cfg := config.MustLoadConsumer()
	logger := applogger.New(cfg.Env, "consumer")

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("notifications consumer stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	storage, err := postgres.Connect(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	defer storage.Close()

	eventRepository := appevents.NewRepository(storage.DB)
	processor, err := appevents.NewProcessor(
		eventRepository,
		appevents.UserRegisteredHandler{},
	)
	if err != nil {
		return fmt.Errorf("create event processor: %w", err)
	}

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	consumer, err := kafkatransport.NewConsumer(
		brokers,
		cfg.KafkaTopic,
		cfg.KafkaConsumerGroup,
		processor,
		logger,
	)
	if err != nil {
		return err
	}
	defer consumer.Close()

	logger.Info(
		"notifications consumer started",
		slog.String("topic", cfg.KafkaTopic),
		slog.String("group", cfg.KafkaConsumerGroup),
	)

	return consumer.Run(ctx)
}
