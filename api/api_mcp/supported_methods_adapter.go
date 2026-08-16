package api_mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/0xdevelop/project_template_go/api/api_executer"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpMaxRequestBodySize = 4 << 20
)

func newMCPHTTPHandler() http.Handler {
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return newMCPServer()
		},
		&mcp.StreamableHTTPOptions{
			Stateless:           true,
			JSONResponse:        true,
			MaxRequestBodyBytes: mcpMaxRequestBodySize,
		},
	)
	crossOriginProtection := http.NewCrossOriginProtection()
	// 不按 Mcp-Protocol-Version 请求头筛请求：按 MCP 规范，该头是握手协商完成
	// 之后的请求才携带，initialize 请求不带；在这里做写死版本号的等值判断会把
	// 握手本身挡在外面，标准客户端第一步就收到非 MCP 响应（官方 TS SDK 表现为
	// `Unexpected content type: null`）。版本协商是 SDK 的职责；非 MCP 的误访问
	// 由路由 GET 分支的 HomeHandler 接住（路由已限定 `POST /{$}` 才进本 handler）。
	return crossOriginProtection.Handler(handler)
}

func newMCPServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    config.ProjectName,
			Version: config.ProjectVersion,
		},
		&mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{
				Tools: &mcp.ToolCapabilities{
					ListChanged: false,
				},
			},
			GetSessionID: func() string {
				return ""
			},
		},
	)
	server.AddReceivingMiddleware(explicitIsErrorMiddleware)

	for _, method := range api_supported_methods.Methods() {
		method := method
		server.AddTool(
			&mcp.Tool{
				Name:        method.Name,
				Description: method.Description,
				InputSchema: method.InputSchema,
			},
			func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return executeSupportedMethod(ctx, request, method.Name)
			},
		)
	}
	return server
}

func executeSupportedMethod(ctx context.Context, request *mcp.CallToolRequest, methodName string) (*mcp.CallToolResult, error) {
	var arguments interface{} = map[string]interface{}{}
	if request != nil && request.Params != nil &&
		len(request.Params.Arguments) > 0 {
		if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInvalidParams,
				Message: "tool arguments must be valid JSON",
			}
		}
	}

	userAgent := mcpRequestUserAgent(request)
	callParams := map[string]interface{}{
		"name":      methodName,
		"arguments": arguments,
	}
	encryptionKey := userAgent + "/"
	result, err := api_executer.APIExecuter(
		ctx,
		api_executer.ToolsCallMethod,
		callParams,
		encryptionKey,
	)
	if err != nil {
		if !errors.Is(err, api_executer.ErrInvalidCall) {
			return nil, err
		}
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: err.Error(),
		}
	}
	return result.CallToolResult, nil
}

type explicitIsErrorResult struct {
	mcp.ResultBase
	result *mcp.CallToolResult
}

func (result *explicitIsErrorResult) MarshalJSON() ([]byte, error) {
	return api_executer.MarshalCallToolResult(result.result, result.GetMeta())
}

func explicitIsErrorMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, request)
		if err != nil || method != api_executer.ToolsCallMethod {
			return result, err
		}
		toolResult, ok := result.(*mcp.CallToolResult)
		if !ok {
			return result, nil
		}
		return &explicitIsErrorResult{
			ResultBase: mcp.ResultBase{Meta: toolResult.Meta},
			result:     toolResult,
		}, nil
	}
}

func mcpRequestUserAgent(request *mcp.CallToolRequest) string {
	if request != nil && request.Extra != nil {
		return request.Extra.Header.Get("User-Agent")
	}
	return ""
}
