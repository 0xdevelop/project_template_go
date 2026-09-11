// Package api_config api/api_config/api_config.go
package api_config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/0xdevelop/project_template_go/api/api_grpc/api_config_grpc"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_config_jsonRPC"
	"github.com/0xdevelop/project_template_go/api/api_mcp/api_config_mcp"
	"github.com/0xdevelop/project_template_go/api/api_websocket/api_config_websocket"
)

type ApiConfig struct {
	NeedEncryption  bool                                     `yaml:"need_encryption" json:"need_encryption" toml:"need_encryption" comment:"Require arguments to be an encrypted JSON string"`
	APICfgJsonRPC   *api_config_jsonRPC.APIConfigJsonRPC     `yaml:"api_cfg_jsonRPC" json:"api_cfg_jsonRPC" toml:"api_cfg_jsonRPC" comment:"API configurations with JSON-RPC"`
	APICfgMCP       *api_config_mcp.APIConfigMCP             `yaml:"api_cfg_mcp" json:"api_cfg_mcp" toml:"api_cfg_mcp" comment:"API configurations with MCP"`
	APICfgWebSocket *api_config_websocket.APIConfigWebSocket `yaml:"api_cfg_websocket" json:"api_cfg_websocket" toml:"api_cfg_websocket" comment:"API configurations with WebSocket"`
	APICfgGRPC      *api_config_grpc.APIConfigGRPC           `yaml:"api_cfg_grpc" json:"api_cfg_grpc" toml:"api_cfg_grpc" comment:"API configurations with gRPC"`
}

var (
	CurrentApiCfg *ApiConfig
)

// Validate 在配置加载层拒绝缺失监听地址或端口非法的已启用 Adapter；返回首个错误。
func (c *ApiConfig) Validate() error {
	if c == nil {
		return errors.New("api_cfg is not configured")
	}
	check := func(section string, enabled bool, bindAddress string, port int) error {
		if !enabled {
			return nil
		}
		if strings.TrimSpace(bindAddress) == "" {
			return fmt.Errorf("api_cfg.%s.bind_address is required (127.0.0.1 for local only, 0.0.0.0 to expose)", section)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("api_cfg.%s.port must be between 1 and 65535", section)
		}
		return nil
	}
	if c.APICfgJsonRPC != nil {
		if err := check("api_cfg_jsonRPC", c.APICfgJsonRPC.Enabled, c.APICfgJsonRPC.BindAddress, c.APICfgJsonRPC.Port); err != nil {
			return err
		}
	}
	if c.APICfgMCP != nil {
		if err := check("api_cfg_mcp", c.APICfgMCP.Enabled, c.APICfgMCP.BindAddress, c.APICfgMCP.Port); err != nil {
			return err
		}
	}
	if c.APICfgWebSocket != nil {
		if err := check("api_cfg_websocket", c.APICfgWebSocket.Enabled, c.APICfgWebSocket.BindAddress, c.APICfgWebSocket.Port); err != nil {
			return err
		}
	}
	if c.APICfgGRPC != nil {
		if err := check("api_cfg_grpc", c.APICfgGRPC.Enabled, c.APICfgGRPC.BindAddress, c.APICfgGRPC.Port); err != nil {
			return err
		}
	}
	return nil
}
