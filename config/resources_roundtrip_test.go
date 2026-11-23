package config

import (
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	"github.com/0xdevelop/project_template_go/common/resource"
	"github.com/0xdevelop/project_template_go/policy/policy_config"
	"os"
	"path/filepath"
	"testing"
)

func TestResourceConsentNotPersisted(t *testing.T) {
	old := GlobalConfig
	oldAuth, oldPolicy := api_auth_config.CurrentCfgAuth, policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		GlobalConfig = old
		api_auth_config.CurrentCfgAuth = oldAuth
		policy_config.CurrentCfgPolicy = oldPolicy
	})
	r := &resource.Config{Enabled: true, Files: []resource.File{{ID: "model", Path: "data/model", SHA256: "operator-must-supply"}}, Download: resource.Options{DownloadEnabled: true, Confirmed: true}}
	cfg := &FileConfig{ResourceCfg: map[string]*resource.Config{"set": r}}
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveConfig(file, cfg); err != nil {
		t.Fatal(err)
	}
	if err := LoadConfig(file); err != nil {
		t.Fatal(err)
	}
	loaded := GlobalConfig.ResourceCfg["set"]
	if loaded == nil || loaded.Download.Confirmed || !loaded.Enabled || loaded.Files[0].ID != "model" {
		t.Fatal("consent persisted or manifest lost")
	}
}

func TestExecutableRootPreservesUnconfiguredOptionalPaths(t *testing.T) {
	oldApp, oldCfg := CurrentApp, GlobalConfig
	oldAuth, oldPolicy := api_auth_config.CurrentCfgAuth, policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		CurrentApp = oldApp
		GlobalConfig = oldCfg
		api_auth_config.CurrentCfgAuth = oldAuth
		policy_config.CurrentCfgPolicy = oldPolicy
	})
	CurrentApp = &ExtendApp{}
	file := filepath.Join(t.TempDir(), "config.json")
	raw := `{"storage_cfg":{"sqlite":{"enabled":false,"db_file_path":""}}}`
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadConfig(file); err != nil {
		t.Fatalf("unused optional paths must not prevent app config loading: %v", err)
	}
	if GlobalConfig.StorageCfg != nil && GlobalConfig.StorageCfg.Sqlite != nil && GlobalConfig.StorageCfg.Sqlite.DBFilePath != "" {
		t.Fatal("empty disabled SQLite path became configured")
	}
}
