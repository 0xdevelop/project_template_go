package api_auth_session

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_user"
	"github.com/0xdevelop/project_template_go/ability/ability_user/ability_user_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_bridge"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	authModel "github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MethodLoginEmail      = "auth.login.email"
	MethodLoginPhone      = "auth.login.phone"
	MethodLogout          = "auth.logout"
	MethodJWTTokenCheck   = "auth.jwt_token.check"
	MethodJWTTokenRefresh = "auth.jwt_token.refresh"
	accessTokenType       = "access"
	loginMethodPassword   = "password"
	loginMethodVerifyCode = "verify_code"
)

func LoadAPIMethods() {
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodLoginEmail,
		Description: "使用邮箱登录",
		Public:      true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"login_method": api_auth_common.EnumStringSchema("password"),
				"email":        api_auth_common.StringSchema("email", 3, 320),
				"password":     api_auth_common.StringSchema("", 1, 128),
			},
			"login_method",
			"email",
			"password",
		),
		Execute: AuthLoginEmail,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodLoginPhone,
		Description: "使用手机号登录",
		Public:      true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"login_method": api_auth_common.EnumStringSchema("password", "verify_code"),
				"phone":        api_auth_common.PhoneSchema(),
				"password":     api_auth_common.StringSchema("", 1, 128),
				"verify_code":  api_auth_common.VerificationCodeSchema(),
			},
			"login_method",
			"phone",
		),
		Execute: AuthLoginPhone,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodLogout,
		Description: "撤销当前登录状态",
		InputSchema: api_auth_common.InputSchema(map[string]interface{}{}),
		Execute:     AuthLogout,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodJWTTokenCheck,
		Description: "检查 JWT token 并返回当前身份",
		InputSchema: api_auth_common.InputSchema(map[string]interface{}{}),
		Execute:     AuthJWTTokenCheck,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodJWTTokenRefresh,
		Description: "轮换 refresh token 并签发新 JWT token",
		Public:      true,
		InputSchema: api_auth_common.TokenInputSchema("refresh_token"),
		Execute:     AuthJWTTokenRefresh,
	})
}

func AuthLoginEmail(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "login_method", "email", "password") {
		return nil, api_error_code.ErrInvalidArguments
	}
	loginMethod, methodOK := api_auth_common.RequiredString(params, "login_method")
	emailValue, emailOK := api_auth_common.RequiredString(params, "email")
	password, passwordOK := api_auth_common.RequiredRawString(params, "password")
	if !methodOK || !emailOK || !passwordOK || len(password) > 1024 {
		return nil, api_error_code.ErrInvalidArguments
	}
	if loginMethod != loginMethodPassword {
		return nil, api_error_code.ErrMethodNotSupported
	}
	email, err := api_auth_common.NormalizeEmail(emailValue)
	if err != nil {
		return nil, api_error_code.ErrPermissionDenied
	}
	user, err := ability_user.AuthenticateEmail(ctx, email, password)
	if errors.Is(err, ability_user.ErrInvalidCredentials) {
		return nil, api_error_code.ErrPermissionDenied
	}
	if err != nil {
		api_auth_common.LogInternalFailure("authenticate user by email")
		return nil, err
	}
	tokens, err := createSession(ctx, user)
	if err != nil {
		api_auth_common.LogInternalFailure("create auth session")
		return nil, err
	}
	tokens["user"] = user
	return tokens, nil
}

func AuthLoginPhone(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok {
		return nil, api_error_code.ErrInvalidArguments
	}
	loginMethod, methodOK := api_auth_common.RequiredString(params, "login_method")
	if !methodOK {
		return nil, api_error_code.ErrInvalidArguments
	}
	if loginMethod == loginMethodVerifyCode {
		return nil, api_error_code.ErrMethodNotSupported
	}
	if loginMethod != loginMethodPassword {
		return nil, api_error_code.ErrMethodNotSupported
	}
	if !api_auth_common.HasOnlyKeys(params, "login_method", "phone", "password") {
		return nil, api_error_code.ErrInvalidArguments
	}
	phoneValue, phoneOK := api_auth_common.RequiredString(params, "phone")
	password, passwordOK := api_auth_common.RequiredRawString(params, "password")
	if !phoneOK || !passwordOK || len(password) > 1024 {
		return nil, api_error_code.ErrInvalidArguments
	}
	phone, err := api_auth_common.NormalizePhone(phoneValue)
	if err != nil {
		return nil, api_error_code.ErrPermissionDenied
	}
	user, err := ability_user.AuthenticatePhone(ctx, phone, password)
	if errors.Is(err, ability_user.ErrInvalidCredentials) {
		return nil, api_error_code.ErrPermissionDenied
	}
	if err != nil {
		api_auth_common.LogInternalFailure("authenticate user by phone")
		return nil, err
	}
	tokens, err := createSession(ctx, user)
	if err != nil {
		api_auth_common.LogInternalFailure("create auth session")
		return nil, err
	}
	tokens["user"] = user
	return tokens, nil
}

func AuthLogout(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params) {
		return nil, api_error_code.ErrInvalidArguments
	}
	identity, err := identityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if callerFromIdentity(identity).AuthSource != AuthSourceSession {
		// API key 没有 session 可撤销；吊销 key 走 auth.api_key.revoke。
		return nil, api_error_code.ErrPermissionDenied
	}
	user := identity.user
	session := identity.session
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result := db.Model(&authModel.AuthSession{}).
		Where(
			"session_id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?",
			session.SessionID,
			user.UserID,
			now,
		).
		Update("revoked_at", now)
	if result.Error != nil {
		api_auth_common.LogInternalFailure("revoke auth session")
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, api_error_code.ErrPermissionDenied
	}
	return map[string]interface{}{"logged_out": true}, nil
}

func AuthJWTTokenCheck(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params) {
		return nil, api_error_code.ErrInvalidArguments
	}
	identity, err := identityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	caller := callerFromIdentity(identity)
	result := map[string]interface{}{
		"user":        identity.user,
		"auth_source": caller.AuthSource,
	}
	switch caller.AuthSource {
	case AuthSourceSession:
		result["session_expires_at"] = identity.session.ExpiresAt.UTC().Format(time.RFC3339)
	case AuthSourceAPIKey:
		result["api_key_id"] = caller.APIKeyID
	}
	return result, nil
}

func AuthJWTTokenRefresh(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "refresh_token") {
		return nil, api_error_code.ErrInvalidArguments
	}
	token, ok := api_auth_common.RequiredString(params, "refresh_token")
	if !ok || len(token) > 8192 {
		return nil, api_error_code.ErrInvalidArguments
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || len(parts[0]) != 36 || parts[1] == "" {
		return nil, api_error_code.ErrPermissionDenied
	}
	config, err := api_auth_config.CurrentSessionConfig()
	if err != nil {
		return nil, err
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	var outcomeErr error
	err = db.Transaction(func(tx *gorm.DB) error {
		session := &authModel.AuthSession{}
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_id = ?", parts[0]).
			Take(session).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			outcomeErr = api_error_code.ErrPermissionDenied
			return nil
		}
		if findErr != nil {
			return findErr
		}
		now := time.Now().UTC()
		if session.RevokedAt != nil ||
			!now.Before(session.ExpiresAt) ||
			!api_auth_common.SecureStringEqual(session.RefreshTokenHash, api_auth_common.TokenHash(token)) {
			outcomeErr = api_error_code.ErrPermissionDenied
			return nil
		}

		user, userErr := ability_user.FindByUserID(tx, session.UserID)
		if userErr != nil {
			if errors.Is(userErr, gorm.ErrRecordNotFound) {
				outcomeErr = api_error_code.ErrPermissionDenied
				return nil
			}
			return userErr
		}
		newSecret, tokenErr := api_auth_common.NewRandomToken()
		if tokenErr != nil {
			return tokenErr
		}
		newRefreshToken := session.SessionID + "." + newSecret
		accessToken, tokenErr := issueAccessToken(
			config,
			user.UserID,
			session.SessionID,
			now,
		)
		if tokenErr != nil {
			return tokenErr
		}
		if updateErr := tx.Model(session).Update(
			"refresh_token_hash",
			api_auth_common.TokenHash(newRefreshToken),
		).Error; updateErr != nil {
			return updateErr
		}
		result = tokenResult(config, accessToken, newRefreshToken)
		return nil
	})
	if err != nil {
		api_auth_common.LogInternalFailure("refresh session transaction")
		return nil, err
	}
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	return result, nil
}

type accessTokenClaims struct {
	SessionID string `json:"sid"`
	UserID    string `json:"uid"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

func createSession(ctx context.Context, user *ability_user_model.User) (map[string]interface{}, error) {
	config, err := api_auth_config.CurrentSessionConfig()
	if err != nil {
		return nil, err
	}
	sessionID := uuid.NewString()
	refreshSecret, err := api_auth_common.NewRandomToken()
	if err != nil {
		return nil, err
	}
	refreshToken := sessionID + "." + refreshSecret
	now := time.Now().UTC()
	accessToken, err := issueAccessToken(config, user.UserID, sessionID, now)
	if err != nil {
		return nil, err
	}
	session := &authModel.AuthSession{
		SessionID:        sessionID,
		UserID:           user.UserID,
		RefreshTokenHash: api_auth_common.TokenHash(refreshToken),
		ExpiresAt: now.Add(
			time.Duration(config.RefreshTokenTTLSeconds) * time.Second,
		),
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	if err = db.Create(session).Error; err != nil {
		return nil, err
	}
	return tokenResult(config, accessToken, refreshToken), nil
}

func issueAccessToken(config *api_auth_config.SessionConfig, userID string, sessionID string, now time.Time) (string, error) {
	accessTokenID := uuid.NewString()
	claims := accessTokenClaims{
		SessionID: sessionID,
		UserID:    userID,
		TokenType: accessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   config.Issuer,
			Audience: jwt.ClaimStrings{config.Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(
				time.Duration(config.AccessTokenTTLSeconds) * time.Second,
			)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        accessTokenID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTSigningSecret))
}

func tokenResult(
	config *api_auth_config.SessionConfig,
	accessToken string,
	refreshToken string,
) map[string]interface{} {
	return map[string]interface{}{
		"jwt_token":     accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    config.AccessTokenTTLSeconds,
	}
}

func parseAccessToken(tokenValue string) (*accessTokenClaims, error) {
	config, err := api_auth_config.CurrentSessionConfig()
	if err != nil {
		return nil, err
	}
	claims := &accessTokenClaims{}
	token, err := jwt.ParseWithClaims(
		tokenValue,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, api_error_code.ErrPermissionDenied
			}
			return []byte(config.JWTSigningSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(config.Issuer),
		jwt.WithAudience(config.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if errors.Is(err, jwt.ErrTokenExpired) {
		return nil, api_error_code.ErrPermissionDenied
	}
	if err != nil || !token.Valid ||
		claims.TokenType != accessTokenType ||
		claims.UserID == "" ||
		claims.SessionID == "" {
		return nil, api_error_code.ErrPermissionDenied
	}
	return claims, nil
}

// APIKeyPrefix 是用户级 API key 明文前缀；门禁据此区分 key 与 JWT。
const APIKeyPrefix = "key_"

// 调用方凭证来源。
const (
	AuthSourceSession = "session"
	AuthSourceAPIKey  = "api_key"
)

// apiKeyLastUsedWriteInterval 限制 last_used_at 的回写频率。
const apiKeyLastUsedWriteInterval = time.Minute

type authenticatedIdentity struct {
	user    *ability_user_model.User
	session *authModel.AuthSession
	apiKey  *authModel.AuthAPIKey
}

type authenticatedIdentityContextKey struct{}

type bearerTokenContextKey struct{}

// WithBearerToken 由协议 Adapter 调用：把 HTTP Authorization 头的值写入 context，
// 供门禁在 arguments.jwt_token 缺席时取用。接受 "Bearer xxx" 或裸 token；空值不写入。
func WithBearerToken(ctx context.Context, authorizationHeader string) context.Context {
	token := strings.TrimSpace(authorizationHeader)
	if len(token) >= 7 && strings.EqualFold(token[:7], "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, bearerTokenContextKey{}, token)
}

// requestCredential 取本次调用的门禁凭证：显式 arguments.jwt_token 优先，否则用 Adapter 写入的 Bearer 头。
func requestCredential(ctx context.Context, params map[string]interface{}) string {
	tokenValue, _ := params["jwt_token"].(string)
	tokenValue = strings.TrimSpace(tokenValue)
	if tokenValue == "" {
		tokenValue, _ = ctx.Value(bearerTokenContextKey{}).(string)
	}
	if tokenValue == "" || len(tokenValue) > 8192 {
		return ""
	}
	return tokenValue
}

// AuthCaller 是业务层看到的调用方身份。APIKeyID 只在凭证是 API key 时有值。
type AuthCaller struct {
	UserID     string
	AuthSource string
	APIKeyID   string
}

// identityFromContext 读取门禁写入的身份。没有用户，或 session 与 API key 都没有，返回权限错误。
func identityFromContext(ctx context.Context) (*authenticatedIdentity, error) {
	identity, ok := ctx.Value(authenticatedIdentityContextKey{}).(*authenticatedIdentity)
	if !ok || identity == nil || identity.user == nil || (identity.session == nil && identity.apiKey == nil) {
		return nil, api_error_code.ErrPermissionDenied
	}
	return identity, nil
}

// callerFromIdentity 从同一份身份派生调用方。有 API key 时来源是 api_key，否则是 session。
func callerFromIdentity(identity *authenticatedIdentity) *AuthCaller {
	caller := &AuthCaller{UserID: identity.user.UserID}
	if identity.apiKey != nil {
		caller.AuthSource = AuthSourceAPIKey
		caller.APIKeyID = identity.apiKey.APIKeyID
		return caller
	}
	caller.AuthSource = AuthSourceSession
	return caller
}

// AuthenticatedCaller 读取门禁下传的调用方身份；非 Public 方法执行时必有。
func AuthenticatedCaller(ctx context.Context) (*AuthCaller, error) {
	identity, err := identityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return callerFromIdentity(identity), nil
}

// AuthenticateRequest 是 APIExecuter 统一准入门禁入口：按 auth_cfg.auth_type 分发鉴权类型
// （jwt：按前缀分流 API key 与自签 JWT；bridge：验外部令牌）。
// 非 Public 方法在门禁启用时 Execute 前必经此函数；失败统一返回业务错误，不区分 token 缺失与无效。
// 验证通过后 jwt_token 由 APIExecuter 从 arguments 移除，业务层零感知。
func AuthenticateRequest(ctx context.Context, abilityParams interface{}) (context.Context, error) {
	authType, err := api_auth_config.CurrentAuthType()
	if err != nil {
		// 非法 auth_type 按 fail-closed 拒绝全部受保护方法；具体原因已由配置层 warn 日志定位。
		return ctx, api_error_code.ErrPermissionDenied
	}
	switch authType {
	case api_auth_config.AuthTypeJWT:
		return authenticateRequestWithJWT(ctx, abilityParams)
	case api_auth_config.AuthTypeBridge:
		return authenticateRequestWithBridge(ctx, abilityParams)
	default:
		return ctx, api_error_code.ErrPermissionDenied
	}
}

// ── bridge 形态（auth_type=bridge）────────────────────────────────────────────
//
// 令牌由外部 IdP 签发，本服务用 IdP 公钥本地验签 + 查本地吊销状态；
// 无 session 库表、无 DB 用户行——身份即验签出的 claims。
// 业务方法在 bridge 形态下用 AuthenticatedBridgePrincipal(ctx) 读身份
// （jwt 形态用 AuthenticatedUser，两者身份形状不同：DB 用户行 vs 令牌 claims）。

var currentBridgeVerifier *api_auth_bridge.BridgeTokenVerifier

type bridgePrincipalContextKey struct{}

// InitializeBridge 按 auth_cfg.bridge 构造验签器（加载并校验 IdP 公钥）。
// 应在服务启动期调用以 fail-fast；失败时门禁保持 fail-closed 全拒。
func InitializeBridge() error {
	bridgeConfig, err := api_auth_config.CurrentBridgeConfig()
	if err != nil {
		return err
	}
	verifier, err := api_auth_bridge.NewBridgeTokenVerifier(
		bridgeConfig.PublicKeyPath,
		bridgeConfig.AllowedAlgs,
		time.Duration(bridgeConfig.LeewaySeconds)*time.Second,
	)
	if err != nil {
		return err
	}
	currentBridgeVerifier = verifier
	return nil
}

// authenticateRequestWithBridge 是 bridge 类型的门禁实现：
// 取凭证（arguments.jwt_token 或 Bearer 头）→ 验签 → 查本地吊销 → claims 身份下传 context。
// jwt_token 由 APIExecuter 在返回后统一从 arguments 移除，业务层零感知。
func authenticateRequestWithBridge(ctx context.Context, abilityParams interface{}) (context.Context, error) {
	params, ok := abilityParams.(map[string]interface{})
	if !ok {
		return ctx, api_error_code.ErrInvalidArguments
	}
	tokenValue := requestCredential(ctx, params)
	if tokenValue == "" {
		return ctx, api_error_code.ErrPermissionDenied
	}
	if currentBridgeVerifier == nil {
		return ctx, api_error_code.ErrPermissionDenied
	}
	claims, err := currentBridgeVerifier.VerifyToken(tokenValue)
	if err != nil || claims == nil || claims.UserID == "" {
		return ctx, api_error_code.ErrPermissionDenied
	}
	if api_auth_config.RevocationEnabled() {
		isRevoked, revokeErr := api_auth_bridge.Revoked(ctx, claims.UserID, claims.IssuedAt)
		if revokeErr != nil || isRevoked {
			return ctx, api_error_code.ErrPermissionDenied
		}
	}
	return context.WithValue(ctx, bridgePrincipalContextKey{}, claims), nil
}

// AuthenticatedBridgePrincipal 供业务方法读取 bridge 门禁下传的调用方身份；
// bridge 形态下非 Public 方法执行时必有。
func AuthenticatedBridgePrincipal(ctx context.Context) (*api_auth_bridge.VerifiedClaims, error) {
	claims, ok := ctx.Value(bridgePrincipalContextKey{}).(*api_auth_bridge.VerifiedClaims)
	if !ok || claims == nil {
		return nil, api_error_code.ErrPermissionDenied
	}
	return claims, nil
}

// jwtCredential 是凭证验完、查用户之前的结果。Session 与 APIKey 只有一个非空。
type jwtCredential struct {
	userID  string
	session *authModel.AuthSession
	apiKey  *authModel.AuthAPIKey
}

// authenticateRequestWithJWT 是 jwt 类型的门禁实现：取凭证，按前缀解出用户标识和 session 或 API key，再由同一处查用户并写入身份。
func authenticateRequestWithJWT(ctx context.Context, abilityParams interface{}) (context.Context, error) {
	params, ok := abilityParams.(map[string]interface{})
	if !ok {
		return ctx, api_error_code.ErrInvalidArguments
	}
	tokenValue := requestCredential(ctx, params)
	if tokenValue == "" {
		return ctx, api_error_code.ErrPermissionDenied
	}
	var credential *jwtCredential
	var err error
	if strings.HasPrefix(tokenValue, APIKeyPrefix) {
		credential, err = resolveAPIKey(ctx, tokenValue)
	} else {
		credential, err = resolveAccessToken(ctx, tokenValue)
	}
	if err != nil {
		return ctx, err
	}
	return admitJWTCredential(ctx, credential)
}

// resolveAPIKey 按 sha256 查未吊销的 API key，返回用户标识与 key 记录。不查用户。
func resolveAPIKey(ctx context.Context, keyValue string) (*jwtCredential, error) {
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	apiKey := &authModel.AuthAPIKey{}
	err = db.Where("key_hash = ?", api_auth_common.TokenHash(keyValue)).Take(apiKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, api_error_code.ErrPermissionDenied
	}
	if err != nil {
		return nil, err
	}
	if apiKey.RevokedAt != nil {
		return nil, api_error_code.ErrPermissionDenied
	}
	return &jwtCredential{userID: apiKey.UserID, apiKey: apiKey}, nil
}

// resolveAccessToken 校验 JWT 与未撤销、未过期的 session，返回用户标识与 session。不查用户。
func resolveAccessToken(ctx context.Context, tokenValue string) (*jwtCredential, error) {
	claims, err := parseAccessToken(tokenValue)
	if err != nil {
		return nil, err
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	session := &authModel.AuthSession{}
	err = db.Where("session_id = ? AND user_id = ?", claims.SessionID, claims.UserID).
		Take(session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, api_error_code.ErrPermissionDenied
	}
	if err != nil {
		return nil, err
	}
	if session.RevokedAt != nil {
		return nil, api_error_code.ErrPermissionDenied
	}
	if !time.Now().UTC().Before(session.ExpiresAt) {
		return nil, api_error_code.ErrPermissionDenied
	}
	return &jwtCredential{userID: claims.UserID, session: session}, nil
}

// admitJWTCredential 按用户标识查用户，不存在则返回权限错误，并写入同一份身份。API key 在查到用户后节流回写 last_used_at。
func admitJWTCredential(ctx context.Context, credential *jwtCredential) (context.Context, error) {
	db, err := storage.GormDB(ctx)
	if err != nil {
		return ctx, err
	}
	user, err := ability_user.FindByUserID(db, credential.userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx, api_error_code.ErrPermissionDenied
		}
		return ctx, err
	}
	if credential.apiKey != nil {
		now := time.Now().UTC()
		if credential.apiKey.LastUsedAt == nil || now.Sub(*credential.apiKey.LastUsedAt) >= apiKeyLastUsedWriteInterval {
			if updateErr := db.Model(&authModel.AuthAPIKey{}).Where("api_key_id = ?", credential.apiKey.APIKeyID).Update("last_used_at", now).Error; updateErr != nil {
				return ctx, updateErr
			}
			credential.apiKey.LastUsedAt = &now
		}
	}
	return context.WithValue(ctx, authenticatedIdentityContextKey{}, &authenticatedIdentity{
		user: user, session: credential.session, apiKey: credential.apiKey,
	}), nil
}

// AuthenticatedUser 供业务方法读取准入门禁下传的当前用户与会话；非 Public 方法执行时必有。
// 凭证为 API key 时 session 为 nil。
func AuthenticatedUser(ctx context.Context) (*ability_user_model.User, *authModel.AuthSession, error) {
	identity, err := identityFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	return identity.user, identity.session, nil
}
