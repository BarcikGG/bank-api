package auth

import "time"

type Session struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    string     `gorm:"type:uuid;not null;index"`
	CreatedAt time.Time  `gorm:"not null"`
	ExpiresAt time.Time  `gorm:"not null;index"`
	RevokedAt *time.Time `gorm:"index"`
}

type RefreshToken struct {
	ID           string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	SessionID    string     `gorm:"type:uuid;not null;index"`
	Session      Session    `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE"`
	TokenHash    string     `gorm:"type:char(64);not null;uniqueIndex"`
	CreatedAt    time.Time  `gorm:"not null"`
	ExpiresAt    time.Time  `gorm:"not null;index"`
	UsedAt       *time.Time `gorm:"index"`
	RevokedAt    *time.Time `gorm:"index"`
	ReplacedByID *string    `gorm:"type:uuid"`
}
