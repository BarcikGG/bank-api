package events

import (
	"context"
	"errors"
	"fmt"
	"notifications/internal/notifications"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

type Processor struct {
	db       *gorm.DB
	handlers map[string]EventHandler
}

func NewProcessor(db *gorm.DB, handlers ...EventHandler) (*Processor, error) {
	if db == nil {
		return nil, errors.New("processor database is nil")
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

	return &Processor{db: db, handlers: registry}, nil
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

	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inbox := notifications.ProcessedEvent{
			EventID:        message.Event.ID,
			EventType:      message.Event.Type,
			EventVersion:   message.Event.Version,
			KafkaTopic:     message.Topic,
			KafkaPartition: message.Partition,
			KafkaOffset:    message.Offset,
			ProcessedAt:    time.Now().UTC(),
		}

		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}},
			DoNothing: true,
		}).Create(&inbox)
		if result.Error != nil {
			return fmt.Errorf("insert processed event: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}

		if len(jobs) == 0 {
			return nil
		}

		if err := tx.Create(&jobs).Error; err != nil {
			return fmt.Errorf("insert notification jobs: %w", err)
		}

		return nil
	})
}
