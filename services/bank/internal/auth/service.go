package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")
var errAuthRecordNotFound = errors.New("auth record not found")

type Repository interface {
	CreateSession(ctx context.Context, session *Session, token *RefreshToken) error
	Transaction(ctx context.Context, fn func(SessionTransaction) error) error
	RevokeAllSessions(ctx context.Context, userID string, revokedAt time.Time) error
}

type SessionTransaction interface {
	FindRefreshTokenForUpdate(tokenHash string) (*RefreshToken, error)
	FindSessionForUpdate(sessionID string) (*Session, error)
	CreateRefreshToken(token *RefreshToken) error
	ConsumeRefreshToken(tokenID, replacementID string, usedAt time.Time) error
	RevokeSession(sessionID string, revokedAt time.Time) error
}

type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type Service struct {
	repository Repository
	tokens     *Manager
	refreshTTL time.Duration
	logger     *slog.Logger
}

func NewService(
	repository Repository,
	tokens *Manager,
	refreshTTL time.Duration,
	logger *slog.Logger,
) *Service {
	return &Service{
		repository: repository,
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

	storedToken := RefreshToken{TokenHash: refreshHash, ExpiresAt: refreshExpiresAt}
	err = s.repository.CreateSession(ctx, &session, &storedToken)
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

	err := s.repository.Transaction(ctx, func(repository SessionTransaction) error {
		currentToken, err := repository.FindRefreshTokenForUpdate(tokenHash)
		if errors.Is(err, errAuthRecordNotFound) {
			invalid = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("find refresh token: %w", err)
		}

		session, err := repository.FindSessionForUpdate(currentToken.SessionID)
		if errors.Is(err, errAuthRecordNotFound) {
			invalid = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("find auth session: %w", err)
		}

		now := time.Now().UTC()
		if session.RevokedAt != nil || !now.Before(session.ExpiresAt) {
			if err := repository.RevokeSession(session.ID, now); err != nil {
				return err
			}
			invalid = true
			return nil
		}

		if currentToken.UsedAt != nil || currentToken.RevokedAt != nil {
			if err := repository.RevokeSession(session.ID, now); err != nil {
				return err
			}
			replayedSessionID = session.ID
			invalid = true
			return nil
		}

		if !now.Before(currentToken.ExpiresAt) {
			if err := repository.RevokeSession(session.ID, now); err != nil {
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
		if err := repository.CreateRefreshToken(&newToken); err != nil {
			return err
		}

		if err := repository.ConsumeRefreshToken(currentToken.ID, newToken.ID, now); err != nil {
			return err
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

	err := s.repository.Transaction(ctx, func(repository SessionTransaction) error {
		refreshToken, err := repository.FindRefreshTokenForUpdate(tokenHash)
		if errors.Is(err, errAuthRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find refresh token for logout: %w", err)
		}

		sessionID = refreshToken.SessionID
		return repository.RevokeSession(sessionID, time.Now().UTC())
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

	err := s.repository.RevokeAllSessions(ctx, userID, time.Now().UTC())
	if err != nil {
		return err
	}

	s.logger.Info("all auth sessions revoked", slog.String("user_id", userID))
	return nil
}
