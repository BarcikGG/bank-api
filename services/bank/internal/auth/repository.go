package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresRepository struct{ db *gorm.DB }
type postgresSessionTransaction struct{ db *gorm.DB }

var _ Repository = (*PostgresRepository)(nil)
var _ SessionTransaction = (*postgresSessionTransaction)(nil)

func NewRepository(db *gorm.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) CreateSession(ctx context.Context, session *Session, token *RefreshToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		token.SessionID = session.ID
		if err := tx.Create(token).Error; err != nil {
			return fmt.Errorf("store refresh token: %w", err)
		}
		return nil
	})
}

func (r *PostgresRepository) Transaction(ctx context.Context, fn func(SessionTransaction) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&postgresSessionTransaction{db: tx})
	})
}

func (r *PostgresRepository) RevokeAllSessions(ctx context.Context, userID string, revokedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sessionIDs := tx.Model(&Session{}).Select("id").Where("user_id = ?", userID)
		if err := tx.Model(&RefreshToken{}).
			Where("session_id IN (?) AND revoked_at IS NULL", sessionIDs).
			Update("revoked_at", revokedAt).Error; err != nil {
			return fmt.Errorf("revoke user refresh tokens: %w", err)
		}
		if err := tx.Model(&Session{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", revokedAt).Error; err != nil {
			return fmt.Errorf("revoke user sessions: %w", err)
		}
		return nil
	})
}

func (r *postgresSessionTransaction) FindRefreshTokenForUpdate(tokenHash string) (*RefreshToken, error) {
	var token RefreshToken
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", tokenHash).First(&token).Error
	if err != nil {
		return nil, authRecordError("find refresh token", err)
	}
	return &token, nil
}

func (r *postgresSessionTransaction) FindSessionForUpdate(sessionID string) (*Session, error) {
	var session Session
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", sessionID).First(&session).Error
	if err != nil {
		return nil, authRecordError("find auth session", err)
	}
	return &session, nil
}

func (r *postgresSessionTransaction) CreateRefreshToken(token *RefreshToken) error {
	if err := r.db.Create(token).Error; err != nil {
		return fmt.Errorf("store rotated refresh token: %w", err)
	}
	return nil
}

func (r *postgresSessionTransaction) ConsumeRefreshToken(tokenID, replacementID string, usedAt time.Time) error {
	if err := r.db.Model(&RefreshToken{}).Where("id = ?", tokenID).Updates(map[string]any{
		"used_at": usedAt, "replaced_by_id": replacementID,
	}).Error; err != nil {
		return fmt.Errorf("consume refresh token: %w", err)
	}
	return nil
}

func (r *postgresSessionTransaction) RevokeSession(sessionID string, revokedAt time.Time) error {
	if err := r.db.Model(&RefreshToken{}).
		Where("session_id = ? AND revoked_at IS NULL", sessionID).
		Update("revoked_at", revokedAt).Error; err != nil {
		return fmt.Errorf("revoke session refresh tokens: %w", err)
	}
	if err := r.db.Model(&Session{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Update("revoked_at", revokedAt).Error; err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func authRecordError(operation string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errAuthRecordNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
