// Package api_auth_verify_code implements verification code abilities for email and SMS channels.
package api_auth_verify_code

import (
	"context"
	"errors"
	"fmt"
	"html"
	"math"
	"net/mail"
	"strings"
	"time"

	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	authModel "github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/google/uuid"
	"github.com/resend/resend-go/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MethodVerificationCodeSendEmail  = "auth.verify_code.send.email"
	MethodVerificationCodeCheckEmail = "auth.verify_code.check.email"
	MethodVerificationCodeSendSMS    = "auth.verify_code.send.sms"
	MethodVerificationCodeCheckSMS   = "auth.verify_code.check.sms"
)

// Dependency seams keep provider/log tests entirely local; normal execution uses the service INFO logger and official SDK.
var verificationCodeInfof = gtbox_log.LogInfof
var sendEmailRequest = func(ctx context.Context, apiKey string, request *resend.SendEmailRequest) error {
	_, err := resend.NewClient(apiKey).Emails.SendWithContext(ctx, request)
	return err
}

func LoadAPIMethods() {
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodVerificationCodeSendEmail,
		Description: "生成邮箱验证码并记录 INFO 日志；邮件可选",
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
	if err != nil || !validVerificationRecipient(email) {
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
	if err != nil || !validVerificationRecipient(email) {
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
	// Every actual generation is INFO-logged by prepareVerificationCode, before storage/mail can fail.
	verificationCode, code, deliveryID, err := prepareVerificationCode(ctx, email)
	if err != nil {
		api_auth_common.LogInternalFailure("prepare verification code")
		return nil, err
	}
	emailEnabled := api_auth_config.EmailDeliveryEnabled()
	if emailEnabled {
		var emailConfig *api_auth_config.EmailConfig
		if api_auth_config.CurrentCfgAuth != nil && api_auth_config.CurrentCfgAuth.Email != nil {
			copyConfig := *api_auth_config.CurrentCfgAuth.Email
			emailConfig = &copyConfig
		}
		// Selected email dependencies are validated at this final boundary, after real generation/logging.
		if err = deliverVerificationEmail(ctx, emailConfig, email, code, verificationConfig.CodeTTLSeconds); err != nil {
			updateContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()

			db, cleanupErr := storage.GormDB(updateContext)
			if cleanupErr == nil {
				cleanupErr = db.Model(&authModel.AuthVerifyCode{}).
					Where("id = ? AND code_hash = ? AND sent_at IS NULL", verificationCode.ID, verificationCode.CodeHash).
					Update("code_hash", "").Error
			}
			if cleanupErr != nil {
				gtbox_log.LogWarnf("auth verification pending-code cleanup failed delivery_id=%s; code remains unactivated", deliveryID)
			}

			// Provider errors may contain addresses, codes or request bodies. Never log those values.
			gtbox_log.LogWarnf("auth verification email delivery failed delivery_id=%s; code not activated", deliveryID)
			return nil, api_error_code.ErrVerifyCodeDeliveryFailed
		}
	}
	// NULL sent_at gates verification/registration until the selected workflow has completed.
	updateContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	db, err := storage.GormDB(updateContext)
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
		return nil, errors.New("verification code was superseded")
	}
	return map[string]interface{}{
		"sent":        emailEnabled,
		"code_logged": true,
		"delivery_id": deliveryID,
		"expires_in":  verificationConfig.CodeTTLSeconds,
	}, nil
}

func validVerificationRecipient(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}

// deliverVerificationEmail 按配置选定的供应商发送验证码邮件，是唯一的邮件供应商调用收口。
func deliverVerificationEmail(ctx context.Context, emailConfig *api_auth_config.EmailConfig, email string, code string, ttlSeconds int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := api_auth_config.ValidateEmailConfig(emailConfig); err != nil {
		return err
	}
	if ttlSeconds < 1 || !api_auth_common.VerificationCodePattern.MatchString(code) {
		return errors.New("invalid verification delivery payload")
	}
	if _, err := api_auth_common.NormalizeEmail(email); err != nil {
		return errors.New("invalid verification recipient")
	}
	if !validVerificationRecipient(email) {
		return errors.New("invalid verification recipient")
	}

	subject := strings.TrimSpace(emailConfig.VerificationSubject)
	productName := strings.TrimSpace(emailConfig.ProductName)
	ttlMinutes := int(math.Ceil(float64(ttlSeconds) / 60))

	switch emailConfig.Provider {
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
		return sendEmailRequest(ctx, emailConfig.APIKey, params)
	default:
		return errors.New("unsupported email provider")
	}
}

// PurgeExpired 删除过期超 24 小时的验证码行（保留窗口供近期排障），作为 policy 定期轮询租户运行。
// 表内行只增不减，此清理是唯一出口；使用 Unscoped 硬删，软删行同样无保留价值。
func PurgeExpired(ctx context.Context) error {
	db, err := storage.GormDB(ctx)
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

func prepareVerificationCode(ctx context.Context, recipient string) (*authModel.AuthVerifyCode, string, string, error) {
	verificationConfig, err := api_auth_config.CurrentVerificationConfig()
	if err != nil {
		return nil, "", "", err
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, "", "", err
	}

	for attempt := 0; attempt < 2; attempt++ {
		verificationCode := &authModel.AuthVerifyCode{}
		code := ""
		deliveryID := ""
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
			deliveryUUID, uuidErr := uuid.NewRandom()
			if uuidErr != nil {
				return uuidErr
			}
			deliveryID = deliveryUUID.String()
			code, err = api_auth_common.NewVerificationCode()
			if err != nil {
				return err
			}
			// Every generated code in all run modes is logged, even if storage/mail later fails.
			// No recipient/user identity; an INFO entry does not imply validity or mail success.
			verificationCodeInfof("auth verification delivery_id=%s code=%s expires_at=%s validity=pending", deliveryID, code,
				now.Add(time.Duration(verificationConfig.CodeTTLSeconds)*time.Second).Format(time.RFC3339))
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
				time.Duration(*verificationConfig.SendIntervalSeconds) * time.Second,
			)
			verificationCode.SendCount++
			verificationCode.SentAt = nil

			if isNew {
				return tx.Create(verificationCode).Error
			}
			return tx.Save(verificationCode).Error
		})
		if err == nil {
			return verificationCode, code, deliveryID, nil
		}
		if !storage.IsDuplicateKeyError(err) || attempt == 1 {
			return nil, "", "", err
		}
	}
	return nil, "", "", errors.New("unable to prepare verification code")
}

func checkVerificationCode(ctx context.Context, recipient string, code string) (map[string]interface{}, error) {
	verificationConfig, err := api_auth_config.CurrentVerificationConfig()
	if err != nil {
		return nil, err
	}
	db, err := storage.GormDB(ctx)
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
