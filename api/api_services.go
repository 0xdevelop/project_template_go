// Package api api/api_services.go
package api

import (
	"github.com/0xdevelop/project_template_go/ability"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_bridge"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_session"
	"github.com/0xdevelop/project_template_go/api/api_config"
	"github.com/0xdevelop/project_template_go/api/api_grpc"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC"
	"github.com/0xdevelop/project_template_go/api/api_mcp"
	"github.com/0xdevelop/project_template_go/api/api_websocket"
	"github.com/george012/gtbox/gtbox_log"
)

func StartAPIServices(apiCfg *api_config.ApiConfig) {
	ability.LoadAbilityAPIMethods()
	api_config.CurrentApiCfg = apiCfg

	// bridge 形态启动期预加载 IdP 公钥（fail-fast）：失败明示告警，
	// 门禁保持 fail-closed 全拒，不静默降级。
	if authType, authTypeErr := api_auth_config.CurrentAuthType(); authTypeErr == nil &&
		authType == api_auth_config.AuthTypeBridge {
		if err := api_auth_session.InitializeBridge(); err != nil {
			gtbox_log.LogErrorf("bridge 初始化失败(auth_cfg.bridge/公钥): %v — 全部受保护方法将被拒绝", err)
		} else if api_auth_config.RevocationEnabled() && !api_auth_bridge.RevocationConfigured() {
			gtbox_log.LogWarnf("bridge 本地吊销未装配(SetupRevocation 未调用): 登出后令牌在过期前仍可用")
		}
	}

	if api_config.CurrentApiCfg.APICfgJsonRPC != nil {
		if api_config.CurrentApiCfg.APICfgJsonRPC.Enabled == true {
			api_jsonRPC.StartAPIServiceWithJsonRPC(apiCfg.APICfgJsonRPC)
		}

	} else {
		gtbox_log.LogErrorf("StartAPIServices API not setup")
	}

	if api_config.CurrentApiCfg.APICfgMCP != nil {
		if api_config.CurrentApiCfg.APICfgMCP.Enabled == true {
			api_mcp.StartAPIServiceWithMCP(apiCfg.APICfgMCP)
		}

	} else {
		gtbox_log.LogErrorf("StartAPIServices API not setup")
	}

	if api_config.CurrentApiCfg.APICfgWebSocket != nil &&
		api_config.CurrentApiCfg.APICfgWebSocket.Enabled {
		api_websocket.StartAPIServiceWithWebSocket(
			apiCfg.APICfgWebSocket,
		)
	}

	if api_config.CurrentApiCfg.APICfgGRPC != nil &&
		api_config.CurrentApiCfg.APICfgGRPC.Enabled {
		api_grpc.StartAPIServiceWithGRPC(apiCfg.APICfgGRPC)
	}
}

func StopApiServices() {
	api_grpc.StopApiServiceWithGRPC()
	api_websocket.StopApiServiceWithWebSocket()
	api_mcp.StopApiServiceWithMCP()
	api_jsonRPC.StopApiServiceWithJsonRPC()
}
