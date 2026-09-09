package outbox

import (
	"time"

	"gorm.io/datatypes"
)

type Event struct {
	ID string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	Topic         string `gorm:"not null"`
	PartitionKey  string `gorm:"not null"`
	EventType     string `gorm:"not null"`
	EventVersion  int    `gorm:"not null;check:event_version_positive,event_version > 0"`
	AggregateType string `gorm:"not null;index:outbox_events_aggregate_idx,priority:1"`
	AggregateID   string `gorm:"not null;index:outbox_events_aggregate_idx,priority:2"`
	CorrelationID string
	Payload       datatypes.JSON `gorm:"type:jsonb;not null"`

	OccurredAt  time.Time `gorm:"not null;index:outbox_events_ready_idx,priority:2,where:published_at IS NULL;index:outbox_events_aggregate_idx,priority:3"`
	AvailableAt time.Time `gorm:"not null;index:outbox_events_ready_idx,priority:1,where:published_at IS NULL"`
	PublishedAt *time.Time
	LockedUntil *time.Time
	LockedBy    *string
	Attempts    int `gorm:"not null;default:0;check:attempts_nonnegative,attempts >= 0"`
	LastError   string
}

func (Event) TableName() string {
	return "outbox_events"
}
