package log

import (
	"context"
	"errors"
	"log/slog"

	"notifications/internal/delivery"
)

type Sender struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Sender {
	return &Sender{
		logger: logger.With(slog.String("component", "log_sender")),
	}
}

func (*Sender) Channel() string {
	return "email"
}

func (s *Sender) Send(ctx context.Context, message delivery.Message) (string, error) {
	if message.IdempotencyKey == "" {
		return "", errors.New("delivery idempotency key is empty")
	}

	s.logger.InfoContext(
		ctx,
		"notification sent",
		slog.String("notification_id", message.NotificationID),
		slog.String("idempotency_key", message.IdempotencyKey),
		slog.String("recipient", message.Recipient),
		slog.String("template", message.Template),
	)

	return "log:" + message.IdempotencyKey, nil
}
