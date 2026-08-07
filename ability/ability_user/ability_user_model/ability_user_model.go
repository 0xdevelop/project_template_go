// Package ability_user_model ability/ability_user/ability_user_model/ability_user_model.go
package ability_user_model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model      `json:"-"`
	UserID          string     `gorm:"type:char(36);not null;uniqueIndex:uk_users_user_id" json:"user_id"`
	UserName        string     `gorm:"size:32;not null;uniqueIndex:uk_users_user_name" json:"user_name"`
	NickName        string     `gorm:"size:32;not null" json:"nick_name"`
	BindEmail       *string    `gorm:"size:320;uniqueIndex:uk_users_bind_email" json:"bind_email"`
	BindPhone       *string    `gorm:"size:32;uniqueIndex:uk_users_bind_phone" json:"bind_phone"`
	PasswordHash    string     `gorm:"size:255;not null" json:"-"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	PhoneVerifiedAt *time.Time `json:"phone_verified_at"`
}
