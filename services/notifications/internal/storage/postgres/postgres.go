package postgres

import (
	"context"
	"fmt"
	"notifications/internal/notifications"
	"time"

	pg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Storage struct {
	DB *gorm.DB
}

func Connect(databaseURL string) (*Storage, error) {
	db, err := gorm.Open(pg.Open(databaseURL), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get SQL database: %w", err)
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetConnMaxIdleTime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	storage := &Storage{DB: db}
	if err := storage.autoMigrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("auto migrate notifications database: %w", err)
	}

	return storage, nil
}

func (s *Storage) autoMigrate() error {
	return s.DB.AutoMigrate(
		&notifications.ProcessedEvent{},
		&notifications.Notification{},
		&notifications.NotificationAttempt{},
	)
}

func (s *Storage) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("get SQL database: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close PostgreSQL: %w", err)
	}

	return nil
}
