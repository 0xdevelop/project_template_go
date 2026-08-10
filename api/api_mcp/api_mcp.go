package api_mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/0xdevelop/project_template_go/api/api_common"
	"github.com/0xdevelop/project_template_go/api/api_mcp/api_config_mcp"
	"github.com/george012/gtbox/gtbox_log"
)

var mcpHTTPServer *http.Server

func StartAPIServiceWithMCP(apiCfgMCP *api_config_mcp.APIConfigMCP) {
	if apiCfgMCP == nil {
		gtbox_log.LogErrorf("MCP API config is nil")
		return
	}
	api_config_mcp.CurrentAPICfgMCP = apiCfgMCP

	if apiCfgMCP.MCPTransportType == "" {
		apiCfgMCP.MCPTransportType = api_config_mcp.MCPTransportTypeStreamableHTTP
	}
	if apiCfgMCP.MCPTransportType != api_config_mcp.MCPTransportTypeStreamableHTTP {
		gtbox_log.LogErrorf(
			"MCP transport [%s] is not supported; use [%s]",
			apiCfgMCP.MCPTransportType,
			api_config_mcp.MCPTransportTypeStreamableHTTP,
		)
		return
	}
	if apiCfgMCP.Port < 1 || apiCfgMCP.Port > 65535 {
		gtbox_log.LogErrorf("MCP API port must be between 1 and 65535")
		return
	}

	muxRouter := http.NewServeMux()
	muxRouter.HandleFunc("GET /{$}", api_common.HomeHandler)
	muxRouter.HandleFunc("GET /robots.txt", api_common.RobotsHandler)
	muxRouter.Handle("POST /{$}", newMCPHTTPHandler())
	muxRouter.HandleFunc("/", api_common.HomeHandler)

	addr := fmt.Sprintf("127.0.0.1:%d", apiCfgMCP.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		gtbox_log.LogErrorf("Failed to start MCP server: %v", err)
		return
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           muxRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}
	mcpHTTPServer = server
	go func() {
		gtbox_log.LogInfof("MCP server Run On  [http://%s]", addr)
		if serveErr := server.Serve(listener); serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) {
			gtbox_log.LogErrorf("MCP server stopped unexpectedly: %v", serveErr)
		}
	}()
}

func StopApiServiceWithMCP() {
	if mcpHTTPServer == nil {
		gtbox_log.LogInfof("MCP server is not running")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gtbox_log.LogInfof("Shutting down MCP server...")
	if err := mcpHTTPServer.Shutdown(ctx); err != nil {
		gtbox_log.LogErrorf("Error shutting down MCP server: %v", err)
	} else {
		gtbox_log.LogInfof("MCP server stopped successfully")
	}
	mcpHTTPServer = nil
}
