package api_auth_register

import (
	"context"
	"errors"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_user"
	"github.com/0xdevelop/project_template_go/ability/ability_user/ability_user_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	appdb "github.com/0xdevelop/project_template_go/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MethodRegister = "auth.register"

func LoadAPIMethods() {
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodRegister,
		Description: "使用邮箱验证码注册账户；user_name 为注册必填主标识",
		Public:      true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"user_name":   api_auth_common.StringSchema("", 3, 32),
				"email":       api_auth_common.StringSchema("email", 3, 320),
				"password":    api_auth_common.StringSchema("", 15, 128),
				"verify_code": api_auth_common.VerificationCodeSchema(),
			},
			"user_name",
			"email",
			"password",
			"verify_code",
		),
		Execute: AuthRegister,
	})
}

func AuthRegister(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "user_name", "email", "password", "verify_code") {
		return nil, api_error_code.ErrInvalidArguments
	}
	userNameValue, userNameOK := api_auth_common.RequiredString(params, "user_name")
	emailValue, emailOK := api_auth_common.RequiredString(params, "email")
	password, passwordOK := api_auth_common.RequiredRawString(params, "password")
	code, codeOK := api_auth_common.RequiredString(params, "verify_code")
	if !userNameOK || !emailOK || !passwordOK || !codeOK ||
		!api_auth_common.VerificationCodePattern.MatchString(code) {
		return nil, api_error_code.ErrInvalidArguments
	}
	userName, err := ability_user.NormalizeUserName(userNameValue)
	if err != nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	email, err := api_auth_common.NormalizeEmail(emailValue)
	if err != nil || ability_user.ValidatePassword(password) != nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	return registerWithEmailCode(ctx, userName, email, password, code)
}

func registerWithEmailCode(ctx context.Context, userName string, email string, password string, code string) (map[string]interface{}, error) {
	verificationConfig, err := api_auth_config.CurrentVerificationConfig()
	if err != nil {
		return nil, err
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}
	var registeredUser *ability_user_model.User
	var outcomeErr error
	err = db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		verificationCode := &api_auth_model.AuthVerifyCode{}
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("recipient = ?", email).
			Take(verificationCode).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			outcomeErr = api_error_code.ErrInvalidArguments
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if verificationCode.AttemptsRemaining < 1 {
			outcomeErr = api_error_code.ErrInvalidArguments
			return nil
		}
		if verificationCode.SentAt == nil || verificationCode.CodeHash == "" {
			outcomeErr = api_error_code.ErrInvalidArguments
			return nil
		}
		if !now.Before(verificationCode.ExpiresAt) {
			outcomeErr = api_error_code.ErrInvalidArguments
			return nil
		}

		expectedHash := api_auth_common.VerificationCodeHash(verificationConfig.CodeHashSecret, email, code)
		if !api_auth_common.SecureStringEqual(verificationCode.CodeHash, expectedHash) {
			verificationCode.AttemptsRemaining--
			outcomeErr = api_error_code.ErrInvalidArguments
			if updateErr := tx.Model(verificationCode).
				Update("attempts_remaining", verificationCode.AttemptsRemaining).Error; updateErr != nil {
				return updateErr
			}
			return nil
		}

		user, createErr := ability_user.Create(
			tx,
			userName,
			email,
			password,
			now,
		)
		if createErr != nil {
			if !appdb.IsDuplicateKeyError(createErr) {
				return createErr
			}
			outcomeErr = api_error_code.ErrInvalidArguments
		}
		if updateErr := tx.Model(verificationCode).Updates(map[string]interface{}{
			"code_hash": "",
			"sent_at":   nil,
		}).Error; updateErr != nil {
			return updateErr
		}
		if outcomeErr == nil {
			registeredUser = user
		}
		return nil
	})
	if err != nil {
		api_auth_common.LogInternalFailure("register user transaction")
		return nil, err
	}
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	return map[string]interface{}{"user": registeredUser}, nil
}
