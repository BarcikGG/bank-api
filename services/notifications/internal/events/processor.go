package events

import (
	"context"
	"errors"
	"fmt"
	"notifications/internal/notifications"
	"time"
)

type Message struct {
	Event     Envelope
	Topic     string
	Partition int32
	Offset    int64
}

type EventHandler interface {
	EventType() string
	Build(event Envelope) ([]notifications.Notification, error)
}

type Repository interface {
	Save(ctx context.Context, event notifications.ProcessedEvent, jobs []notifications.Notification) error
}

type Processor struct {
	repository Repository
	handlers   map[string]EventHandler
}

func NewProcessor(repository Repository, handlers ...EventHandler) (*Processor, error) {
	if repository == nil {
		return nil, errors.New("event repository is nil")
	}

	registry := make(map[string]EventHandler, len(handlers))
	for _, handler := range handlers {
		eventType := handler.EventType()
		if eventType == "" {
			return nil, errors.New("handler event type is empty")
		}
		if _, exists := registry[eventType]; exists {
			return nil, fmt.Errorf("duplicate handler for event type %s", eventType)
		}
		registry[eventType] = handler
	}

	return &Processor{repository: repository, handlers: registry}, nil
}

func (p *Processor) Process(ctx context.Context, message Message) error {
	handler, ok := p.handlers[message.Event.Type]
	if !ok {
		return fmt.Errorf("unsupported event type: %s", message.Event.Type)
	}

	jobs, err := handler.Build(message.Event)
	if err != nil {
		return fmt.Errorf("build notification jobs: %w", err)
	}

	inbox := notifications.ProcessedEvent{
		EventID:        message.Event.ID,
		EventType:      message.Event.Type,
		EventVersion:   message.Event.Version,
		KafkaTopic:     message.Topic,
		KafkaPartition: message.Partition,
		KafkaOffset:    message.Offset,
		ProcessedAt:    time.Now().UTC(),
	}

	return p.repository.Save(ctx, inbox, jobs)
}
