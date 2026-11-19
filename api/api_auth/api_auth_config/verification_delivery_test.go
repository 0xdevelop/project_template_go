package api_auth_config

import "testing"

func boolValue(value bool) *bool { return &value }
func intValue(value int) *int    { return &value }

func TestEmailEnablementAndLegacyCompatibility(t *testing.T) {
	previous := CurrentCfgAuth
	t.Cleanup(func() { CurrentCfgAuth = previous })
	for _, tc := range []struct {
		name string
		cfg  *AuthConfig
		want bool
	}{
		{"no_auth", nil, false}, {"no_email", &AuthConfig{}, false},
		{"legacy_email_block", &AuthConfig{Email: &EmailConfig{}}, true},
		{"explicit_false_even_with_credentials", &AuthConfig{Email: &EmailConfig{Enabled: boolValue(false), APIKey: "test-only"}}, false},
		{"explicit_true_missing_credentials", &AuthConfig{Email: &EmailConfig{Enabled: boolValue(true)}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			CurrentCfgAuth = tc.cfg
			if EmailDeliveryEnabled() != tc.want {
				t.Fatal("email enablement contract changed")
			}
		})
	}
}
func TestFinalEmailDependencies(t *testing.T) {
	valid := EmailConfig{Enabled: boolValue(true), Provider: EmailProviderResend, APIKey: "test-only", From: "Fixture <sender@example.invalid>", ProductName: "fixture", VerificationSubject: "fixture"}
	if err := ValidateEmailConfig(&valid); err != nil {
		t.Fatal("valid mock configuration rejected")
	}
	cases := []struct {
		name   string
		change func(*EmailConfig)
	}{
		{"disabled", func(c *EmailConfig) { c.Enabled = boolValue(false) }},
		{"provider_empty", func(c *EmailConfig) { c.Provider = "" }},
		{"provider_case", func(c *EmailConfig) { c.Provider = "RESEND" }},
		{"provider_whitespace", func(c *EmailConfig) { c.Provider = " resend" }},
		{"key_whitespace", func(c *EmailConfig) { c.APIKey = "fixture key" }},
		{"subject_injection", func(c *EmailConfig) { c.VerificationSubject = "fixture\r\nInjected" }},
		{"from_injection", func(c *EmailConfig) { c.From = "sender@example.invalid\r\nInjected" }},
		{"provider_invalid", func(c *EmailConfig) { c.Provider = "unsupported" }},
		{"key_missing", func(c *EmailConfig) { c.APIKey = "" }},
		{"from_missing", func(c *EmailConfig) { c.From = "" }},
		{"from_invalid", func(c *EmailConfig) { c.From = "not-an-address" }},
		{"reply_invalid", func(c *EmailConfig) { c.ReplyTo = "invalid" }},
		{"product_missing", func(c *EmailConfig) { c.ProductName = "" }},
		{"subject_missing", func(c *EmailConfig) { c.VerificationSubject = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.change(&c)
			if ValidateEmailConfig(&c) == nil {
				t.Fatal("invalid selected-path dependency accepted")
			}
		})
	}
	if ValidateEmailConfig(nil) == nil {
		t.Fatal("nil email dependency accepted")
	}
}
func TestCooldownAliasContract(t *testing.T) {
	previous := CurrentCfgAuth
	t.Cleanup(func() { CurrentCfgAuth = previous })
	for _, tc := range []struct {
		name              string
		canonical, legacy *int
		valid             bool
	}{
		{"canonical", intValue(12), nil, true}, {"legacy", nil, intValue(12), true},
		{"equal", intValue(12), intValue(12), true}, {"conflict", intValue(12), intValue(13), false},
		{"explicit_zero_not_absence", intValue(0), intValue(12), false}, {"legacy_zero", nil, intValue(0), false},
		{"negative", intValue(-1), nil, false}, {"missing", nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			CurrentCfgAuth = &AuthConfig{Verification: &VerificationConfig{CodeTTLSeconds: 60, MaxAttempts: 3, SendIntervalSeconds: tc.canonical, ResendIntervalSeconds: tc.legacy, HourlySendLimit: 5, CodeHashSecret: "test-only-hash-secret-with-32-characters"}}
			cfg, err := CurrentVerificationConfig()
			if (err == nil) != tc.valid {
				t.Fatal("cooldown migration validation mismatch")
			}
			if err == nil && (cfg.SendIntervalSeconds == nil || *cfg.SendIntervalSeconds != 12) {
				t.Fatal("cooldown alias not resolved")
			}
		})
	}
}
