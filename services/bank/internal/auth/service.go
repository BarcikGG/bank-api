package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type Service struct {
	db         *gorm.DB
	tokens     *Manager
	refreshTTL time.Duration
	logger     *slog.Logger
}

func NewService(
	db *gorm.DB,
	tokens *Manager,
	refreshTTL time.Duration,
	logger *slog.Logger,
) *Service {
	return &Service{
		db:         db,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		logger:     logger.With(slog.String("component", "auth_service")),
	}
}

func (s *Service) StartSession(ctx context.Context, userID string) (*TokenPair, error) {
	accessToken, accessExpiresAt, err := s.tokens.Issue(userID)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refreshToken, refreshHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now().UTC()
	refreshExpiresAt := now.Add(s.refreshTTL)
	session := Session{
		UserID:    userID,
		ExpiresAt: refreshExpiresAt,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&session).Error; err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		storedToken := RefreshToken{
			SessionID: session.ID,
			TokenHash: refreshHash,
			ExpiresAt: refreshExpiresAt,
		}
		if err := tx.Create(&storedToken).Error; err != nil {
			return fmt.Errorf("store refresh token: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logger.Info(
		"auth session started",
		slog.String("session_id", session.ID),
		slog.String("user_id", userID),
	)

	return &TokenPair{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*TokenPair, error) {
	if rawRefreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	tokenHash := HashRefreshToken(rawRefreshToken)
	var pair *TokenPair
	var invalid bool
	var replayedSessionID string

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var currentToken RefreshToken
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash).
			First(&currentToken).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			invalid = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("find refresh token: %w", err)
		}

		var session Session
		err = tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", currentToken.SessionID).
			First(&session).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			invalid = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("find auth session: %w", err)
		}

		now := time.Now().UTC()
		if session.RevokedAt != nil || !now.Before(session.ExpiresAt) {
			if err := revokeSession(tx, session.ID, now); err != nil {
				return err
			}
			invalid = true
			return nil
		}

		if currentToken.UsedAt != nil || currentToken.RevokedAt != nil {
			if err := revokeSession(tx, session.ID, now); err != nil {
				return err
			}
			replayedSessionID = session.ID
			invalid = true
			return nil
		}

		if !now.Before(currentToken.ExpiresAt) {
			if err := revokeSession(tx, session.ID, now); err != nil {
				return err
			}
			invalid = true
			return nil
		}

		newRawToken, newTokenHash, err := GenerateRefreshToken()
		if err != nil {
			return fmt.Errorf("generate rotated refresh token: %w", err)
		}

		accessToken, accessExpiresAt, err := s.tokens.Issue(session.UserID)
		if err != nil {
			return fmt.Errorf("issue access token: %w", err)
		}

		newToken := RefreshToken{
			SessionID: session.ID,
			TokenHash: newTokenHash,
			ExpiresAt: session.ExpiresAt,
		}
		if err := tx.Create(&newToken).Error; err != nil {
			return fmt.Errorf("store rotated refresh token: %w", err)
		}

		if err := tx.Model(&RefreshToken{}).
			Where("id = ?", currentToken.ID).
			Updates(map[string]any{
				"used_at":        now,
				"replaced_by_id": newToken.ID,
			}).Error; err != nil {
			return fmt.Errorf("consume refresh token: %w", err)
		}

		pair = &TokenPair{
			AccessToken:      accessToken,
			AccessExpiresAt:  accessExpiresAt,
			RefreshToken:     newRawToken,
			RefreshExpiresAt: session.ExpiresAt,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if invalid {
		if replayedSessionID != "" {
			s.logger.Warn(
				"refresh token replay detected; session revoked",
				slog.String("session_id", replayedSessionID),
			)
		}
		return nil, ErrInvalidRefreshToken
	}

	return pair, nil
}

func (s *Service) RevokeSession(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}

	tokenHash := HashRefreshToken(rawRefreshToken)
	var sessionID string

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var refreshToken RefreshToken
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash).
			First(&refreshToken).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find refresh token for logout: %w", err)
		}

		sessionID = refreshToken.SessionID
		return revokeSession(tx, sessionID, time.Now().UTC())
	})
	if err != nil {
		return err
	}

	if sessionID != "" {
		s.logger.Info("auth session revoked", slog.String("session_id", sessionID))
	}

	return nil
}

func (s *Service) RevokeAllSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalidToken
	}

	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sessionIDs := tx.
			Model(&Session{}).
			Select("id").
			Where("user_id = ?", userID)

		if err := tx.Model(&RefreshToken{}).
			Where("session_id IN (?) AND revoked_at IS NULL", sessionIDs).
			Update("revoked_at", now).
			Error; err != nil {
			return fmt.Errorf("revoke user refresh tokens: %w", err)
		}

		if err := tx.Model(&Session{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", now).
			Error; err != nil {
			return fmt.Errorf("revoke user sessions: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.logger.Info("all auth sessions revoked", slog.String("user_id", userID))
	return nil
}

func revokeSession(tx *gorm.DB, sessionID string, revokedAt time.Time) error {
	if err := tx.Model(&RefreshToken{}).
		Where("session_id = ? AND revoked_at IS NULL", sessionID).
		Update("revoked_at", revokedAt).
		Error; err != nil {
		return fmt.Errorf("revoke session refresh tokens: %w", err)
	}

	if err := tx.Model(&Session{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Update("revoked_at", revokedAt).
		Error; err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}
