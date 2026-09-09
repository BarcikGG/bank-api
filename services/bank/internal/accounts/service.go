package accounts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"bank/internal/transfers"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidEmailInput = errors.New("email is required")
	ErrInvalidID         = errors.New("invalid ID")
	ErrNotFound          = errors.New("account not found")
	ErrInvalidAmount     = errors.New("amount must be positive")
	ErrUnsupportedCur    = errors.New("now supported only RUB")
	ErrNotEnoughMoney    = errors.New("not enough money on your balance")
)

const maxOperationAmountMinor int64 = 100_000_000_00

type Service struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewService(db *gorm.DB, logger *slog.Logger) *Service {
	return &Service{db: db, logger: logger.With(slog.String("component", "account_service"))}
}

func (s *Service) GetAccountByUserID(ctx context.Context, userID string) (*Account, error) {
	if userID == "" {
		return nil, ErrInvalidID
	}

	var account Account

	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("can't get account by id: %w", err)
	}

	return &account, nil
}

func (s *Service) Deposit(ctx context.Context, userID string, amount int64) (*Account, error) {
	err := validateAmount(amount)
	if err != nil {
		return nil, err
	}

	var account Account
	var operation transfers.Transfer

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
		}).
			Where("user_id = ?", userID).
			First(&account).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find account for deposit: %w", err)
		}

		if account.Currency != "RUB" {
			return ErrUnsupportedCur
		}

		newBalance := account.Balance + amount

		result := tx.Model(&account).Update("balance", newBalance)

		if result.Error != nil {
			return fmt.Errorf("update account balance: %w", result.Error)
		}

		if result.RowsAffected != 1 {
			return fmt.Errorf("update account balance: expected one updated row")
		}

		account.Balance = newBalance

		operation = transfers.Transfer{
			ToAccID:  &account.ID,
			Type:     transfers.OperationDeposit,
			Currency: "RUB",
			Amount:   amount,
		}

		if err := tx.Create(&operation).Error; err != nil {
			return fmt.Errorf("create deposit operation: %w", err)
		}

		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrUnsupportedCur) {
			s.logger.Error(
				"failed to create deposit",
				slog.String("transfer_id", operation.ID),
				slog.Any("error", err),
			)
		}

		return nil, err
	}

	s.logger.Info(
		"deposit completed",
		slog.String("transfer_id", operation.ID),
		slog.String("account_id", account.ID),
		slog.Int64("amount", amount),
		slog.Int64("balance", account.Balance),
	)

	return &account, nil
}

func (s *Service) Withdraw(ctx context.Context, userID string, amount int64) (*Account, error) {
	err := validateAmount(amount)
	if err != nil {
		return nil, err
	}

	var account Account
	var operation transfers.Transfer

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
		}).
			Where("user_id = ?", userID).
			First(&account).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find account for withdraw: %w", err)
		}

		if account.Currency != "RUB" {
			return ErrUnsupportedCur
		}

		if amount > account.Balance {
			return ErrNotEnoughMoney
		}

		newBalance := account.Balance - amount
		result := tx.Model(&account).Update("balance", newBalance)

		if result.Error != nil {
			return fmt.Errorf("update account balance: %w", result.Error)
		}

		if result.RowsAffected != 1 {
			return fmt.Errorf("update account balance: expected one updated row")
		}

		account.Balance = newBalance

		operation = transfers.Transfer{
			FromAccID: &account.ID,
			Type:      transfers.OperationWithdrawal,
			Currency:  "RUB",
			Amount:    amount,
		}

		if err := tx.Create(&operation).Error; err != nil {
			return fmt.Errorf("create withdraw operation: %w", err)
		}

		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrUnsupportedCur) && !errors.Is(err, ErrNotEnoughMoney) {
			s.logger.Error(
				"failed to create withdraw",
				slog.String("transfer_id", operation.ID),
				slog.Any("error", err),
			)
		}

		return nil, err
	}

	s.logger.Info(
		"withdraw completed",
		slog.String("transfer_id", operation.ID),
		slog.String("account_id", account.ID),
		slog.Int64("amount", amount),
		slog.Int64("balance", account.Balance),
	)

	return &account, nil
}

func (s *Service) Transfer(ctx context.Context, userID string, amount int64, toAccountID string) (*Account, error) {
	err := validateAmount(amount)
	if err != nil {
		return nil, err
	}

	var account Account
	var toAccount Account
	var operation transfers.Transfer
	var toOperation transfers.Transfer

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
		}).
			Where("user_id = ?", userID).
			First(&account).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find account for transfer: %w", err)
		}

		if account.Currency != "RUB" {
			return ErrUnsupportedCur
		}

		if amount > account.Balance {
			return ErrNotEnoughMoney
		}

		newBalance := account.Balance - amount

		result := tx.Model(&account).Update("balance", newBalance)

		if result.Error != nil {
			return fmt.Errorf("update account balance: %w", result.Error)
		}

		if result.RowsAffected != 1 {
			return fmt.Errorf("update account balance: expected one updated row")
		}

		account.Balance = newBalance

		operation = transfers.Transfer{
			FromAccID: &account.ID,
			ToAccID:   &toAccount.ID,
			Type:      transfers.OperationTransfer,
			Currency:  "RUB",
			Amount:    amount,
		}

		err = tx.Clauses(clause.Locking{
			Strength: "UPDATE",
		}).
			Where("id = ?", toAccountID).
			First(&toAccount).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find account for transfer: %w", err)
		}

		if toAccount.Currency != "RUB" {
			return ErrUnsupportedCur
		}

		newToBalance := toAccount.Balance + amount

		result = tx.Model(&toAccount).Update("balance", newToBalance)

		if result.Error != nil {
			return fmt.Errorf("update to account balance: %w", result.Error)
		}

		if result.RowsAffected != 1 {
			return fmt.Errorf("update to account balance: expected one updated row")
		}

		toAccount.Balance = newToBalance

		toOperation = transfers.Transfer{
			FromAccID: &account.ID,
			ToAccID:   &toAccount.ID,
			Type:      transfers.OperationTransfer,
			Currency:  "RUB",
			Amount:    amount,
		}

		if err := tx.Create(&toOperation).Error; err != nil {
			return fmt.Errorf("create transfer operation: %w", err)
		}

		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrUnsupportedCur) && !errors.Is(err, ErrNotEnoughMoney) {
			s.logger.Error(
				"failed to create withdraw",
				slog.String("transfer_id", operation.ID),
				slog.Any("error", err),
			)
		}

		return nil, err
	}

	s.logger.Info(
		"transfer completed",
		slog.String("transfer_id", operation.ID),
		slog.String("account_id", account.ID),
		slog.Int64("amount", amount),
		slog.Int64("balance", account.Balance),
		slog.String("to_account_id", toAccount.ID),
		slog.Int64("to_balance", toAccount.Balance),
	)

	return &account, nil
}

func validateAmount(amountMinor int64) error {
	if amountMinor <= 0 || amountMinor > maxOperationAmountMinor {
		return ErrInvalidAmount
	}

	return nil
}
