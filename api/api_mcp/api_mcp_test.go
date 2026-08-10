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
	if initializeResult.ProtocolVersion != mcpProtocolVersion {
		t.Fatalf(
			"protocol version = %q, want %q",
			initializeResult.ProtocolVersion,
			mcpProtocolVersion,
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
	request.Header.Set("Mcp-Protocol-Version", mcpProtocolVersion)
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
