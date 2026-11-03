package frontend_config

import (
	"errors"
	"strings"
)

type FrontendConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled" toml:"enabled" comment:"启用前端页面服务"`
	BindAddress string `yaml:"bind_address" json:"bind_address" toml:"bind_address" comment:"监听地址（启用时必填）"`
	Port        int    `yaml:"port" json:"port" toml:"port" comment:"监听端口（启用时必填，1-65535）"`
}

// Validate 检查已启用前端的监听地址和端口；缺少配置时不启用。
func (cfg *FrontendConfig) Validate() error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.BindAddress) == "" {
		return errors.New("frontend_cfg.bind_address is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("frontend_cfg.port must be between 1 and 65535")
	}
	return nil
}
