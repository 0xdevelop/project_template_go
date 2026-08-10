// Package api_auth_model api/api_auth/api_auth_model/api_auth_model.go
package api_auth_model

import (
	"time"

	"gorm.io/gorm"
)

type AuthSession struct {
	gorm.Model
	SessionID        string     `gorm:"type:char(36);not null;uniqueIndex"`
	UserID           string     `gorm:"type:char(36);not null;index"`
	RefreshTokenHash string     `gorm:"type:char(64);not null"`
	ExpiresAt        time.Time  `gorm:"not null;index"`
	RevokedAt        *time.Time `gorm:"index"`
}

type AuthVerifyCode struct {
	gorm.Model
	Recipient           string    `gorm:"size:320;not null;uniqueIndex"`
	CodeHash            string    `gorm:"size:64;not null"`
	AttemptsRemaining   int       `gorm:"not null"`
	ExpiresAt           time.Time `gorm:"not null;index"`
	ResendAvailableAt   time.Time `gorm:"not null"`
	SendWindowStartedAt time.Time `gorm:"not null"`
	SendCount           int       `gorm:"not null"`
	SentAt              *time.Time
}
