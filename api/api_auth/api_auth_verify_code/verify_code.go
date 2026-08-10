// Package api_auth_verify_code implements verification code abilities for email and SMS channels.
package api_auth_verify_code

import (
	"context"
	"errors"
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	authModel "github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/config"
	appdb "github.com/0xdevelop/project_template_go/db"
	"github.com/george012/gtbox"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/resend/resend-go/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MethodVerificationCodeSendEmail  = "auth.verify_code.send.email"
	MethodVerificationCodeCheckEmail = "auth.verify_code.check.email"
	MethodVerificationCodeSendSMS    = "auth.verify_code.send.sms"
	MethodVerificationCodeCheckSMS   = "auth.verify_code.check.sms"
)

func LoadAPIMethods() {
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodVerificationCodeSendEmail,
		Description: "发送邮箱验证码",
		Public:      true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"email": api_auth_common.StringSchema("email", 3, 320),
			},
			"email",
		),
		Execute: VerificationCodeEmailSend,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodVerificationCodeCheckEmail,
		Description: "检查邮箱验证码",
		Public:      true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"email":       api_auth_common.StringSchema("email", 3, 320),
				"verify_code": api_auth_common.VerificationCodeSchema(),
			},
			"email",
			"verify_code",
		),
		Execute: VerificationCodeEmailCheck,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodVerificationCodeSendSMS,
		Description: "发送短信验证码（暂不支持）",
		Public:      true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"phone": api_auth_common.PhoneSchema(),
			},
			"phone",
		),
		Execute: VerificationCodePhoneSend,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodVerificationCodeCheckSMS,
		Description: "检查短信验证码（暂不支持）",
		Public:      true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"phone":       api_auth_common.PhoneSchema(),
				"verify_code": api_auth_common.VerificationCodeSchema(),
			},
			"phone",
			"verify_code",
		),
		Execute: VerificationCodePhoneCheck,
	})
}

func VerificationCodeEmailSend(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "email") {
		return nil, api_error_code.ErrInvalidArguments
	}
	emailValue, emailOK := api_auth_common.RequiredString(params, "email")
	if !emailOK {
		return nil, api_error_code.ErrInvalidArguments
	}
	email, err := api_auth_common.NormalizeEmail(emailValue)
	if err != nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	return sendVerificationCodeEmail(ctx, email)
}

func VerificationCodeEmailCheck(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "email", "verify_code") {
		return nil, api_error_code.ErrInvalidArguments
	}
	emailValue, emailOK := api_auth_common.RequiredString(params, "email")
	code, codeOK := api_auth_common.RequiredString(params, "verify_code")
	if !emailOK || !codeOK ||
		!api_auth_common.VerificationCodePattern.MatchString(code) {
		return nil, api_error_code.ErrInvalidArguments
	}
	email, err := api_auth_common.NormalizeEmail(emailValue)
	if err != nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	return checkVerificationCode(ctx, email, code)
}

func VerificationCodePhoneSend(context.Context, interface{}) (interface{}, error) {
	return nil, api_error_code.ErrMethodNotSupported
}

func VerificationCodePhoneCheck(context.Context, interface{}) (interface{}, error) {
	return nil, api_error_code.ErrMethodNotSupported
}

func sendVerificationCodeEmail(ctx context.Context, email string) (map[string]interface{}, error) {
	verificationConfig, err := api_auth_config.CurrentVerificationConfig()
	if err != nil {
		api_auth_common.LogInternalFailure("load verification config")
		return nil, api_error_code.ErrVerifyCodeDeliveryFailed
	}
	emailConfig, err := api_auth_config.CurrentEmailConfig()
	if err != nil {
		api_auth_common.LogInternalFailure("load email config")
		return nil, api_error_code.ErrVerifyCodeDeliveryFailed
	}

	verificationCode, code, err := prepareVerificationCode(ctx, email)
	if err != nil {
		api_auth_common.LogInternalFailure("prepare verification code")
		return nil, err
	}

	// 仅 Debug 运行模式输出验证码明文，供本地联调模拟无真实邮箱的注册；Release/Test 禁止（契约见 AGENTS.md Auth 节）。
	if config.CurrentApp != nil && config.CurrentApp.CurrentRunMode == gtbox.RunModeDebug {
		gtbox_log.LogDebugf("debug-only email verification code for %s: %s", email, code)
	}

	if err = deliverVerificationEmail(ctx, emailConfig, email, code, verificationConfig.CodeTTLSeconds); err != nil {
		updateContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if db, dbErr := appdb.MysqlDB(updateContext); dbErr == nil {
			_ = db.Model(&authModel.AuthVerifyCode{}).
				Where("id = ? AND code_hash = ? AND sent_at IS NULL", verificationCode.ID, verificationCode.CodeHash).
				Update("code_hash", "").Error
		}
		api_auth_common.LogInternalFailure(fmt.Sprintf("deliver email verification code: %v", err))
		return nil, api_error_code.ErrVerifyCodeDeliveryFailed
	}

	updateContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	db, err := appdb.MysqlDB(updateContext)
	if err != nil {
		return nil, err
	}
	result := db.Model(&authModel.AuthVerifyCode{}).
		Where("id = ? AND code_hash = ? AND sent_at IS NULL", verificationCode.ID, verificationCode.CodeHash).
		Update("sent_at", time.Now().UTC())
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("email verification code was superseded")
	}
	return map[string]interface{}{
		"sent":       true,
		"expires_in": verificationConfig.CodeTTLSeconds,
	}, nil
}

// deliverVerificationEmail 按配置选定的供应商发送验证码邮件，是唯一的邮件供应商调用收口。
func deliverVerificationEmail(ctx context.Context, emailConfig *api_auth_config.EmailConfig, email string, code string, ttlSeconds int) error {
	subject := strings.TrimSpace(emailConfig.VerificationSubject)
	productName := strings.TrimSpace(emailConfig.ProductName)
	ttlMinutes := int(math.Ceil(float64(ttlSeconds) / 60))

	switch strings.ToLower(strings.TrimSpace(emailConfig.Provider)) {
	case api_auth_config.EmailProviderResend:
		params := &resend.SendEmailRequest{
			From:    emailConfig.From,
			To:      []string{email},
			Subject: subject,
			ReplyTo: emailConfig.ReplyTo,
			Text: fmt.Sprintf(
				"Your %s verification code is %s. It expires in %d minutes.",
				productName,
				code,
				ttlMinutes,
			),
			Html: fmt.Sprintf(
				"<p>Your %s verification code is <strong>%s</strong>.</p><p>It expires in %d minutes.</p>",
				html.EscapeString(productName),
				html.EscapeString(code),
				ttlMinutes,
			),
		}
		client := resend.NewClient(emailConfig.APIKey)
		_, err := client.Emails.SendWithContext(ctx, params)
		return err
	default:
		return errors.New("unsupported email provider")
	}
}

// PurgeExpired 删除过期超 24 小时的验证码行（保留窗口供近期排障），作为 policy 定期轮询租户运行。
// 表内行只增不减，此清理是唯一出口；使用 Unscoped 硬删，软删行同样无保留价值。
func PurgeExpired(ctx context.Context) error {
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return err
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	result := db.Unscoped().
		Where("expires_at < ?", cutoff).
		Delete(&authModel.AuthVerifyCode{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		gtbox_log.LogInfof("auth verify code purge: removed %d expired row(s)", result.RowsAffected)
	}
	return nil
}

func prepareVerificationCode(ctx context.Context, recipient string) (*authModel.AuthVerifyCode, string, error) {
	verificationConfig, err := api_auth_config.CurrentVerificationConfig()
	if err != nil {
		return nil, "", err
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, "", err
	}

	for attempt := 0; attempt < 2; attempt++ {
		verificationCode := &authModel.AuthVerifyCode{}
		code := ""
		now := time.Now().UTC()
		err = db.Transaction(func(tx *gorm.DB) error {
			findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("recipient = ?", recipient).
				Take(verificationCode).Error
			if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			isNew := errors.Is(findErr, gorm.ErrRecordNotFound)
			if isNew {
				verificationCode.Recipient = recipient
			} else {
				if now.Before(verificationCode.ResendAvailableAt) {
					return api_error_code.ErrInvalidArguments
				}
				if now.Before(verificationCode.SendWindowStartedAt.Add(time.Hour)) &&
					verificationCode.SendCount >= verificationConfig.HourlySendLimit {
					return api_error_code.ErrInvalidArguments
				}
			}

			if verificationCode.SendWindowStartedAt.IsZero() ||
				!now.Before(verificationCode.SendWindowStartedAt.Add(time.Hour)) {
				verificationCode.SendWindowStartedAt = now
				verificationCode.SendCount = 0
			}
			code, err = api_auth_common.NewVerificationCode()
			if err != nil {
				return err
			}
			verificationCode.CodeHash = api_auth_common.VerificationCodeHash(
				verificationConfig.CodeHashSecret,
				recipient,
				code,
			)
			verificationCode.AttemptsRemaining = verificationConfig.MaxAttempts
			verificationCode.ExpiresAt = now.Add(
				time.Duration(verificationConfig.CodeTTLSeconds) * time.Second,
			)
			verificationCode.ResendAvailableAt = now.Add(
				time.Duration(verificationConfig.ResendIntervalSeconds) * time.Second,
			)
			verificationCode.SendCount++
			verificationCode.SentAt = nil

			if isNew {
				return tx.Create(verificationCode).Error
			}
			return tx.Save(verificationCode).Error
		})
		if err == nil {
			return verificationCode, code, nil
		}
		if !appdb.IsDuplicateKeyError(err) || attempt == 1 {
			return nil, "", err
		}
	}
	return nil, "", errors.New("unable to prepare verification code")
}

func checkVerificationCode(ctx context.Context, recipient string, code string) (map[string]interface{}, error) {
	verificationConfig, err := api_auth_config.CurrentVerificationConfig()
	if err != nil {
		return nil, err
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}
	var expiresAt time.Time
	var outcomeErr error
	err = db.Transaction(func(tx *gorm.DB) error {
		verificationCode := &authModel.AuthVerifyCode{}
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("recipient = ?", recipient).
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
		if !time.Now().UTC().Before(verificationCode.ExpiresAt) {
			outcomeErr = api_error_code.ErrInvalidArguments
			return nil
		}

		expectedHash := api_auth_common.VerificationCodeHash(verificationConfig.CodeHashSecret, recipient, code)
		if !api_auth_common.SecureStringEqual(verificationCode.CodeHash, expectedHash) {
			verificationCode.AttemptsRemaining--
			outcomeErr = api_error_code.ErrInvalidArguments
			return tx.Model(verificationCode).
				Update("attempts_remaining", verificationCode.AttemptsRemaining).Error
		}
		expiresAt = verificationCode.ExpiresAt
		return nil
	})
	if err != nil {
		return nil, err
	}
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	return map[string]interface{}{
		"valid":      true,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	}, nil
}
