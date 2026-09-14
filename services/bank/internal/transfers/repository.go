package transfers

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type Repository interface {
	FindByUserID(ctx context.Context, userID string) ([]Transfer, error)
	FindByIDAndUserID(ctx context.Context, transferID, userID string) (*Transfer, error)
}

type PostgresRepository struct{ db *gorm.DB }

var _ Repository = (*PostgresRepository)(nil)

func NewRepository(db *gorm.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) FindByUserID(ctx context.Context, userID string) ([]Transfer, error) {
	result := make([]Transfer, 0)
	err := r.forUser(ctx, userID).Order("transfers.created_at DESC").Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("find transfers by user ID: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) FindByIDAndUserID(ctx context.Context, transferID, userID string) (*Transfer, error) {
	var transfer Transfer
	err := r.forUser(ctx, userID).Where("transfers.id = ?", transferID).First(&transfer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find transfer by ID: %w", err)
	}
	return &transfer, nil
}

func (r *PostgresRepository) forUser(ctx context.Context, userID string) *gorm.DB {
	return r.db.WithContext(ctx).
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
