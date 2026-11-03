package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFrontendConfigurationLoading(t *testing.T) {
	previous := GlobalConfig
	t.Cleanup(func() { GlobalConfig = previous })
	for _, testCase := range []struct {
		name, content string
		enabled       bool
		invalid       bool
	}{
		{name: "enabled.yaml", content: "frontend_cfg:\n  enabled: true\n  bind_address: 127.0.0.1\n  port: 13006\n", enabled: true},
		{name: "enabled.json", content: `{"frontend_cfg":{"enabled":true,"bind_address":"127.0.0.1","port":13006}}`, enabled: true},
		{name: "enabled.toml", content: "[frontend_cfg]\nenabled = true\nbind_address = \"127.0.0.1\"\nport = 13006\n", enabled: true},
		{name: "legacy.yaml", content: "{}\n"},
		{name: "disabled.yaml", content: "frontend_cfg:\n  enabled: false\n"},
		{name: "missing-address.yaml", content: "frontend_cfg:\n  enabled: true\n  port: 13006\n", invalid: true},
		{name: "missing-port.yaml", content: "frontend_cfg:\n  enabled: true\n  bind_address: 127.0.0.1\n", invalid: true},
		{name: "invalid-port.yaml", content: "frontend_cfg:\n  enabled: true\n  bind_address: 127.0.0.1\n  port: 65536\n", invalid: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			GlobalConfig = nil
			file := filepath.Join(t.TempDir(), testCase.name)
			if err := os.WriteFile(file, []byte(testCase.content), 0600); err != nil {
				t.Fatal(err)
			}
			err := LoadConfig(file)
			if (err != nil) != testCase.invalid {
				t.Fatalf("invalid=%v: %v", testCase.invalid, err)
			}
			if testCase.enabled && (GlobalConfig.FrontendCfg == nil || !GlobalConfig.FrontendCfg.Enabled ||
				GlobalConfig.FrontendCfg.BindAddress != "127.0.0.1" || GlobalConfig.FrontendCfg.Port != 13006) {
				t.Fatalf("frontend configuration was not loaded: %+v", GlobalConfig.FrontendCfg)
			}
			if !testCase.enabled && !testCase.invalid && GlobalConfig.FrontendCfg != nil && GlobalConfig.FrontendCfg.Enabled {
				t.Fatal("legacy or disabled configuration enabled the frontend")
			}
		})
	}
}
