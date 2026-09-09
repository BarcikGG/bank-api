package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type UserRegisteredV1 struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type Envelope struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	Producer      string          `json:"producer"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Data          json.RawMessage `json:"data"`
}

func Decode(value []byte) (Envelope, error) {
	var event Envelope
	if err := json.Unmarshal(value, &event); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}

	if event.ID == "" {
		return Envelope{}, errors.New("event ID is empty")
	}
	if event.Type == "" {
		return Envelope{}, errors.New("event type is empty")
	}
	if event.Version <= 0 {
		return Envelope{}, errors.New("event version must be positive")
	}
	if event.AggregateID == "" {
		return Envelope{}, errors.New("aggregate ID is empty")
	}
	if event.OccurredAt.IsZero() {
		return Envelope{}, errors.New("occurred_at is empty")
	}
	if len(event.Data) == 0 || string(event.Data) == "null" {
		return Envelope{}, errors.New("event data is empty")
	}

	return event, nil
}
