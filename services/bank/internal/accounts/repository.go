package accounts

import (
	"bank/internal/transfers"
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresRepository struct{ db *gorm.DB }
type postgresAccountTransaction struct{ db *gorm.DB }

var _ Repository = (*PostgresRepository)(nil)
var _ AccountTransaction = (*postgresAccountTransaction)(nil)

func NewRepository(db *gorm.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) FindByUserID(ctx context.Context, userID string) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, accountError("find account by user ID", err)
	}
	return &account, nil
}

func (r *PostgresRepository) Transaction(ctx context.Context, fn func(AccountTransaction) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&postgresAccountTransaction{db: tx})
	})
}

func (r *postgresAccountTransaction) FindByUserIDForUpdate(userID string) (*Account, error) {
	var account Account
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&account).Error
	if err != nil {
		return nil, accountError("find account by user ID for update", err)
	}
	return &account, nil
}

func (r *postgresAccountTransaction) FindByIDForUpdate(accountID string) (*Account, error) {
	var account Account
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", accountID).First(&account).Error
	if err != nil {
		return nil, accountError("find account by ID for update", err)
	}
	return &account, nil
}

func (r *postgresAccountTransaction) UpdateBalance(account *Account, balance int64) error {
	result := r.db.Model(account).Update("balance", balance)
	if result.Error != nil {
		return fmt.Errorf("update account balance: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update account balance: expected one updated row")
	}
	return nil
}

func (r *postgresAccountTransaction) CreateTransfer(transfer *transfers.Transfer) error {
	if err := r.db.Create(transfer).Error; err != nil {
		return fmt.Errorf("create transfer operation: %w", err)
	}
	return nil
}

func accountError(operation string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
