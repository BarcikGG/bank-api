package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	appevents "notifications/internal/events"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Processor interface {
	Process(ctx context.Context, message appevents.Message) error
}

type Consumer struct {
	client    *kgo.Client
	processor Processor
	logger    *slog.Logger
}

func NewConsumer(
	brokers []string,
	topic string,
	group string,
	processor Processor,
	logger *slog.Logger,
) (*Consumer, error) {
	if processor == nil {
		return nil, errors.New("consumer processor is nil")
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(group),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}

	return &Consumer{
		client:    client,
		processor: processor,
		logger:    logger.With(slog.String("component", "kafka_consumer")),
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}

		var fetchErr error
		for _, current := range fetches.Errors() {
			fetchErr = errors.Join(
				fetchErr,
				fmt.Errorf(
					"fetch %s partition %d: %w",
					current.Topic,
					current.Partition,
					current.Err,
				),
			)
		}
		if fetchErr != nil {
			return fetchErr
		}

		iterator := fetches.RecordIter()
		for !iterator.Done() {
			record := iterator.Next()

			event, err := appevents.Decode(record.Value)
			if err != nil {
				return fmt.Errorf(
					"decode record %s/%d/%d: %w",
					record.Topic,
					record.Partition,
					record.Offset,
					err,
				)
			}

			message := appevents.Message{
				Event:     event,
				Topic:     record.Topic,
				Partition: record.Partition,
				Offset:    record.Offset,
			}

			if err := c.processor.Process(ctx, message); err != nil {
				return fmt.Errorf(
					"process record %s/%d/%d: %w",
					record.Topic,
					record.Partition,
					record.Offset,
					err,
				)
			}

			if err := c.client.CommitRecords(ctx, record); err != nil {
				return fmt.Errorf(
					"commit record %s/%d/%d: %w",
					record.Topic,
					record.Partition,
					record.Offset,
					err,
				)
			}

			c.logger.InfoContext(
				ctx,
				"Kafka event processed",
				slog.String("event_id", event.ID),
				slog.String("event_type", event.Type),
				slog.String("kafka_topic", record.Topic),
				slog.Int("kafka_partition", int(record.Partition)),
				slog.Int64("kafka_offset", record.Offset),
			)
		}
	}
}

func (c *Consumer) Close() {
	c.client.Close()
}
