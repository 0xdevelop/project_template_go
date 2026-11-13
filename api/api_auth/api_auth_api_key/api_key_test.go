package api_auth_api_key

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_user"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	authModel "github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_session"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
)

const testPassword = "correct-horse-battery-staple"

func setupAuthTestDB(t *testing.T) {
	t.Helper()
	previousConfig := config.GlobalConfig
	previousAuth := api_auth_config.CurrentCfgAuth
	config.GlobalConfig = &config.FileConfig{StorageCfg: storage_config.Default(filepath.Join(t.TempDir(), "api_key.db"))}
	api_auth_config.CurrentCfgAuth = &api_auth_config.AuthConfig{
		Session: &api_auth_config.SessionConfig{
			JWTSigningSecret:       "test-only-signing-secret-at-least-32-bytes",
			Issuer:                 "template-test",
			Audience:               "template-test-client",
			AccessTokenTTLSeconds:  60,
			RefreshTokenTTLSeconds: 120,
		},
	}
	t.Cleanup(func() {
		_ = storage.CloseStorage()
		config.GlobalConfig = previousConfig
		api_auth_config.CurrentCfgAuth = previousAuth
	})
	if err := storage.OpenStorage(); err != nil {
		t.Fatalf("OpenStorage: %v", err)
	}
	if err := storage.GormAutoMigrate(); err != nil {
		t.Fatalf("GormAutoMigrate: %v", err)
	}
}

func sessionContext(t *testing.T, email string) context.Context {
	t.Helper()
	result, err := api_auth_session.AuthLoginEmail(context.Background(), map[string]interface{}{
		"login_method": "password", "email": email, "password": testPassword,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	jwt := result.(map[string]interface{})["jwt_token"].(string)
	ctx, err := api_auth_session.AuthenticateRequest(context.Background(), map[string]interface{}{"jwt_token": jwt})
	if err != nil {
		t.Fatalf("gate with jwt: %v", err)
	}
	return ctx
}

func TestAPIKeyLifecycleThroughTheGate(t *testing.T) {
	setupAuthTestDB(t)
	gdb, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	user, err := ability_user.Create(gdb, "key_user", "key@example.com", testPassword, time.Now().UTC())
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	sessionCtx := sessionContext(t, "key@example.com")

	issued, err := AuthAPIKeyIssue(sessionCtx, map[string]interface{}{"name": "workstation"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	plaintext := issued.(map[string]interface{})["api_key"].(string)
	if issued.(map[string]interface{})["token_type"] != "Bearer" {
		t.Fatalf("token_type = %v", issued.(map[string]interface{})["token_type"])
	}
	record := issued.(map[string]interface{})["record"].(*APIKeyRecord)
	if plaintext[:4] != api_auth_session.APIKeyPrefix || record.UserID != user.UserID || record.KeyPrefix == "" {
		t.Fatalf("unexpected issue result: %q %+v", plaintext, record)
	}

	keyCtx, err := api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), "Bearer "+plaintext),
		map[string]interface{}{},
	)
	if err != nil {
		t.Fatalf("gate with api key header: %v", err)
	}
	caller, err := api_auth_session.AuthenticatedCaller(keyCtx)
	if err != nil || caller.UserID != user.UserID || caller.AuthSource != api_auth_session.AuthSourceAPIKey || caller.APIKeyID != record.APIKeyID {
		t.Fatalf("caller = %+v err %v", caller, err)
	}
	keyUser, keySession, err := api_auth_session.AuthenticatedUser(keyCtx)
	if err != nil || keyUser.UserID != caller.UserID || keySession != nil {
		t.Fatalf("api key user = %+v session %v err %v", keyUser, keySession, err)
	}
	if _, err = api_auth_session.AuthenticateRequest(context.Background(), map[string]interface{}{"jwt_token": plaintext}); err != nil {
		t.Fatalf("gate with api key argument: %v", err)
	}
	checked, err := api_auth_session.AuthJWTTokenCheck(keyCtx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	checkedMap := checked.(map[string]interface{})
	if checkedMap["auth_source"] != api_auth_session.AuthSourceAPIKey || checkedMap["api_key_id"] != record.APIKeyID {
		t.Fatalf("check = %#v", checkedMap)
	}
	if _, exists := checkedMap["session_expires_at"]; exists {
		t.Fatal("api key check must not report a session expiry")
	}
	if _, err = api_auth_session.AuthLogout(keyCtx, map[string]interface{}{}); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("api key must not logout a session, got %v", err)
	}
	if _, err = AuthAPIKeyIssue(keyCtx, map[string]interface{}{"name": "escalate"}); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("api key must not issue keys, got %v", err)
	}
	if _, err = AuthAPIKeyRevoke(keyCtx, map[string]interface{}{"api_key_id": record.APIKeyID}); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("api key must not revoke keys, got %v", err)
	}
	if _, err = AuthAPIKeyDelete(keyCtx, map[string]interface{}{"api_key_id": record.APIKeyID}); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("api key must not delete keys, got %v", err)
	}
	listed, err := AuthAPIKeyList(keyCtx, map[string]interface{}{})
	if err != nil || len(listed.([]*APIKeyRecord)) != 1 {
		t.Fatalf("list = %#v err %v", listed, err)
	}
	if _, err = AuthAPIKeyDelete(sessionCtx, map[string]interface{}{"api_key_id": record.APIKeyID}); !errors.Is(err, api_error_code.ErrInvalidArguments) {
		t.Fatalf("active key must not be deleted, got %v", err)
	}
	if _, err = ability_user.Create(gdb, "other_user", "other@example.com", testPassword, time.Now().UTC()); err != nil {
		t.Fatalf("create other: %v", err)
	}
	if _, err = AuthAPIKeyRevoke(sessionContext(t, "other@example.com"), map[string]interface{}{"api_key_id": record.APIKeyID}); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("other user must not revoke, got %v", err)
	}
	revoked, err := AuthAPIKeyRevoke(sessionCtx, map[string]interface{}{"api_key_id": record.APIKeyID})
	if err != nil || revoked.(*APIKeyRecord).RevokedAt == nil {
		t.Fatalf("revoke = %+v err %v", revoked, err)
	}
	if _, err = api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), plaintext), map[string]interface{}{},
	); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("revoked key must be denied, got %v", err)
	}
	if _, err = api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), "Bearer "+api_auth_session.APIKeyPrefix+"unknown"), map[string]interface{}{},
	); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("unknown key must be denied, got %v", err)
	}
	if _, err = AuthAPIKeyDelete(sessionContext(t, "other@example.com"), map[string]interface{}{"api_key_id": record.APIKeyID}); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("other user must not delete, got %v", err)
	}
	deleted, err := AuthAPIKeyDelete(sessionCtx, map[string]interface{}{"api_key_id": record.APIKeyID})
	if err != nil || deleted.(*APIKeyRecord).APIKeyID != record.APIKeyID {
		t.Fatalf("delete = %+v err %v", deleted, err)
	}
	listed, err = AuthAPIKeyList(sessionCtx, map[string]interface{}{})
	if err != nil || len(listed.([]*APIKeyRecord)) != 0 {
		t.Fatalf("deleted key must leave the list: %#v err %v", listed, err)
	}
	var kept authModel.AuthAPIKey
	if err = gdb.Unscoped().Where("api_key_id = ?", record.APIKeyID).Take(&kept).Error; err != nil || kept.DeletedAt.Time.IsZero() || kept.RevokedAt == nil || kept.KeyHash == "" {
		t.Fatalf("delete must be a soft delete of the revoked row: %+v err %v", kept, err)
	}
	if _, err = api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), plaintext), map[string]interface{}{},
	); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("deleted key must be denied, got %v", err)
	}
}

func TestGateRejectsMissingUserForJWTAndAPIKey(t *testing.T) {
	setupAuthTestDB(t)
	gdb, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	user, err := ability_user.Create(gdb, "gone_user", "gone@example.com", testPassword, time.Now().UTC())
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	login, err := api_auth_session.AuthLoginEmail(context.Background(), map[string]interface{}{
		"login_method": "password", "email": "gone@example.com", "password": testPassword,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	jwt := login.(map[string]interface{})["jwt_token"].(string)
	sessionCtx, err := api_auth_session.AuthenticateRequest(context.Background(), map[string]interface{}{"jwt_token": jwt})
	if err != nil {
		t.Fatalf("gate with jwt: %v", err)
	}
	issued, err := AuthAPIKeyIssue(sessionCtx, map[string]interface{}{"name": "probe"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	plaintext := issued.(map[string]interface{})["api_key"].(string)
	if _, err = api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), "Bearer "+plaintext), map[string]interface{}{},
	); err != nil {
		t.Fatalf("gate with api key before user removal: %v", err)
	}
	if err = gdb.Delete(user).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err = api_auth_session.AuthenticateRequest(context.Background(), map[string]interface{}{"jwt_token": jwt}); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("jwt must be denied when user is gone, got %v", err)
	}
	if _, err = api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), "Bearer "+plaintext), map[string]interface{}{},
	); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("api key must be denied when user is gone, got %v", err)
	}
}
