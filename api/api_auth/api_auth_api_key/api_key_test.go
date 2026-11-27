package api_auth_api_key

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/project_template_go/ability/ability_user"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	authModel "github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_session"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_executer"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
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
	ctx, err := api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), "Bearer "+jwt), map[string]interface{}{},
	)
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
	if !strings.HasPrefix(plaintext, api_auth_session.APIKeyPrefix) || record.UserID != user.UserID || record.KeyPrefix == "" {
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
	if _, err = api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), "Bearer "+plaintext),
		map[string]interface{}{"jwt_token": "invalid-token"},
	); !errors.Is(err, api_error_code.ErrPermissionDenied) {
		t.Fatalf("invalid explicit credential must not fall back to a valid header: %v", err)
	}
	if _, err = api_auth_session.AuthenticateRequest(
		api_auth_session.WithBearerToken(context.Background(), "Bearer invalid-token"),
		map[string]interface{}{"jwt_token": plaintext},
	); err != nil {
		t.Fatalf("explicit credential must take precedence over the header: %v", err)
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

func TestAPIKeyMethodAuthorizationThroughExecuter(t *testing.T) {
	setupAuthTestDB(t)
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	user, err := ability_user.Create(db, "api_key_gate_user", "gate@example.com", testPassword, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	jwtCtx := sessionContext(t, "gate@example.com")
	issued, err := AuthAPIKeyIssue(jwtCtx, map[string]interface{}{"name": "gate"})
	if err != nil {
		t.Fatal(err)
	}
	key := issued.(map[string]interface{})["api_key"].(string)
	keyCtx := api_auth_session.WithBearerToken(context.Background(), "Bearer "+key)
	invalidCtx := api_auth_session.WithBearerToken(context.Background(), "Bearer key_invalid")
	api_supported_methods.SupportedMethodsSetup()
	t.Cleanup(api_supported_methods.SupportedMethodsSetup)
	executions := 0
	for _, method := range []*api_supported_methods.SupportedMethod{
		{Name: "api_key_gate.public", Public: true, APIKeyAllowed: false},
		{Name: "api_key_gate.public_allowed", Public: true, APIKeyAllowed: true},
		{Name: "api_key_gate.jwt"},
		{Name: "api_key_gate.allowed", Public: false, APIKeyAllowed: true},
		{Name: "api_key_gate.async_denied", Public: false, APIKeyAllowed: false, Async: true},
		{Name: "api_key_gate.async_allowed", Public: false, APIKeyAllowed: true, Async: true},
	} {
		method.InputSchema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		method.Execute = func(ctx context.Context, input interface{}) (interface{}, error) {
			executions++
			if !method.Public {
				if _, exists := input.(map[string]interface{})["jwt_token"]; exists {
					return nil, errors.New("protected Execute received a credential")
				}
				caller, callerErr := api_auth_session.AuthenticatedCaller(ctx)
				if callerErr != nil || caller.UserID != user.UserID {
					return nil, errors.New("protected Execute did not receive the authenticated user")
				}
			}
			return "executed", nil
		}
		api_supported_methods.AddMethod(method)
	}
	for _, test := range []struct {
		name    string
		ctx     context.Context
		method  string
		allowed bool
	}{
		{"public ignores invalid key", invalidCtx, "api_key_gate.public", true},
		{"public ignores APIKeyAllowed", invalidCtx, "api_key_gate.public_allowed", true},
		{"public needs no credential", context.Background(), "api_key_gate.public", true},
		{"JWT accepts default false", jwtCtx, "api_key_gate.jwt", true},
		{"JWT accepts true", jwtCtx, "api_key_gate.allowed", true},
		{"key rejects default false", keyCtx, "api_key_gate.jwt", false},
		{"key accepts true", keyCtx, "api_key_gate.allowed", true},
		{"true still verifies key", invalidCtx, "api_key_gate.allowed", false},
		{"true still requires credential", context.Background(), "api_key_gate.allowed", false},
		{"async denied before acceptance", keyCtx, "api_key_gate.async_denied", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := executions
			result, callErr := api_executer.APIExecuter(test.ctx, "tools/call", map[string]interface{}{
				"name": test.method, "arguments": map[string]interface{}{},
			}, "")
			if callErr != nil || result == nil || result.IsError == test.allowed {
				t.Fatalf("allowed=%v result=%v err=%v", test.allowed, result, callErr)
			}
			if !test.allowed && executions != before {
				t.Fatal("denied call reached Execute")
			}
			if test.allowed && executions != before+1 {
				t.Fatal("allowed synchronous call did not execute once")
			}
		})
	}
	var tasks int64
	if err = db.Model(&ability_task_model.AsyncTask{}).Count(&tasks).Error; err != nil || tasks != 0 {
		t.Fatalf("denied async call created tasks: count=%d err=%v", tasks, err)
	}
	beforeAccept := executions
	result, err := api_executer.APIExecuter(context.Background(), "tools/call", map[string]interface{}{
		"name": "api_key_gate.async_allowed", "arguments": map[string]interface{}{"jwt_token": key},
	}, "")
	if err != nil || result == nil || result.IsError {
		t.Fatalf("allowed async acceptance failed: result=%v err=%v", result, err)
	}
	if executions != beforeAccept || len(result.Content) != 1 {
		t.Fatal("async acceptance executed business work or returned no receipt")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("async acceptance did not return a JSON receipt")
	}
	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err = json.Unmarshal([]byte(text.Text), &accepted); err != nil || accepted.TaskID == "" {
		t.Fatal("async acceptance returned no task_id")
	}
	var task ability_task_model.AsyncTask
	if err = db.Take(&task).Error; err != nil || task.TaskID != accepted.TaskID || task.UserID == nil || *task.UserID != user.UserID || task.InputPayload != "{}" {
		t.Fatalf("async record identity or credential stripping failed: err=%v", err)
	}
	if _, err = AuthAPIKeyRevoke(jwtCtx, map[string]interface{}{"api_key_id": issued.(map[string]interface{})["record"].(*APIKeyRecord).APIKeyID}); err != nil {
		t.Fatal(err)
	}
	result, err = api_executer.APIExecuter(keyCtx, "tools/call", map[string]interface{}{
		"name": "api_key_gate.allowed", "arguments": map[string]interface{}{},
	}, "")
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("revoked key reached an allowed method: err=%v", err)
	}
	disabled := false
	api_auth_config.CurrentCfgAuth.Enabled = &disabled
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name: "api_key_gate.disabled", Public: false, APIKeyAllowed: false,
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Execute: func(ctx context.Context, input interface{}) (interface{}, error) {
			_, callerErr := api_auth_session.AuthenticatedCaller(ctx)
			_, hasToken := input.(map[string]interface{})["jwt_token"]
			if callerErr == nil || hasToken {
				return nil, errors.New("disabled gate fabricated identity or retained credential")
			}
			return "executed", nil
		},
	})
	result, err = api_executer.APIExecuter(context.Background(), "tools/call", map[string]interface{}{
		"name": "api_key_gate.disabled", "arguments": map[string]interface{}{"jwt_token": "invalid"},
	}, "")
	if err != nil || result == nil || result.IsError {
		t.Fatalf("disabled gate failed: err=%v", err)
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

func TestAPIKeyNameAndTimestampContract(t *testing.T) {
	setupAuthTestDB(t)
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ability_user.Create(db, "name_user", "name@example.com", testPassword, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	ctx := sessionContext(t, "name@example.com")
	for _, name := range []string{"workstation", strings.Repeat("测", 64)} {
		t.Run(name, func(t *testing.T) {
			issued, issueErr := AuthAPIKeyIssue(ctx, map[string]interface{}{"name": name})
			if issueErr != nil {
				t.Fatal(issueErr)
			}
			encoded, marshalErr := json.Marshal(issued.(map[string]interface{})["record"])
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var record map[string]interface{}
			if err = json.Unmarshal(encoded, &record); err != nil {
				t.Fatal(err)
			}
			createdAt, ok := record["created_at"].(string)
			millis, parseErr := strconv.ParseInt(createdAt, 10, 64)
			if !ok || parseErr != nil || len(createdAt) != 13 || time.Since(time.UnixMilli(millis)).Abs() > time.Minute {
				t.Errorf("created_at must be a UTC millisecond string, got %v", record["created_at"])
			}
			if _, exists := record["last_used_at"]; exists {
				t.Error("unused key must omit last_used_at")
			}
			if _, exists := record["revoked_at"]; exists {
				t.Error("active key must omit revoked_at")
			}
			if _, err = api_auth_session.AuthenticateRequest(
				api_auth_session.WithBearerToken(context.Background(), "Bearer "+issued.(map[string]interface{})["api_key"].(string)),
				map[string]interface{}{},
			); err != nil {
				t.Fatal(err)
			}
			revoked, revokeErr := AuthAPIKeyRevoke(ctx, map[string]interface{}{"api_key_id": record["api_key_id"]})
			if revokeErr != nil {
				t.Fatal(revokeErr)
			}
			encoded, marshalErr = json.Marshal(revoked)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var revokedRecord map[string]string
			if err = json.Unmarshal(encoded, &revokedRecord); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"created_at", "last_used_at", "revoked_at"} {
				millis, parseErr = strconv.ParseInt(revokedRecord[field], 10, 64)
				if parseErr != nil || len(revokedRecord[field]) != 13 || time.Since(time.UnixMilli(millis)).Abs() > time.Minute {
					t.Errorf("%s must be a UTC millisecond string, got %s", field, revokedRecord[field])
				}
			}
		})
	}
	if _, err = AuthAPIKeyIssue(ctx, map[string]interface{}{"name": strings.Repeat("测", 65)}); !errors.Is(err, api_error_code.ErrInvalidArguments) {
		t.Fatalf("65 characters must be rejected: %v", err)
	}
}

func TestAPIKeyConcurrentUsageTimestamp(t *testing.T) {
	setupAuthTestDB(t)
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ability_user.Create(db, "concurrent_user", "concurrent@example.com", testPassword, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	issued, err := AuthAPIKeyIssue(sessionContext(t, "concurrent@example.com"), map[string]interface{}{"name": "concurrent"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = api_auth_session.WithBearerToken(ctx, "Bearer "+issued.(map[string]interface{})["api_key"].(string))
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	const callers = 4
	read := make(chan struct{}, callers)
	release := make(chan struct{})
	results := make(chan error, callers)
	var writes atomic.Int64
	if err = db.Callback().Query().After("gorm:query").Register("auth_api_key_test_read", func(query *gorm.DB) {
		if query.Statement.Schema != nil && query.Statement.Schema.Name == "AuthAPIKey" {
			read <- struct{}{}
			<-release
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove("auth_api_key_test_read") })
	if err = db.Callback().Update().After("gorm:update").Register("auth_api_key_test_write", func(update *gorm.DB) {
		writes.Add(update.RowsAffected)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove("auth_api_key_test_write") })
	for range callers {
		go func() {
			_, callErr := api_auth_session.AuthenticateRequest(ctx, map[string]interface{}{})
			results <- callErr
		}()
	}
	timeout := time.After(10 * time.Second)
	allRead := true
readLoop:
	for range callers {
		select {
		case <-read:
		case <-timeout:
			allRead = false
			break readLoop
		}
	}
	close(release)
	for range callers {
		if err = <-results; err != nil {
			t.Errorf("authenticate concurrent caller: %v", err)
		}
	}
	if !allRead {
		t.Fatal("concurrent callers did not all read the unused key")
	}
	if writes.Load() != 1 {
		t.Fatalf("concurrent calls must update last_used_at once, got %d", writes.Load())
	}
}
