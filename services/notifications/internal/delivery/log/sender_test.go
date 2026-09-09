package log

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"notifications/internal/delivery"
)

func TestSenderRequiresIdempotencyKey(t *testing.T) {
	sender := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := sender.Send(context.Background(), delivery.Message{
		NotificationID: "notification-1",
	})
	if err == nil {
		t.Fatal("Send() error = nil, want missing idempotency key error")
	}
}

func TestSenderUsesIdempotencyKeyAsProviderMessageID(t *testing.T) {
	sender := New(slog.New(slog.NewTextHandler(io.Discard, nil)))

	providerMessageID, err := sender.Send(context.Background(), delivery.Message{
		NotificationID: "notification-1",
		IdempotencyKey: "stable-delivery-key",
	})
	if err != nil {
		t.Fatalf("Send(): %v", err)
	}
	if providerMessageID != "log:stable-delivery-key" {
		t.Fatalf(
			"provider message ID = %q, want %q",
			providerMessageID,
			"log:stable-delivery-key",
		)
	}
}
