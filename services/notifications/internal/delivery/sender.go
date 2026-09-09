package delivery

import (
	"context"
	"encoding/json"
)

type Message struct {
	NotificationID string
	IdempotencyKey string
	Recipient      string
	Template       string
	Payload        json.RawMessage
}

type Sender interface {
	Channel() string
	Send(ctx context.Context, message Message) (providerMessageID string, err error)
}
