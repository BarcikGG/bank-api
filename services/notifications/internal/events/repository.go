package events

import (
	"context"
	"fmt"
	"notifications/internal/notifications"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresRepository struct{ db *gorm.DB }

var _ Repository = (*PostgresRepository)(nil)

func NewRepository(db *gorm.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) Save(
	ctx context.Context,
	event notifications.ProcessedEvent,
	jobs []notifications.Notification,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}},
			DoNothing: true,
		}).Create(&event)
		if result.Error != nil {
			return fmt.Errorf("insert processed event: %w", result.Error)
		}
		if result.RowsAffected == 0 || len(jobs) == 0 {
			return nil
		}
		if err := tx.Create(&jobs).Error; err != nil {
			return fmt.Errorf("insert notification jobs: %w", err)
		}
		return nil
	})
}
