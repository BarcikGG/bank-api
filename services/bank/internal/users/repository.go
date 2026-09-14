package users

import (
	"bank/internal/accounts"
	"bank/internal/outbox"
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type PostgresRepository struct {
	db *gorm.DB
}

type postgresRegistrationRepository struct {
	db *gorm.DB
}

var _ Repository = (*PostgresRepository)(nil)
var _ RegistrationRepository = (*postgresRegistrationRepository)(nil)

func NewRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Transaction(ctx context.Context, fn func(RegistrationRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&postgresRegistrationRepository{db: tx})
	})
}

func (r *postgresRegistrationRepository) CreateUser(user *User) error {
	if err := r.db.Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrEmailTaken
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *postgresRegistrationRepository) CreateAccount(account *accounts.Account) error {
	if err := r.db.Create(account).Error; err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

func (r *postgresRegistrationRepository) AddEvent(event *outbox.Event) error {
	if err := r.db.Create(event).Error; err != nil {
		return fmt.Errorf("add registration event: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &user, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &user, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
