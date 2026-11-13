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

// AuthAPIKey 是用户级长期凭证。明文只在签发时返回一次，库中存 sha256。
// APIKeyID 是签发时另生成的 UUID 行句柄，不是调用方身份；身份是 UserID。
// 调用方一次只带一种凭证：Authorization: Bearer key_…，或兼容字段 arguments.jwt_token。
type AuthAPIKey struct {
	gorm.Model
	APIKeyID   string `gorm:"type:char(36);not null;uniqueIndex"`
	UserID     string `gorm:"type:char(36);not null;index"`
	Name       string `gorm:"size:64;not null"`
	KeyHash    string `gorm:"type:char(64);not null;uniqueIndex"`
	KeyPrefix  string `gorm:"size:16;not null"`
	LastUsedAt *time.Time
	RevokedAt  *time.Time `gorm:"index"`
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
