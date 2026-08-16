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

// 鉴权类型取值；未来新增类型在此常量组扩展并在门禁分发处加分支。
//
//	jwt    —— self-issuer：本服务自签自验、持 session 库表（模板默认形态）；
//	bridge —— 身份桥接：令牌由外部 IdP 签发，本服务用 IdP 公钥本地验签，
//	          并在本地管理这批外部令牌（登出吊销——IdP 的 logout 只吊销它那侧的
//	          refresh token，access token 在本地验签下仍有效到自然过期，
//	          这一段只有本服务自己能拦）。
const (
	AuthTypeJWT    = "jwt"
	AuthTypeBridge = "bridge"
)

type AuthConfig struct {
	Enabled      *bool               `yaml:"enabled" json:"enabled" toml:"enabled" comment:"统一准入门禁开关；缺省 true（fail-closed）；false=门禁放行不提供身份，依赖身份的方法不可用"`
	AuthType     string              `yaml:"auth_type" json:"auth_type" toml:"auth_type" comment:"鉴权类型；缺省 jwt（自签发），bridge=验外部 IdP 签发的令牌；非法值使全部受保护方法拒绝"`
	Email        *EmailConfig        `yaml:"email" json:"email" toml:"email" comment:"Email delivery configuration"`
	Verification *VerificationConfig `yaml:"verification" json:"verification" toml:"verification" comment:"Verification code policy"`
	Session      *SessionConfig      `yaml:"session" json:"session" toml:"session" comment:"JWT and refresh session policy"`
	Bridge       *BridgeConfig       `yaml:"bridge" json:"bridge" toml:"bridge" comment:"bridge 形态配置（auth_type=bridge 时必填）"`
}

type BridgeConfig struct {
	Issuer                  string   `yaml:"issuer" json:"issuer" toml:"issuer" comment:"IdP 标识（记录用途）"`
	AllowedAlgs             []string `yaml:"allowed_algs" json:"allowed_algs" toml:"allowed_algs" comment:"验签算法白名单；缺省 [ES256]；禁 none 与空串（fail-closed，防降级）"`
	PublicKeyPath           string   `yaml:"public_key_path" json:"public_key_path" toml:"public_key_path" comment:"IdP 公钥 PEM 文件路径（PKIX，P-256）；bridge 形态必填"`
	LeewaySeconds           int      `yaml:"leeway_seconds" json:"leeway_seconds" toml:"leeway_seconds" comment:"时钟偏移容忍秒数；缺省 60"`
	RevocationEnabled       *bool    `yaml:"revocation_enabled" json:"revocation_enabled" toml:"revocation_enabled" comment:"本地登出吊销开关；缺省 true"`
	RevocationMaxTTLSeconds int      `yaml:"revocation_max_ttl_seconds" json:"revocation_max_ttl_seconds" toml:"revocation_max_ttl_seconds" comment:"吊销记录保留秒数，应 >= IdP access token 最长寿命；缺省 86400"`
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

// AuthGateEnabled 统一准入门禁是否启用：配置段或 enabled 键缺席一律按启用处理（fail-closed），
// 只有显式 enabled: false 才放行门禁——放行不提供身份，依赖 AuthenticatedUser 的方法照常拒绝。
func AuthGateEnabled() bool {
	if CurrentCfgAuth == nil || CurrentCfgAuth.Enabled == nil {
		return true
	}
	return *CurrentCfgAuth.Enabled
}

// CurrentAuthType 返回门禁鉴权类型：缺省 jwt；非法值返回 error，门禁按 fail-closed 拒绝全部受保护方法。
func CurrentAuthType() (string, error) {
	if CurrentCfgAuth == nil {
		return AuthTypeJWT, nil
	}
	authType := strings.ToLower(strings.TrimSpace(CurrentCfgAuth.AuthType))
	switch authType {
	case "", AuthTypeJWT:
		return AuthTypeJWT, nil
	case AuthTypeBridge:
		return AuthTypeBridge, nil
	default:
		invalidConfigField("auth_cfg.auth_type", "is not a supported auth type")
		return "", errors.New("unsupported auth type")
	}
}

// CurrentBridgeConfig 返回 bridge 形态配置（带缺省值），并做 fail-closed 校验：
// 公钥路径必填；算法白名单禁 none 与空串（防降级）。
func CurrentBridgeConfig() (*BridgeConfig, error) {
	if CurrentCfgAuth == nil || CurrentCfgAuth.Bridge == nil {
		invalidConfigField("auth_cfg.bridge", "block is not configured")
		return nil, errors.New("bridge config is not initialized")
	}
	config := *CurrentCfgAuth.Bridge
	if strings.TrimSpace(config.PublicKeyPath) == "" {
		invalidConfigField("auth_cfg.bridge.public_key_path", "is empty")
		return nil, errors.New("bridge config is invalid")
	}
	if len(config.AllowedAlgs) == 0 {
		config.AllowedAlgs = []string{"ES256"}
	}
	for _, alg := range config.AllowedAlgs {
		normalized := strings.ToLower(strings.TrimSpace(alg))
		if normalized == "" || normalized == "none" {
			invalidConfigField("auth_cfg.bridge.allowed_algs", "must not contain none or empty")
			return nil, errors.New("bridge config is invalid")
		}
	}
	if config.LeewaySeconds <= 0 {
		config.LeewaySeconds = 60
	}
	if config.RevocationMaxTTLSeconds <= 0 {
		config.RevocationMaxTTLSeconds = 86400
	}
	return &config, nil
}

// RevocationEnabled bridge 本地吊销是否启用：键缺席按启用处理（fail-closed 方向）。
func RevocationEnabled() bool {
	if CurrentCfgAuth == nil || CurrentCfgAuth.Bridge == nil ||
		CurrentCfgAuth.Bridge.RevocationEnabled == nil {
		return true
	}
	return *CurrentCfgAuth.Bridge.RevocationEnabled
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
