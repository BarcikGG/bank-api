package notifications

import (
	"time"

	"gorm.io/datatypes"
)

const (
	StatusPending = "pending"
	StatusSending = "sending"
	StatusRetry   = "retry"
	StatusSent    = "sent"
	StatusDead    = "dead"
)

type ProcessedEvent struct {
	EventID      string `gorm:"type:uuid;primaryKey"`
	EventType    string `gorm:"not null"`
	EventVersion int    `gorm:"not null;check:processed_event_version_positive,event_version > 0"`

	KafkaTopic     string `gorm:"not null;uniqueIndex:processed_events_kafka_position_uidx,priority:1"`
	KafkaPartition int32  `gorm:"not null;uniqueIndex:processed_events_kafka_position_uidx,priority:2"`
	KafkaOffset    int64  `gorm:"not null;uniqueIndex:processed_events_kafka_position_uidx,priority:3"`

	ProcessedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (ProcessedEvent) TableName() string {
	return "processed_events"
}

type Notification struct {
	ID string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	EventID string `gorm:"type:uuid;not null;uniqueIndex:notifications_delivery_uidx,priority:1"`
	UserID  string `gorm:"type:uuid;not null"`

	Recipient string         `gorm:"not null;uniqueIndex:notifications_delivery_uidx,priority:2"`
	Channel   string         `gorm:"not null;uniqueIndex:notifications_delivery_uidx,priority:3"`
	Template  string         `gorm:"not null"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null"`

	Status   string `gorm:"not null;default:'pending';check:notifications_status_valid,status IN ('pending','sending','retry','sent','dead')"`
	Attempts int    `gorm:"not null;default:0;check:notification_attempts_nonnegative,attempts >= 0"`

	AvailableAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;index:notifications_ready_idx,priority:1,where:status IN ('pending','retry')"`
	LockedUntil *time.Time `gorm:"index:notifications_stale_sending_idx,priority:1,where:status = 'sending'"`
	LockedBy    *string

	ProviderMessageID *string
	SentAt            *time.Time
	LastError         *string

	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;index:notifications_ready_idx,priority:2,where:status IN ('pending','retry');index:notifications_stale_sending_idx,priority:2,where:status = 'sending'"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`

	ProcessedEvent *ProcessedEvent `gorm:"foreignKey:EventID;references:EventID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (Notification) TableName() string {
	return "notifications"
}

type NotificationAttempt struct {
	ID int64 `gorm:"primaryKey;autoIncrement"`

	NotificationID string `gorm:"type:uuid;not null;uniqueIndex:notification_attempt_number_uidx,priority:1"`
	AttemptNumber  int    `gorm:"not null;uniqueIndex:notification_attempt_number_uidx,priority:2;check:notification_attempt_number_positive,attempt_number > 0"`
	WorkerID       string `gorm:"not null"`

	StartedAt  time.Time `gorm:"not null"`
	FinishedAt time.Time `gorm:"not null"`
	Outcome    string    `gorm:"not null;check:notification_attempt_outcome_valid,outcome IN ('sent','retry','dead')"`

	ProviderMessageID *string
	Error             *string

	Notification *Notification `gorm:"foreignKey:NotificationID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (NotificationAttempt) TableName() string {
	return "notification_attempts"
}
