package accounts

import (
	"bank/internal/transfers"
	"context"
	"errors"
	"log/slog"
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

type Repository interface {
	FindByUserID(ctx context.Context, userID string) (*Account, error)
	Transaction(ctx context.Context, fn func(AccountTransaction) error) error
}

type AccountTransaction interface {
	FindByUserIDForUpdate(userID string) (*Account, error)
	FindByIDForUpdate(accountID string) (*Account, error)
	UpdateBalance(account *Account, balance int64) error
	CreateTransfer(transfer *transfers.Transfer) error
}

type Service struct {
	repository Repository
	logger     *slog.Logger
}

func NewService(repository Repository, logger *slog.Logger) *Service {
	return &Service{repository: repository, logger: logger.With(slog.String("component", "account_service"))}
}

func (s *Service) GetAccountByUserID(ctx context.Context, userID string) (*Account, error) {
	if userID == "" {
		return nil, ErrInvalidID
	}
	return s.repository.FindByUserID(ctx, userID)
}

func (s *Service) Deposit(ctx context.Context, userID string, amount int64) (*Account, error) {
	if err := validateAmount(amount); err != nil {
		return nil, err
	}

	var account *Account
	var operation transfers.Transfer
	err := s.repository.Transaction(ctx, func(repository AccountTransaction) error {
		var err error
		account, err = repository.FindByUserIDForUpdate(userID)
		if err != nil {
			return err
		}
		if account.Currency != "RUB" {
			return ErrUnsupportedCur
		}

		newBalance := account.Balance + amount
		if err := repository.UpdateBalance(account, newBalance); err != nil {
			return err
		}
		account.Balance = newBalance
		operation = transfers.Transfer{ToAccID: &account.ID, Type: transfers.OperationDeposit, Currency: "RUB", Amount: amount}
		return repository.CreateTransfer(&operation)
	})
	if err != nil {
		s.logOperationError("failed to create deposit", operation.ID, err)
		return nil, err
	}

	s.logger.Info("deposit completed", slog.String("transfer_id", operation.ID), slog.String("account_id", account.ID), slog.Int64("amount", amount), slog.Int64("balance", account.Balance))
	return account, nil
}

func (s *Service) Withdraw(ctx context.Context, userID string, amount int64) (*Account, error) {
	if err := validateAmount(amount); err != nil {
		return nil, err
	}

	var account *Account
	var operation transfers.Transfer
	err := s.repository.Transaction(ctx, func(repository AccountTransaction) error {
		var err error
		account, err = repository.FindByUserIDForUpdate(userID)
		if err != nil {
			return err
		}
		if account.Currency != "RUB" {
			return ErrUnsupportedCur
		}
		if amount > account.Balance {
			return ErrNotEnoughMoney
		}

		newBalance := account.Balance - amount
		if err := repository.UpdateBalance(account, newBalance); err != nil {
			return err
		}
		account.Balance = newBalance
		operation = transfers.Transfer{FromAccID: &account.ID, Type: transfers.OperationWithdrawal, Currency: "RUB", Amount: amount}
		return repository.CreateTransfer(&operation)
	})
	if err != nil {
		s.logOperationError("failed to create withdraw", operation.ID, err)
		return nil, err
	}

	s.logger.Info("withdraw completed", slog.String("transfer_id", operation.ID), slog.String("account_id", account.ID), slog.Int64("amount", amount), slog.Int64("balance", account.Balance))
	return account, nil
}

func (s *Service) Transfer(ctx context.Context, userID string, amount int64, toAccountID string) (*Account, error) {
	if err := validateAmount(amount); err != nil {
		return nil, err
	}

	var account *Account
	var toAccount *Account
	var operation transfers.Transfer
	err := s.repository.Transaction(ctx, func(repository AccountTransaction) error {
		var err error
		account, err = repository.FindByUserIDForUpdate(userID)
		if err != nil {
			return err
		}
		if account.Currency != "RUB" {
			return ErrUnsupportedCur
		}
		if amount > account.Balance {
			return ErrNotEnoughMoney
		}

		newBalance := account.Balance - amount
		if err := repository.UpdateBalance(account, newBalance); err != nil {
			return err
		}
		account.Balance = newBalance

		toAccount, err = repository.FindByIDForUpdate(toAccountID)
		if err != nil {
			return err
		}
		if toAccount.Currency != "RUB" {
			return ErrUnsupportedCur
		}

		newToBalance := toAccount.Balance + amount
		if err := repository.UpdateBalance(toAccount, newToBalance); err != nil {
			return err
		}
		toAccount.Balance = newToBalance

		operation = transfers.Transfer{FromAccID: &account.ID, ToAccID: &toAccount.ID, Type: transfers.OperationTransfer, Currency: "RUB", Amount: amount}
		return repository.CreateTransfer(&operation)
	})
	if err != nil {
		s.logOperationError("failed to create transfer", operation.ID, err)
		return nil, err
	}

	s.logger.Info("transfer completed", slog.String("transfer_id", operation.ID), slog.String("account_id", account.ID), slog.Int64("amount", amount), slog.Int64("balance", account.Balance), slog.String("to_account_id", toAccount.ID), slog.Int64("to_balance", toAccount.Balance))
	return account, nil
}

func (s *Service) logOperationError(message, transferID string, err error) {
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnsupportedCur) || errors.Is(err, ErrNotEnoughMoney) {
		return
	}
	s.logger.Error(message, slog.String("transfer_id", transferID), slog.Any("error", err))
}

func validateAmount(amountMinor int64) error {
	if amountMinor <= 0 || amountMinor > maxOperationAmountMinor {
		return ErrInvalidAmount
	}
	return nil
}
