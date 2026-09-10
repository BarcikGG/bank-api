package transfers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

var (
	ErrInvalidUserID = errors.New("invalid user ID")
	ErrNotFound      = errors.New("not found")
)

type Service struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewService(db *gorm.DB, logger *slog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

func (s *Service) GetTransferHistory(ctx context.Context, userID string) ([]Transfer, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	transfers := make([]Transfer, 0)
	err := s.forUser(ctx, userID).
		Order("transfers.created_at DESC").
		Find(&transfers).Error
	if err != nil {
		return nil, fmt.Errorf("get transfer history: %w", err)
	}

	return transfers, nil
}

func (s *Service) GetTransferHistoryByID(ctx context.Context, userID string, transferID string) (Transfer, error) {
	if userID == "" {
		return Transfer{}, ErrInvalidUserID
	}

	var transfer Transfer
	err := s.forUser(ctx, userID).
		Where("transfers.id = ?", transferID).
		First(&transfer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Transfer{}, ErrNotFound
	}

	if err != nil {
		return Transfer{}, fmt.Errorf("get transfer by ID: %w", err)
	}

	return transfer, nil
}

func (s *Service) forUser(ctx context.Context, userID string) *gorm.DB {
	return s.db.WithContext(ctx).
		Model(&Transfer{}).
		Where(
			`EXISTS (
				SELECT 1
				FROM accounts
				WHERE accounts.user_id = ?
				AND (
					accounts.id = transfers.from_acc_id
					OR accounts.id = transfers.to_acc_id
				)
			)`,
			userID,
		)
}
