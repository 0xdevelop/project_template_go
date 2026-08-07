package api_auth_session

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/0xYeah/project_template_go/ability/ability_user"
	"github.com/0xYeah/project_template_go/ability/ability_user/ability_user_model"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_config"
	authModel "github.com/0xYeah/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xYeah/project_template_go/api/api_error_code"
	"github.com/0xYeah/project_template_go/api/api_supported_methods"
	appdb "github.com/0xYeah/project_template_go/db"
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
		InputSchema: api_auth_common.TokenInputSchema("jwt_token"),
		Execute:     AuthLogout,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodJWTTokenCheck,
		Description: "检查 JWT token 并返回当前身份",
		InputSchema: api_auth_common.TokenInputSchema("jwt_token"),
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
	if !ok || !api_auth_common.HasOnlyKeys(params, "jwt_token") {
		return nil, api_error_code.ErrInvalidArguments
	}
	user, session, err := AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	db, err := appdb.MysqlDB(ctx)
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
	if !ok || !api_auth_common.HasOnlyKeys(params, "jwt_token") {
		return nil, api_error_code.ErrInvalidArguments
	}
	user, session, err := AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"user":               user,
		"session_expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
	}, nil
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
	db, err := appdb.MysqlDB(ctx)
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
	db, err := appdb.MysqlDB(ctx)
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

type authenticatedIdentity struct {
	user    *ability_user_model.User
	session *authModel.AuthSession
}

type authenticatedIdentityContextKey struct{}

// AuthenticateRequest 是 APIExecuter 统一准入门禁入口：验证 arguments.jwt_token 并把身份写入 context。
// 非 Public 方法在 Execute 前必经此函数；失败统一返回业务错误，不区分 token 缺失与无效。
func AuthenticateRequest(ctx context.Context, abilityParams interface{}) (context.Context, error) {
	params, ok := abilityParams.(map[string]interface{})
	if !ok {
		return ctx, api_error_code.ErrInvalidArguments
	}
	tokenValue, _ := params["jwt_token"].(string)
	tokenValue = strings.TrimSpace(tokenValue)
	if tokenValue == "" || len(tokenValue) > 8192 {
		return ctx, api_error_code.ErrPermissionDenied
	}
	user, session, err := authenticateAccessToken(ctx, tokenValue)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(
		ctx,
		authenticatedIdentityContextKey{},
		&authenticatedIdentity{user: user, session: session},
	), nil
}

// AuthenticatedUser 供业务方法读取准入门禁下传的当前用户与会话；非 Public 方法执行时必有。
func AuthenticatedUser(ctx context.Context) (*ability_user_model.User, *authModel.AuthSession, error) {
	identity, ok := ctx.Value(authenticatedIdentityContextKey{}).(*authenticatedIdentity)
	if !ok || identity == nil {
		return nil, nil, api_error_code.ErrPermissionDenied
	}
	return identity.user, identity.session, nil
}

// authenticateAccessToken 校验 JWT、session 与用户状态，返回当前用户与会话；只由准入门禁调用。
func authenticateAccessToken(ctx context.Context, tokenValue string) (*ability_user_model.User, *authModel.AuthSession, error) {
	claims, err := parseAccessToken(tokenValue)
	if err != nil {
		return nil, nil, err
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, nil, err
	}

	session := &authModel.AuthSession{}
	err = db.Where("session_id = ? AND user_id = ?", claims.SessionID, claims.UserID).
		Take(session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, api_error_code.ErrPermissionDenied
	}
	if err != nil {
		return nil, nil, err
	}
	if session.RevokedAt != nil {
		return nil, nil, api_error_code.ErrPermissionDenied
	}
	if !time.Now().UTC().Before(session.ExpiresAt) {
		return nil, nil, api_error_code.ErrPermissionDenied
	}
	user, err := ability_user.FindByUserID(db, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, api_error_code.ErrPermissionDenied
		}
		return nil, nil, err
	}
	return user, session, nil
}
