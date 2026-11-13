// Package api_auth_api_key 签发、列出、吊销与删除用户级 API key（auth.api_key.*）。
// key 经 Authorization: Bearer 或 arguments.jwt_token 进入统一门禁。
package api_auth_api_key

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_common"
	authModel "github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_session"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	MethodAPIKeyIssue  = "auth.api_key.issue"
	MethodAPIKeyList   = "auth.api_key.list"
	MethodAPIKeyRevoke = "auth.api_key.revoke"
	MethodAPIKeyDelete = "auth.api_key.delete"

	apiKeyPrefixLength = 12
)

func LoadAPIMethods() {
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodAPIKeyIssue,
		Description: "签发当前用户的 API key，明文只返回一次；仅登录 session 可签发，key 不能再签发。",
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"name": api_auth_common.StringSchema("", 1, 64),
			},
			"name",
		),
		Execute: AuthAPIKeyIssue,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodAPIKeyList,
		Description: "列出当前用户的 API key，不含明文与哈希。",
		InputSchema: api_auth_common.InputSchema(map[string]interface{}{}),
		Execute:     AuthAPIKeyList,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodAPIKeyRevoke,
		Description: "吊销一把 API key；仅登录 session 可吊销，且只能吊销自己的 key。",
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"api_key_id": api_auth_common.StringSchema("uuid", 36, 36),
			},
			"api_key_id",
		),
		Execute: AuthAPIKeyRevoke,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodAPIKeyDelete,
		Description: "删除一把已吊销的 API key；未吊销拒绝删除。仅登录 session 可删自己的 key。",
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"api_key_id": api_auth_common.StringSchema("uuid", 36, 36),
			},
			"api_key_id",
		),
		Execute: AuthAPIKeyDelete,
	})
}

// APIKeyRecord 是对外返回的 key 视图：不含哈希。
type APIKeyRecord struct {
	APIKeyID   string     `json:"api_key_id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func recordOf(apiKey *authModel.AuthAPIKey) *APIKeyRecord {
	return &APIKeyRecord{
		APIKeyID: apiKey.APIKeyID, UserID: apiKey.UserID, Name: apiKey.Name,
		KeyPrefix: apiKey.KeyPrefix, CreatedAt: apiKey.CreatedAt.UTC(), LastUsedAt: apiKey.LastUsedAt, RevokedAt: apiKey.RevokedAt,
	}
}

// sessionCaller 只接受登录 session：签发、吊销和删除不允许用 key 换 key。
func sessionCaller(ctx context.Context) (*api_auth_session.AuthCaller, error) {
	caller, err := api_auth_session.AuthenticatedCaller(ctx)
	if err != nil {
		return nil, err
	}
	if caller.AuthSource != api_auth_session.AuthSourceSession {
		return nil, api_error_code.ErrPermissionDenied
	}
	return caller, nil
}

func AuthAPIKeyIssue(ctx context.Context, input interface{}) (interface{}, error) {
	caller, err := sessionCaller(ctx)
	if err != nil {
		return nil, err
	}
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "name") {
		return nil, api_error_code.ErrInvalidArguments
	}
	name, nameOK := api_auth_common.RequiredString(params, "name")
	if !nameOK || len(name) > 64 {
		return nil, api_error_code.ErrInvalidArguments
	}
	secret, err := api_auth_common.NewRandomToken()
	if err != nil {
		return nil, err
	}
	plaintext := api_auth_session.APIKeyPrefix + secret
	apiKey := &authModel.AuthAPIKey{
		APIKeyID: uuid.NewString(), UserID: caller.UserID, Name: name,
		KeyHash: api_auth_common.TokenHash(plaintext), KeyPrefix: plaintext[:apiKeyPrefixLength],
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	if err = db.Create(apiKey).Error; err != nil {
		api_auth_common.LogInternalFailure("issue api key")
		return nil, err
	}
	return map[string]interface{}{
		"api_key":    plaintext,
		"token_type": "Bearer",
		"record":     recordOf(apiKey),
	}, nil
}

func AuthAPIKeyList(ctx context.Context, input interface{}) (interface{}, error) {
	caller, err := api_auth_session.AuthenticatedCaller(ctx)
	if err != nil {
		return nil, err
	}
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params) {
		return nil, api_error_code.ErrInvalidArguments
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	var keys []*authModel.AuthAPIKey
	if err = db.Where("user_id = ?", caller.UserID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	records := make([]*APIKeyRecord, 0, len(keys))
	for _, apiKey := range keys {
		records = append(records, recordOf(apiKey))
	}
	return records, nil
}

func AuthAPIKeyRevoke(ctx context.Context, input interface{}) (interface{}, error) {
	apiKey, db, err := managedAPIKey(ctx, input)
	if err != nil {
		return nil, err
	}
	if apiKey.RevokedAt == nil {
		now := time.Now().UTC()
		if err = db.Model(&authModel.AuthAPIKey{}).Where("api_key_id = ?", apiKey.APIKeyID).Update("revoked_at", now).Error; err != nil {
			return nil, err
		}
		apiKey.RevokedAt = &now
	}
	return recordOf(apiKey), nil
}

func AuthAPIKeyDelete(ctx context.Context, input interface{}) (interface{}, error) {
	apiKey, db, err := managedAPIKey(ctx, input)
	if err != nil {
		return nil, err
	}
	if apiKey.RevokedAt == nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	if err = db.Delete(apiKey).Error; err != nil {
		return nil, err
	}
	return recordOf(apiKey), nil
}

// managedAPIKey 取出 session 调用者本人的一把 key。不存在与非本人都返回权限错误。
func managedAPIKey(ctx context.Context, input interface{}) (*authModel.AuthAPIKey, *gorm.DB, error) {
	caller, err := sessionCaller(ctx)
	if err != nil {
		return nil, nil, err
	}
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "api_key_id") {
		return nil, nil, api_error_code.ErrInvalidArguments
	}
	apiKeyID, idOK := api_auth_common.RequiredString(params, "api_key_id")
	if !idOK {
		return nil, nil, api_error_code.ErrInvalidArguments
	}
	if _, parseErr := uuid.Parse(strings.TrimSpace(apiKeyID)); parseErr != nil {
		return nil, nil, api_error_code.ErrInvalidArguments
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return nil, nil, err
	}
	apiKey := &authModel.AuthAPIKey{}
	err = db.Where("api_key_id = ?", apiKeyID).Take(apiKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, api_error_code.ErrPermissionDenied
	}
	if err != nil {
		return nil, nil, err
	}
	if apiKey.UserID != caller.UserID {
		return nil, nil, api_error_code.ErrPermissionDenied
	}
	return apiKey, db, nil
}
