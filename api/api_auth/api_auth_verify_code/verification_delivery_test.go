package api_auth_verify_code

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	authModel "github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_register"
	"github.com/0xdevelop/project_template_go/api/api_config"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_handler"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/resend/resend-go/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func boolPointer(v bool) *bool { return &v }
func fixtureMailConfig() *api_auth_config.EmailConfig {
	return &api_auth_config.EmailConfig{Enabled: boolPointer(true), Provider: api_auth_config.EmailProviderResend, APIKey: "fixture-only-not-a-credential", From: "sender@example.invalid", ProductName: "fixture", VerificationSubject: "fixture verification"}
}

type deliveryFixture struct {
	db            *gorm.DB
	logs          []string
	providerCalls int
	providerCode  string
	providerError error
	onProvider    func()
}

func newDeliveryFixture(t *testing.T, emailConfig *api_auth_config.EmailConfig) *deliveryFixture {
	t.Helper()
	f := &deliveryFixture{}
	previousConfig, previousAuth := config.GlobalConfig, api_auth_config.CurrentCfgAuth
	previousLogger, previousSender := verificationCodeInfof, sendEmailRequest
	t.Cleanup(func() {
		storage.CloseStorage()
		config.GlobalConfig = previousConfig
		api_auth_config.CurrentCfgAuth = previousAuth
		verificationCodeInfof = previousLogger
		sendEmailRequest = previousSender
	})
	interval := 60
	api_auth_config.CurrentCfgAuth = &api_auth_config.AuthConfig{Email: emailConfig, Verification: &api_auth_config.VerificationConfig{CodeTTLSeconds: 600, MaxAttempts: 3, SendIntervalSeconds: &interval, HourlySendLimit: 5, CodeHashSecret: "fixture-only-32-character-secret-hash-key"}}
	config.GlobalConfig = &config.FileConfig{StorageCfg: storage_config.Default(filepath.Join(t.TempDir(), "verification.db"))}
	if storage.OpenStorage() != nil || storage.GormAutoMigrate() != nil {
		t.Fatal("SQLite fixture setup failed")
	}
	var err error
	f.db, err = storage.GormDB(context.Background())
	if err != nil {
		t.Fatal("SQLite unavailable")
	}
	f.db.Logger = logger.Default.LogMode(logger.Silent)
	verificationCodeInfof = func(format string, args ...interface{}) { f.logs = append(f.logs, fmt.Sprintf(format, args...)) }
	sendEmailRequest = func(_ context.Context, _ string, request *resend.SendEmailRequest) error {
		f.providerCalls++
		if len(f.logs) != 1 {
			t.Fatal("generated code was not INFO-logged before provider invocation")
		}
		match := regexp.MustCompile(`<strong>([0-9]{6})</strong>`).FindStringSubmatch(request.Html)
		if len(match) != 2 {
			t.Fatal("mock provider payload has no real code")
		}
		f.providerCode = match[1]
		if f.onProvider != nil {
			f.onProvider()
		}
		return f.providerError
	}
	return f
}
func (f *deliveryFixture) loggedCode(t *testing.T) string {
	t.Helper()
	if len(f.logs) != 1 {
		t.Fatal("expected exactly one generated-code INFO record")
	}
	match := regexp.MustCompile(`\bcode=([0-9]{6})\b`).FindStringSubmatch(f.logs[0])
	if len(match) != 2 {
		t.Fatal("INFO record lacks generated code")
	}
	if !strings.Contains(f.logs[0], "delivery_id=") || !strings.Contains(f.logs[0], "validity=pending") {
		t.Fatal("missing non-identity correlation/validity warning")
	}
	if strings.Contains(f.logs[0], "@") || strings.Contains(f.logs[0], "fixture-only") {
		t.Fatal("INFO record contains recipient or secret material")
	}
	return match[1]
}
func (f *deliveryFixture) row(t *testing.T, email string) *authModel.AuthVerifyCode {
	t.Helper()
	r := &authModel.AuthVerifyCode{}
	if f.db.Where("recipient = ?", email).Take(r).Error != nil {
		t.Fatal("verification row missing")
	}
	return r
}
func requestFixtureCode(ctx context.Context, email string) (interface{}, error) {
	return VerificationCodeEmailSend(ctx, map[string]interface{}{"email": email})
}
func checkFixtureCode(ctx context.Context, email, code string) (interface{}, error) {
	return VerificationCodeEmailCheck(ctx, map[string]interface{}{"email": email, "verify_code": code})
}

func TestGeneratedCodeAlwaysInfoLoggedAndEmailRemainsIndependent(t *testing.T) {
	cases := []struct {
		name          string
		mail          func() *api_auth_config.EmailConfig
		providerFails bool
		wantCalls     int
		wantSuccess   bool
	}{
		{"no_email_configuration", func() *api_auth_config.EmailConfig { return nil }, false, 0, true},
		{"explicit_false_empty_dependencies", func() *api_auth_config.EmailConfig { return &api_auth_config.EmailConfig{Enabled: boolPointer(false)} }, false, 0, true},
		{"explicit_false_complete_dependencies", func() *api_auth_config.EmailConfig {
			c := fixtureMailConfig()
			c.Enabled = boolPointer(false)
			return c
		}, false, 0, true},
		{"email_success", fixtureMailConfig, false, 1, true},
		{"email_failure", fixtureMailConfig, true, 1, false},
		{"email_incomplete", func() *api_auth_config.EmailConfig { c := fixtureMailConfig(); c.APIKey = ""; return c }, false, 0, false},
		{"email_unsupported_provider", func() *api_auth_config.EmailConfig { c := fixtureMailConfig(); c.Provider = "unsupported"; return c }, false, 0, false},
		{"legacy_email_block_still_sends", func() *api_auth_config.EmailConfig { c := fixtureMailConfig(); c.Enabled = nil; return c }, false, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDeliveryFixture(t, tc.mail())
			if tc.providerFails {
				f.providerError = errors.New("fixture provider failure must not leak recipient or body")
			}
			const email = "recipient@example.invalid"
			response, err := requestFixtureCode(context.Background(), email)
			code := f.loggedCode(t)
			if f.providerCalls != tc.wantCalls {
				t.Fatal("mail enablement/dependency boundary mismatch")
			}
			if f.providerCalls > 0 && f.providerCode != code {
				t.Fatal("logged code differs from actual provider payload")
			}
			row := f.row(t, email)
			if row.SendCount != 1 || row.AttemptsRemaining != 3 {
				t.Fatal("generation quota/attempt contract changed")
			}
			if tc.wantSuccess {
				if err != nil || row.SentAt == nil {
					t.Fatal("selected workflow did not activate generated code")
				}
				expected := api_auth_common.VerificationCodeHash(api_auth_config.CurrentCfgAuth.Verification.CodeHashSecret, email, code)
				if row.CodeHash != expected || row.CodeHash == code {
					t.Fatal("stored hash does not match INFO code")
				}
				result, ok := response.(map[string]interface{})
				if !ok || result["sent"] != (tc.wantCalls == 1) || result["code_logged"] != true {
					t.Fatal("API does not distinguish logging from mail acknowledgement")
				}
				if _, ok = result["verify_code"]; ok {
					t.Fatal("API exposed a verification code field")
				}
				if _, ok = result["code"]; ok {
					t.Fatal("API exposed a code field")
				}
				for _, value := range result {
					if text, ok := value.(string); ok && text == code {
						t.Fatal("API exposed generated code")
					}
				}
				if _, err := checkFixtureCode(context.Background(), email, code); err != nil {
					t.Fatal("INFO code did not verify against SQLite")
				}
			} else {
				if !errors.Is(err, api_error_code.ErrVerifyCodeDeliveryFailed) {
					t.Fatal("email failure was silently converted to success")
				}
				if data, ok := response.(map[string]interface{}); ok && len(data) != 0 {
					t.Fatal("failed email returned success payload")
				}
				if row.SentAt != nil || row.CodeHash != "" {
					t.Fatal("failed email code was activated or not invalidated")
				}
				if _, err := checkFixtureCode(context.Background(), email, code); err == nil {
					t.Fatal("logged failed-delivery code was usable")
				}
			}
		})
	}
}

func TestSQLiteRequestInfoCaptureVerifyRegisterAndRejectDuplicate(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	const email = "roundtrip@example.invalid"
	if _, err := requestFixtureCode(context.Background(), email); err != nil {
		t.Fatal("request failed")
	}
	code := f.loggedCode(t)
	if _, err := checkFixtureCode(context.Background(), email, code); err != nil {
		t.Fatal("check failed")
	}
	input := map[string]interface{}{"user_name": "fixture_user", "email": email, "password": "fixture password never printed", "verify_code": code}
	if _, err := api_auth_register.AuthRegister(context.Background(), input); err != nil {
		t.Fatal("registration failed")
	}
	row := f.row(t, email)
	if row.SentAt != nil || row.CodeHash != "" {
		t.Fatal("registration did not consume code")
	}
	if _, err := api_auth_register.AuthRegister(context.Background(), input); err == nil {
		t.Fatal("duplicate registration reused consumed code")
	}
	if _, err := checkFixtureCode(context.Background(), email, code); err == nil {
		t.Fatal("consumed code remained valid")
	}
	if len(f.logs) != 1 {
		t.Fatal("verification/consumption invented a new code log")
	}
}
func TestRejectedRequestsDoNotInventCodeLogs(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		f := newDeliveryFixture(t, nil)
		for _, input := range []interface{}{nil, map[string]interface{}{}, map[string]interface{}{"email": "invalid"}, map[string]interface{}{"email": "a<>@example.invalid"}, map[string]interface{}{"email": "fixture@example.invalid", "unknown": true}} {
			if _, err := VerificationCodeEmailSend(context.Background(), input); err == nil {
				t.Fatal("bad input accepted")
			}
		}
		if len(f.logs) != 0 || f.providerCalls != 0 {
			t.Fatal("rejected request generated/logged a code")
		}
	})
	for _, name := range []string{"cooldown", "hourly", "failed_email_cooldown"} {
		t.Run(name, func(t *testing.T) {
			var mail *api_auth_config.EmailConfig
			if name == "failed_email_cooldown" {
				mail = fixtureMailConfig()
			}
			f := newDeliveryFixture(t, mail)
			if mail != nil {
				f.providerError = errors.New("fixture failure")
			}
			const email = "limited@example.invalid"
			_, _ = requestFixtureCode(context.Background(), email)
			_ = f.loggedCode(t)
			if name == "hourly" {
				if f.db.Model(&authModel.AuthVerifyCode{}).Where("recipient = ?", email).Updates(map[string]interface{}{"resend_available_at": time.Now().UTC().Add(-time.Second), "send_count": 5}).Error != nil {
					t.Fatal("rate-limit fixture setup")
				}
			}
			calls := f.providerCalls
			if _, err := requestFixtureCode(context.Background(), email); err == nil {
				t.Fatal("rate limit bypassed")
			}
			if len(f.logs) != 1 || f.providerCalls != calls {
				t.Fatal("rate limited request invented code or mail send")
			}
		})
	}
}
func TestGeneratedCodeLoggedEvenWhenStorageTransactionFails(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	name := "fixture:reject_verification_insert"
	if f.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "AuthVerifyCode" {
			tx.AddError(errors.New("fixture insert failure"))
		}
	}) != nil {
		t.Fatal("register storage failure fixture")
	}
	t.Cleanup(func() { f.db.Callback().Create().Remove(name) })
	if _, err := requestFixtureCode(context.Background(), "rollback@example.invalid"); err == nil {
		t.Fatal("storage failure became success")
	}
	_ = f.loggedCode(t)
	var count int64
	if f.db.Model(&authModel.AuthVerifyCode{}).Count(&count).Error != nil || count != 0 {
		t.Fatal("failed generation transaction was committed")
	}
}
func TestPendingCodeCannotBeConsumedDuringProviderCall(t *testing.T) {
	f := newDeliveryFixture(t, fixtureMailConfig())
	const email = "pending@example.invalid"
	f.onProvider = func() {
		if _, err := checkFixtureCode(context.Background(), email, f.providerCode); err == nil {
			t.Fatal("pending unactivated code was consumable")
		}
	}
	if _, err := requestFixtureCode(context.Background(), email); err != nil {
		t.Fatal("mock email failed")
	}
	code := f.loggedCode(t)
	if _, err := checkFixtureCode(context.Background(), email, code); err != nil {
		t.Fatal("acknowledged code did not activate")
	}
}
func TestWrongCodeAttemptsAndExpiryStillApply(t *testing.T) {
	for _, name := range []string{"attempts", "expiry"} {
		t.Run(name, func(t *testing.T) {
			f := newDeliveryFixture(t, nil)
			const email = "security@example.invalid"
			if _, err := requestFixtureCode(context.Background(), email); err != nil {
				t.Fatal("request failed")
			}
			code := f.loggedCode(t)
			if name == "expiry" {
				if f.db.Model(&authModel.AuthVerifyCode{}).Where("recipient = ?", email).Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error != nil {
					t.Fatal("fixture expiry update")
				}
			} else {
				wrong := code[:5] + string('0'+(code[5]-'0'+1)%10)
				for i := 0; i < 3; i++ {
					if _, err := checkFixtureCode(context.Background(), email, wrong); err == nil {
						t.Fatal("wrong code accepted")
					}
				}
				if f.row(t, email).AttemptsRemaining != 0 {
					t.Fatal("attempt budget not exhausted")
				}
			}
			if _, err := checkFixtureCode(context.Background(), email, code); err == nil {
				t.Fatal("expired/exhausted code accepted")
			}
			if len(f.logs) != 1 {
				t.Fatal("check invented a code log")
			}
		})
	}
}
func TestFinalProviderBoundaryRejectsDisabledAndInvalidPayloads(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	if _, err := requestFixtureCode(context.Background(), "boundary@example.invalid"); err != nil {
		t.Fatal("boundary fixture generation")
	}
	code := f.loggedCode(t)

	for _, tc := range []struct {
		name            string
		cfg             *api_auth_config.EmailConfig
		recipient, code string
		ttl             int
	}{
		{"nil", nil, "fixture@example.invalid", "", 600},
		{"disabled", func() *api_auth_config.EmailConfig {
			c := fixtureMailConfig()
			c.Enabled = boolPointer(false)
			return c
		}(), "fixture@example.invalid", "", 600},
		{"invalid_code", fixtureMailConfig(), "fixture@example.invalid", "invalid", 600},
		{"invalid_ttl", fixtureMailConfig(), "fixture@example.invalid", code, 0},
		{"invalid_recipient", fixtureMailConfig(), "a<>@example.invalid", code, 600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if deliverVerificationEmail(context.Background(), tc.cfg, tc.recipient, tc.code, tc.ttl) == nil {
				t.Fatal("invalid final call accepted")
			}
		})
	}
	if f.providerCalls != 0 {
		t.Fatal("invalid final call reached provider")
	}
}

// This is real loopback HTTP through the production JSON-RPC Handler and API executor,
// not a replacement handler. Only the external email provider and INFO sink are local seams.
func TestIsolatedHTTPVerificationWorkflows(t *testing.T) {
	for _, name := range []string{"no_email", "email_success", "email_failure"} {
		t.Run(name, func(t *testing.T) {
			var mailConfig *api_auth_config.EmailConfig
			if name != "no_email" {
				mailConfig = fixtureMailConfig()
			}
			f := newDeliveryFixture(t, mailConfig)
			if name == "email_failure" {
				f.providerError = errors.New("fixture provider failure")
			}
			previousMethods := api_supported_methods.Methods()
			previousAPI := api_config.CurrentApiCfg
			t.Cleanup(func() {
				api_supported_methods.SupportedMethodsSetup()
				for i := range previousMethods {
					method := previousMethods[i]
					api_supported_methods.AddMethod(&method)
				}
				api_config.CurrentApiCfg = previousAPI
			})
			api_config.CurrentApiCfg = &api_config.ApiConfig{NeedEncryption: false}
			api_supported_methods.SupportedMethodsSetup()
			LoadAPIMethods()
			api_auth_register.LoadAPIMethods()
			// Explicitly synchronize captured test evidence across the HTTP server/client goroutines.
			var state sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				state.Lock()
				defer state.Unlock()
				api_jsonRPC_handler.APIJsonRPCHandler(w, r)
			}))
			t.Cleanup(server.Close)
			client := server.Client()
			client.Timeout = 5 * time.Second
			type responseData struct {
				Sent       *bool           `json:"sent"`
				CodeLogged bool            `json:"code_logged"`
				DeliveryID string          `json:"delivery_id"`
				ErrorCode  int             `json:"error_code"`
				Verified   bool            `json:"verified"`
				User       json.RawMessage `json:"user"`
			}
			invoke := func(method string, args map[string]interface{}) (bool, responseData) {
				t.Helper()
				body, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]interface{}{"name": method, "arguments": args}})
				if err != nil {
					t.Fatal("encode HTTP request")
				}
				response, err := client.Post(server.URL, "application/json", bytes.NewReader(body))
				if err != nil {
					t.Fatal("isolated HTTP call failed")
				}
				defer response.Body.Close()
				var wire struct {
					Result struct {
						IsError bool `json:"isError"`
						Content []struct {
							Text string `json:"text"`
						} `json:"content"`
					} `json:"result"`
					Error json.RawMessage `json:"error"`
				}
				if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&wire) != nil || len(wire.Error) > 0 || len(wire.Result.Content) != 1 {
					t.Fatal("unexpected HTTP/RPC envelope")
				}
				var data responseData
				if json.Unmarshal([]byte(wire.Result.Content[0].Text), &data) != nil {
					t.Fatal("invalid typed business response")
				}
				var fields map[string]json.RawMessage
				if json.Unmarshal([]byte(wire.Result.Content[0].Text), &fields) != nil {
					t.Fatal("invalid response data")
				}
				if fields["code"] != nil || fields["verify_code"] != nil {
					t.Fatal("HTTP response exposed code")
				}
				return wire.Result.IsError, data
			}
			const email = "http-roundtrip@example.invalid"
			failed, sendResult := invoke(MethodVerificationCodeSendEmail, map[string]interface{}{"email": email})
			state.Lock()
			code := f.loggedCode(t)
			providerCalls := f.providerCalls
			state.Unlock()
			if name == "email_failure" {
				if !failed || sendResult.ErrorCode != api_error_code.ErrVerifyCodeDeliveryFailed.Code || providerCalls != 1 {
					t.Fatal("HTTP email failure was not explicit")
				}
				if failed, _ := invoke(MethodVerificationCodeCheckEmail, map[string]interface{}{"email": email, "verify_code": code}); !failed {
					t.Fatal("HTTP accepted failed-email code")
				}
			} else {
				if failed || sendResult.Sent == nil || *sendResult.Sent != (name == "email_success") || !sendResult.CodeLogged || sendResult.DeliveryID == "" {
					t.Fatal("HTTP logging/mail response semantics incorrect")
				}
				if failed, _ := invoke(MethodVerificationCodeCheckEmail, map[string]interface{}{"email": email, "verify_code": code}); failed {
					t.Fatal("HTTP could not verify INFO code")
				}
				input := map[string]interface{}{"user_name": "http_fixture_user", "email": email, "password": "fixture password never printed", "verify_code": code}
				if failed, _ := invoke(api_auth_register.MethodRegister, input); failed {
					t.Fatal("HTTP registration failed")
				}
				if failed, _ := invoke(api_auth_register.MethodRegister, input); !failed {
					t.Fatal("HTTP duplicate reused consumed code")
				}
			}
			state.Lock()
			logsBefore := len(f.logs)
			state.Unlock()
			if failed, _ := invoke(MethodVerificationCodeSendEmail, map[string]interface{}{"email": email}); !failed {
				t.Fatal("HTTP frequency restriction bypassed")
			}
			if failed, _ := invoke(MethodVerificationCodeSendEmail, map[string]interface{}{"email": 42}); !failed {
				t.Fatal("HTTP coerced invalid email type")
			}
			state.Lock()
			defer state.Unlock()
			if len(f.logs) != logsBefore {
				t.Fatal("rejected HTTP request invented a code log")
			}
		})
	}
}

func TestEmailFailureCleanupFailureRemainsUnusable(t *testing.T) {
	f := newDeliveryFixture(t, fixtureMailConfig())
	f.providerError = errors.New("fixture provider failure")
	name := "fixture:reject_cleanup_update"
	if f.db.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "AuthVerifyCode" {
			tx.AddError(errors.New("fixture cleanup failure"))
		}
	}) != nil {
		t.Fatal("install failure fixture")
	}
	t.Cleanup(func() { f.db.Callback().Update().Remove(name) })
	const email = "cleanup-failed@example.invalid"
	if _, err := requestFixtureCode(context.Background(), email); !errors.Is(err, api_error_code.ErrVerifyCodeDeliveryFailed) {
		t.Fatal("provider/cleanup failure became success")
	}
	code := f.loggedCode(t)
	if f.row(t, email).SentAt != nil {
		t.Fatal("failed cleanup activated code")
	}
	if _, err := checkFixtureCode(context.Background(), email, code); err == nil {
		t.Fatal("cleanup failure made logged code usable")
	}
}
