package api_mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/ability"
	"github.com/0xdevelop/project_template_go/api/api_config"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// expectedNegotiatedProtocolVersion 是当前 SDK 版本双方协商出的协议版本，
// SDK 升级时同步更新；服务端不以它做请求头等值判断。
const expectedNegotiatedProtocolVersion = "2026-07-28"

func TestMCPUsesSupportedMethodsThroughLatestOfficialSDK(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	previousAPICfg := api_config.CurrentApiCfg
	api_config.CurrentApiCfg = &api_config.ApiConfig{}
	t.Cleanup(func() {
		api_config.CurrentApiCfg = previousAPICfg
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := newMCPServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect official MCP server: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "template-test", Version: "1.0.0"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect official MCP client: %v", err)
	}
	defer session.Close()
	initializeResult := session.InitializeResult()
	// 协议版本由 SDK 双方协商得出，服务端不做写死版本号的请求头等值判断；
	// 此处只断言当前 SDK 版本协商出的结果，SDK 升级时同步更新期望值。
	if initializeResult.ProtocolVersion != expectedNegotiatedProtocolVersion {
		t.Fatalf(
			"protocol version = %q, want %q",
			initializeResult.ProtocolVersion,
			expectedNegotiatedProtocolVersion,
		)
	}
	if initializeResult.Capabilities == nil ||
		initializeResult.Capabilities.Tools == nil ||
		initializeResult.Capabilities.Logging != nil {
		t.Fatalf(
			"unexpected MCP capabilities: %#v",
			initializeResult.Capabilities,
		)
	}

	listResult, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	expectedTools := make(map[string]struct{})
	for _, method := range api_supported_methods.Methods() {
		expectedTools[method.Name] = struct{}{}
	}
	for _, tool := range listResult.Tools {
		delete(expectedTools, tool.Name)
		// MCP 的门禁凭证走 HTTP Authorization 头，工具入参不暴露 jwt_token。
		schema, _ := tool.InputSchema.(map[string]interface{})
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			if _, leaked := properties[api_supported_methods.GateTokenArgument]; leaked {
				t.Fatalf("tool %s exposes %s in its MCP schema", tool.Name, api_supported_methods.GateTokenArgument)
			}
		}
	}
	if len(expectedTools) != 0 {
		t.Fatalf("missing tools: %#v", expectedTools)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "test",
		Arguments: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	contentText := resultText(t, result)
	if result.IsError || result.StructuredContent != nil ||
		contentText != "this is test method, request is success" {
		t.Fatalf("unexpected result: %#v", result)
	}

	// MCP 由官方 SDK 原生管理 tools 目录：调用未注册 tool 属协议错误，
	// 不进入统一执行链；业务错误码路径由其余三个协议 Adapter 的测试覆盖。
	if _, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "no.such.method",
		Arguments: map[string]interface{}{},
	}); err == nil {
		t.Fatalf("expected protocol error for unknown tool")
	}
}

func TestMCPStatelessHTTPUsesProtocol20260728(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	const requestID = "mcp-protocol-request-id"
	body := `{
		"jsonrpc": "2.0",
		"id": "` + requestID + `",
		"method": "tools/call",
		"params": {
			"name": "test",
			"arguments": {},
			"_meta": {
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientInfo": {
					"name": "template-test",
					"version": "1.0.0"
				},
				"io.modelcontextprotocol/clientCapabilities": {
					"extensions": {}
				}
			}
		}
	}`
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	// 握手之后形态的 stateless 直调：SDK 要求 _meta 带 protocolVersion 的请求
	// 必须同时带 Mcp-Protocol-Version 头（握手后的请求按规范携带该头）。
	request.Header.Set("Mcp-Protocol-Version", expectedNegotiatedProtocolVersion)
	request.Header.Set("Mcp-Method", "tools/call")
	request.Header.Set("Mcp-Name", "test")

	response := httptest.NewRecorder()
	newMCPHTTPHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected MCP status %d: %s", response.Code, response.Body.String())
	}
	var rpcResponse struct {
		ID     string `json:"id"`
		Result struct {
			Meta       map[string]interface{} `json:"_meta"`
			IsError    *bool                  `json:"isError"`
			ResultType string                 `json:"resultType"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rpcResponse); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	serverInfo, ok := rpcResponse.Result.Meta[mcp.MetaKeyServerInfo].(map[string]interface{})
	if rpcResponse.ID != requestID || !ok || rpcResponse.Result.IsError == nil ||
		*rpcResponse.Result.IsError ||
		rpcResponse.Result.ResultType != "complete" ||
		serverInfo["name"] != config.ProjectName ||
		serverInfo["version"] != config.ProjectVersion {
		t.Fatalf("unexpected response: %#v", rpcResponse)
	}
}

// TestMCPInitializeWithoutProtocolVersionHeader 是握手回归防线：按 MCP 规范，
// initialize 请求不携带 Mcp-Protocol-Version 头（该头是协商完成之后的请求才带），
// handler 必须照常完成握手。曾有按该头做写死版本号等值判断的实现把握手本身
// 挡在外面，标准客户端第一步就收到非 MCP 响应（官方 TS SDK 报
// `Unexpected content type: null`）。
func TestMCPInitializeWithoutProtocolVersionHeader(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	const requestID = "mcp-initialize-request-id"
	body := `{
		"jsonrpc": "2.0",
		"id": "` + requestID + `",
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-11-25",
			"capabilities": {},
			"clientInfo": {"name": "template-test", "version": "1.0.0"}
		}
	}`
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")

	response := httptest.NewRecorder()
	newMCPHTTPHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected MCP status %d: %s", response.Code, response.Body.String())
	}
	var rpcResponse struct {
		ID     string `json:"id"`
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rpcResponse); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if rpcResponse.ID != requestID || rpcResponse.Result.ProtocolVersion == "" {
		t.Fatalf("unexpected initialize response: %s", response.Body.String())
	}
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("unexpected content: %#v", result)
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type: %#v", result.Content[0])
	}
	return textContent.Text
}
