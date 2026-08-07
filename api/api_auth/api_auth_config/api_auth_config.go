// Package api_auth_config defines Auth configuration loaded by config.
package api_auth_config

import (
	"errors"
	"strings"

	"github.com/george012/gtbox/gtbox_log"
)

var (
	CurrentCfgAuth *AuthConfig
)

// EmailProviderResend 是当前唯一支持的邮件供应商取值。
const EmailProviderResend = "resend"

type AuthConfig struct {
	Email        *EmailConfig        `yaml:"email" json:"email" toml:"email" comment:"Email delivery configuration"`
	Verification *VerificationConfig `yaml:"verification" json:"verification" toml:"verification" comment:"Verification code policy"`
	Session      *SessionConfig      `yaml:"session" json:"session" toml:"session" comment:"JWT and refresh session policy"`
}

type EmailConfig struct {
	Provider            string `yaml:"provider" json:"provider" toml:"provider"`
	APIKey              string `yaml:"api_key" json:"api_key" toml:"api_key"`
	From                string `yaml:"from" json:"from" toml:"from"`
	ReplyTo             string `yaml:"reply_to" json:"reply_to" toml:"reply_to"`
	ProductName         string `yaml:"product_name" json:"product_name" toml:"product_name"`
	VerificationSubject string `yaml:"verification_subject" json:"verification_subject" toml:"verification_subject"`
}

type VerificationConfig struct {
	CodeTTLSeconds        int    `yaml:"code_ttl_seconds" json:"code_ttl_seconds" toml:"code_ttl_seconds"`
	MaxAttempts           int    `yaml:"max_attempts" json:"max_attempts" toml:"max_attempts"`
	ResendIntervalSeconds int    `yaml:"resend_interval_seconds" json:"resend_interval_seconds" toml:"resend_interval_seconds"`
	HourlySendLimit       int    `yaml:"hourly_send_limit" json:"hourly_send_limit" toml:"hourly_send_limit"`
	CodeHashSecret        string `yaml:"code_hash_secret" json:"code_hash_secret" toml:"code_hash_secret"`
}

type SessionConfig struct {
	JWTSigningSecret       string `yaml:"jwt_signing_secret" json:"jwt_signing_secret" toml:"jwt_signing_secret"`
	Issuer                 string `yaml:"issuer" json:"issuer" toml:"issuer"`
	Audience               string `yaml:"audience" json:"audience" toml:"audience"`
	AccessTokenTTLSeconds  int    `yaml:"access_token_ttl_seconds" json:"access_token_ttl_seconds" toml:"access_token_ttl_seconds"`
	RefreshTokenTTLSeconds int    `yaml:"refresh_token_ttl_seconds" json:"refresh_token_ttl_seconds" toml:"refresh_token_ttl_seconds"`
}

// invalidConfigField 记录指向具体配置键的告警日志，帮助后端快速定位配置问题；敏感值只报字段名不报内容。
func invalidConfigField(configKeyPath string, reason string) {
	gtbox_log.LogWarnf("auth config invalid: %s %s", configKeyPath, reason)
}

func CurrentEmailConfig() (*EmailConfig, error) {
	if CurrentCfgAuth == nil || CurrentCfgAuth.Email == nil {
		invalidConfigField("auth_cfg.email", "block is not configured")
		return nil, errors.New("email config is not initialized")
	}
	config := CurrentCfgAuth.Email
	if strings.ToLower(strings.TrimSpace(config.Provider)) != EmailProviderResend {
		invalidConfigField("auth_cfg.email.provider", "is not a supported email provider")
		return nil, errors.New("unsupported email provider")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		invalidConfigField("auth_cfg.email.api_key", "is empty")
		return nil, errors.New("email config is invalid")
	}
	if strings.TrimSpace(config.From) == "" {
		invalidConfigField("auth_cfg.email.from", "is empty")
		return nil, errors.New("email config is invalid")
	}
	if strings.TrimSpace(config.ProductName) == "" {
		invalidConfigField("auth_cfg.email.product_name", "is empty")
		return nil, errors.New("email config is invalid")
	}
	if strings.TrimSpace(config.VerificationSubject) == "" {
		invalidConfigField("auth_cfg.email.verification_subject", "is empty")
		return nil, errors.New("email config is invalid")
	}
	return config, nil
}

func CurrentVerificationConfig() (*VerificationConfig, error) {
	if CurrentCfgAuth == nil || CurrentCfgAuth.Verification == nil {
		invalidConfigField("auth_cfg.verification", "block is not configured")
		return nil, errors.New("verification config is not initialized")
	}
	config := CurrentCfgAuth.Verification
	if config.CodeTTLSeconds < 1 {
		invalidConfigField("auth_cfg.verification.code_ttl_seconds", "must be >= 1")
		return nil, errors.New("verification config is invalid")
	}
	if config.MaxAttempts < 1 {
		invalidConfigField("auth_cfg.verification.max_attempts", "must be >= 1")
		return nil, errors.New("verification config is invalid")
	}
	if config.ResendIntervalSeconds < 1 {
		invalidConfigField("auth_cfg.verification.resend_interval_seconds", "must be >= 1")
		return nil, errors.New("verification config is invalid")
	}
	if config.HourlySendLimit < 1 {
		invalidConfigField("auth_cfg.verification.hourly_send_limit", "must be >= 1")
		return nil, errors.New("verification config is invalid")
	}
	if len(config.CodeHashSecret) < 32 {
		invalidConfigField("auth_cfg.verification.code_hash_secret", "must be at least 32 characters")
		return nil, errors.New("verification config is invalid")
	}
	return config, nil
}

func CurrentSessionConfig() (*SessionConfig, error) {
	if CurrentCfgAuth == nil || CurrentCfgAuth.Session == nil {
		invalidConfigField("auth_cfg.session", "block is not configured")
		return nil, errors.New("session config is not initialized")
	}
	config := CurrentCfgAuth.Session
	if len(config.JWTSigningSecret) < 32 {
		invalidConfigField("auth_cfg.session.jwt_signing_secret", "must be at least 32 characters")
		return nil, errors.New("session config is invalid")
	}
	if strings.TrimSpace(config.Issuer) == "" {
		invalidConfigField("auth_cfg.session.issuer", "is empty")
		return nil, errors.New("session config is invalid")
	}
	if strings.TrimSpace(config.Audience) == "" {
		invalidConfigField("auth_cfg.session.audience", "is empty")
		return nil, errors.New("session config is invalid")
	}
	if config.AccessTokenTTLSeconds < 1 {
		invalidConfigField("auth_cfg.session.access_token_ttl_seconds", "must be >= 1")
		return nil, errors.New("session config is invalid")
	}
	if config.RefreshTokenTTLSeconds < config.AccessTokenTTLSeconds {
		invalidConfigField("auth_cfg.session.refresh_token_ttl_seconds", "must be >= access_token_ttl_seconds")
		return nil, errors.New("session config is invalid")
	}
	return config, nil
}
