package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type KafkaPublisher struct {
	client *kgo.Client
}

func NewKafkaPublisher(client *kgo.Client) *KafkaPublisher {
	return &KafkaPublisher{client: client}
}

func (p *KafkaPublisher) Publish(ctx context.Context, event Event) error {
	envelope := Envelope{
		ID:            event.ID,
		Type:          event.EventType,
		Version:       event.EventVersion,
		Producer:      "bank-api",
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		OccurredAt:    event.OccurredAt,
		CorrelationID: event.CorrelationID,
		Data:          json.RawMessage(event.Payload),
	}

	value, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}

	record := &kgo.Record{
		Topic: event.Topic,
		Key:   []byte(event.PartitionKey),
		Value: value,
		Headers: []kgo.RecordHeader{
			{Key: "event_id", Value: []byte(event.ID)},
			{Key: "event_type", Value: []byte(event.EventType)},
		},
	}

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce event %s: %w", event.ID, err)
	}

	return nil
}

func (p *KafkaPublisher) Close() {
	p.client.Close()
}
