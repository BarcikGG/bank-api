package transfers

import (
	"context"
	"errors"
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

	var transfers []Transfer
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&transfers).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}
	return transfers, nil
}

func (s *Service) GetTransferHistoryByID(ctx context.Context, userID string, transferID string) (Transfer, error) {
	if userID == "" {
		return Transfer{}, ErrInvalidUserID
	}

	var transfer Transfer
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Where("id = ?", transferID).First(&transfer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Transfer{}, ErrNotFound
	}

	if err != nil {
		return Transfer{}, err
	}
	return transfer, nil
}
