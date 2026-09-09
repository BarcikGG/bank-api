package users

import (
	"bank/internal/accounts"
	"bank/internal/outbox"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type EventWriter interface {
	Add(tx *gorm.DB, event *outbox.Event) error
}

type Service struct {
	db          *gorm.DB
	logger      *slog.Logger
	eventWriter EventWriter
}

var (
	ErrInvalidInput       = errors.New("name, email and password are required")
	ErrInvalidID          = errors.New("invalid ID")
	ErrNotFound           = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type CreateInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

func NewService(db *gorm.DB, logger *slog.Logger, eventWriter EventWriter) *Service {
	return &Service{db: db, logger: logger.With(slog.String("component", "users_service")), eventWriter: eventWriter}
}

func (s *Service) Register(ctx context.Context, input CreateInput) (*User, error) {
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	parsedEmail, err := mail.ParseAddress(email)
	if name == "" || err != nil || parsedEmail.Address != email || password == "" {
		return nil, ErrInvalidInput
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := User{
		Name:     name,
		Email:    email,
		Password: hashedPassword,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrEmailTaken
			}

			return fmt.Errorf("create user: %w", err)
		}

		account := accounts.Account{
			UserID:   user.ID,
			Balance:  0,
			Currency: "RUB",
		}

		if err := tx.Create(&account).Error; err != nil {
			return fmt.Errorf("create account: %w", err)
		}

		data, err := json.Marshal(outbox.UserRegisteredV1{
			UserID: user.ID,
			Email:  user.Email,
			Name:   user.Name,
		})
		if err != nil {
			return fmt.Errorf("marshal user registered: %w", err)
		}

		now := time.Now().UTC()
		event := outbox.Event{
			Topic:         "bank.events.v1",
			PartitionKey:  user.ID,
			EventType:     "bank.user.registered",
			EventVersion:  1,
			AggregateType: "user",
			AggregateID:   user.ID,
			Payload:       datatypes.JSON(data),
			OccurredAt:    now,
			AvailableAt:   now,
		}

		if err := s.eventWriter.Add(tx, &event); err != nil {
			return fmt.Errorf("add registration event: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error(
			"failed to create account or user",
			slog.String("user_id", user.ID),
			slog.Any("error", err),
		)
		return nil, err
	}

	s.logger.Info("user registered", slog.String("user_id", user.ID))
	return &user, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	var user User

	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	if !checkPasswordHash(password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	return &user, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	if id == "" {
		return nil, ErrInvalidID
	}

	var user User

	err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("can't get user by id: %w", err)
	}

	return &user, nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
