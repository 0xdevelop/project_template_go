// Package ability_user implements protocol-neutral User abilities.
package ability_user

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/0xYeah/project_template_go/ability/ability_user/ability_user_model"
	appdb "github.com/0xYeah/project_template_go/db"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid user credentials")
	ErrInvalidUserName    = errors.New("invalid user name")
	ErrInvalidNickName    = errors.New("invalid nick name")

	userNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
)

// NormalizeUserName 统一用户名 canonical form；UserName 是注册必填主标识，无修改接口。
func NormalizeUserName(rawUserName string) (string, error) {
	userName := strings.ToLower(strings.TrimSpace(rawUserName))
	if !userNamePattern.MatchString(userName) {
		return "", ErrInvalidUserName
	}
	return userName, nil
}

func Create(tx *gorm.DB, userName string, email string, password string, emailVerifiedAt time.Time) (*ability_user_model.User, error) {
	if tx == nil || userName == "" || email == "" {
		return nil, errors.New("invalid user creation input")
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	record := &ability_user_model.User{
		UserID:          uuid.NewString(),
		UserName:        userName,
		NickName:        userName,
		BindEmail:       &email,
		PasswordHash:    passwordHash,
		EmailVerifiedAt: &emailVerifiedAt,
	}
	if err = tx.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// NicknameChangeByUserID 更新昵称；NickName 创建时默认取 UserName，可随时修改。
func NicknameChangeByUserID(ctx context.Context, userID string, rawNickName string) (*ability_user_model.User, error) {
	nickName := strings.TrimSpace(rawNickName)
	if userID == "" || nickName == "" || len(nickName) > 32 {
		return nil, ErrInvalidNickName
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}
	record := &ability_user_model.User{}
	if err = db.Where("user_id = ?", userID).Take(record).Error; err != nil {
		return nil, err
	}
	if err = db.Model(record).Update("nick_name", nickName).Error; err != nil {
		return nil, err
	}
	record.NickName = nickName
	return record, nil
}

func AuthenticateEmail(ctx context.Context, email string, password string) (*ability_user_model.User, error) {
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}
	record := &ability_user_model.User{}
	err = db.Where("bind_email = ?", email).Take(record).Error
	return authenticate(record, password, err)
}

func AuthenticatePhone(ctx context.Context, phone string, password string) (*ability_user_model.User, error) {
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}
	record := &ability_user_model.User{}
	err = db.Where("bind_phone = ?", phone).Take(record).Error
	return authenticate(record, password, err)
}

func FindByUserID(db *gorm.DB, userID string) (*ability_user_model.User, error) {
	if db == nil {
		return nil, errors.New("mysql database is not initialized")
	}
	record := &ability_user_model.User{}
	err := db.Where("user_id = ?", userID).Take(record).Error
	if err != nil {
		return nil, err
	}
	return record, nil
}

func authenticate(record *ability_user_model.User, password string, findErr error) (*ability_user_model.User, error) {
	if errors.Is(findErr, gorm.ErrRecordNotFound) {
		burnPasswordVerificationCost(password)
		return nil, ErrInvalidCredentials
	}
	if findErr != nil {
		return nil, findErr
	}
	matches, err := verifyPassword(record.PasswordHash, password)
	if err != nil {
		return nil, err
	}
	if !matches {
		return nil, ErrInvalidCredentials
	}
	return record, nil
}
