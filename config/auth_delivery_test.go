package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	"github.com/goccy/go-json"
	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
)

func TestAuthDeliveryConfigFileContracts(t *testing.T) {
	prev, prevAuth := GlobalConfig, api_auth_config.CurrentCfgAuth
	t.Cleanup(func() { GlobalConfig = prev; api_auth_config.CurrentCfgAuth = prevAuth })
	cases := []struct {
		name    string
		snippet string
		valid   bool
	}{
		{"absent_email_default_no_mail", `{"verification":{"send_interval_seconds":60}}`, true},
		{"explicit_false", `{"email":{"enabled":false}}`, true},
		{"true_missing_credentials_rejected", `{"email":{"enabled":true,"provider":"resend"}}`, false},
		{"legacy_email_retains_intent", `{"email":{"provider":"resend","api_key":"fixture-only","from":"sender@example.invalid","product_name":"fixture","verification_subject":"fixture"}}`, true},
		{"old_interval", `{"verification":{"resend_interval_seconds":60}}`, true},
		{"equal_aliases", `{"verification":{"send_interval_seconds":60,"resend_interval_seconds":60}}`, true},
		{"conflicting_aliases", `{"verification":{"send_interval_seconds":60,"resend_interval_seconds":61}}`, false},
		{"vendor_named_block", `{"resend":{"api_key":"test-only"}}`, false},
		{"misspelled_vendor_block", `{"resen":{}}`, false},
		{"unknown_email_key", `{"email":{"enable":false}}`, false},
		{"numeric_switch", `{"email":{"enabled":0}}`, false},
		{"string_switch", `{"email":{"enabled":"false"}}`, false},
		{"invalid_enum_case", `{"email":{"enabled":false,"provider":"RESEND"}}`, false},
		{"numeric_provider", `{"email":{"enabled":false,"provider":5}}`, false},
		{"numeric_from", `{"email":{"enabled":false,"from":5}}`, false},
		{"null_email_block", `{"email":null}`, false},
		{"null_verification_block", `{"verification":null}`, false},
		{"disabled_malformed_from", `{"email":{"enabled":false,"from":"invalid"}}`, false},
		{"disabled_bad_header", `{"email":{"enabled":false,"verification_subject":"bad\r\nheader"}}`, false},
		{"integral_float_interval", `{"verification":{"send_interval_seconds":60.0}}`, false},
		{"fractional_interval", `{"verification":{"send_interval_seconds":1.5}}`, false},
		{"string_interval", `{"verification":{"send_interval_seconds":"60"}}`, false},
		{"zero_interval", `{"verification":{"send_interval_seconds":0}}`, false},
		{"negative_interval", `{"verification":{"send_interval_seconds":-1}}`, false},
		{"null_switch", `{"email":{"enabled":null}}`, false},
		{"discarded_mode_not_supported", `{"verification":{"delivery_mode":"log"}}`, false},
		{"null_interval", `{"verification":{"send_interval_seconds":null}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var auth map[string]interface{}
			if err := json.Unmarshal([]byte(tc.snippet), &auth); err != nil {
				t.Fatal("bad test setup")
			}
			// JSON decodes numbers as float64; serialize intended TOML integer fixtures as integers.
			if verification, ok := auth["verification"].(map[string]interface{}); ok {
				for key, value := range verification {
					if number, ok := value.(float64); ok && number == float64(int(number)) && tc.name != "integral_float_interval" {
						verification[key] = int(number)
					}
				}
			}
			for _, ext := range []string{".yaml", ".json", ".toml"} {
				// TOML has no null: null cases are independently covered in JSON/YAML.
				if ext == ".toml" && strings.Contains(tc.snippet, "null") {
					continue
				}
				base := map[string]interface{}{"auth_cfg": auth}
				var raw []byte
				var err error
				switch ext {
				case ".json":
					raw = []byte(`{"auth_cfg":` + tc.snippet + `}`)
				case ".yaml":
					raw, err = yaml.Marshal(base)
				case ".toml":
					raw, err = toml.Marshal(base)
				}
				if err != nil {
					t.Fatal("serialize fixture")
				}
				path := filepath.Join(t.TempDir(), "config"+ext)
				if err = os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal("write fixture")
				}
				before := GlobalConfig
				err = LoadConfig(path)
				if (err == nil) != tc.valid {
					t.Fatalf("file routing/key contract mismatch for %s", ext)
				}
				if err != nil && GlobalConfig != before {
					t.Fatal("failed load published partial configuration")
				}
			}
		})
	}
}
func TestOmittedEmailDoesNotInheritPreviousLoadedConfig(t *testing.T) {
	prev, prevAuth := GlobalConfig, api_auth_config.CurrentCfgAuth
	t.Cleanup(func() { GlobalConfig = prev; api_auth_config.CurrentCfgAuth = prevAuth })
	root := t.TempDir()
	one := filepath.Join(root, "one.json")
	two := filepath.Join(root, "two.json")
	os.WriteFile(one, []byte(`{"auth_cfg":{"email":{"enabled":true,"provider":"resend","api_key":"fixture-only","from":"sender@example.invalid","product_name":"fixture","verification_subject":"fixture"}}}`), 0600)
	os.WriteFile(two, []byte(`{"auth_cfg":{}}`), 0600)
	if LoadConfig(one) != nil || !api_auth_config.EmailDeliveryEnabled() {
		t.Fatal("explicit email configuration not applied")
	}
	if LoadConfig(two) != nil || api_auth_config.EmailDeliveryEnabled() {
		t.Fatal("omitted email inherited old send intent")
	}
}
