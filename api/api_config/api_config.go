// Package api_config api/api_config/api_config.go
package api_config

import (
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
