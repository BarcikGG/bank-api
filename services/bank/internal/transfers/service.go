package transfers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

var (
	ErrInvalidUserID = errors.New("invalid user ID")
	ErrNotFound      = errors.New("not found")
)

type Service struct {
	repository Repository
	logger     *slog.Logger
}

func NewService(repository Repository, logger *slog.Logger) *Service {
	return &Service{repository: repository, logger: logger}
}

func (s *Service) GetTransferHistory(ctx context.Context, userID string) ([]Transfer, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	transfers, err := s.repository.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get transfer history: %w", err)
	}

	return transfers, nil
}

func (s *Service) GetTransferHistoryByID(ctx context.Context, userID string, transferID string) (Transfer, error) {
	if userID == "" {
		return Transfer{}, ErrInvalidUserID
	}

	transfer, err := s.repository.FindByIDAndUserID(ctx, transferID, userID)
	if errors.Is(err, ErrNotFound) {
		return Transfer{}, ErrNotFound
	}

	if err != nil {
		return Transfer{}, fmt.Errorf("get transfer by ID: %w", err)
	}

	return *transfer, nil
}
