// Package api api/api_services.go
package api

import (
	"github.com/0xdevelop/project_template_go/ability"
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
