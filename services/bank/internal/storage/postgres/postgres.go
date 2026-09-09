package postgres

import (
	"bank/internal/accounts"
	"bank/internal/auth"
	"bank/internal/outbox"
	"bank/internal/transfers"
	"bank/internal/users"
	"context"
	"fmt"
	"time"

	pg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Storage struct {
	DB *gorm.DB
}

func Connect(databaseUrl string) (*Storage, error) {
	db, err := gorm.Open(pg.Open(databaseUrl), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("Не удалось подключиться к БД postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("Не удалось установить соединение с БД postgres: %w", err)
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxIdleTime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("Не удалось проверить соединение с БД postgres: %w", err)
	}

	storage := &Storage{DB: db}

	if err := storage.autoMigrate(); err != nil {
		return nil, fmt.Errorf("Не удалось запустить автомиграции: %w", err)
	}

	return storage, nil
}

func (s *Storage) autoMigrate() error {
	return s.DB.AutoMigrate(
		&users.User{},
		&accounts.Account{},
		&transfers.Transfer{},
		&auth.Session{},
		&auth.RefreshToken{},
		&outbox.Event{},
	)
}
