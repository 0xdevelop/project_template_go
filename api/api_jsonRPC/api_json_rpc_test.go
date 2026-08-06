package api_jsonRPC

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xYeah/project_template_go/ability"
	"github.com/0xYeah/project_template_go/api/api_config"
	"github.com/0xYeah/project_template_go/api/api_error_code"
	"github.com/0xYeah/project_template_go/api/api_jsonRPC/api_jsonRPC_handler"
	"github.com/0xYeah/project_template_go/api/api_jsonRPC/api_jsonRPC_protocol"
	"github.com/0xYeah/project_template_go/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestJSONRPCRegisteredMethod(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	api_config.CurrentApiCfg = &api_config.ApiConfig{}
	requestBody := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "test",
			"arguments": {}
		}
	}`)
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(requestBody))
	request.Header.Set("User-Agent", "test-client/1.0")
	response := httptest.NewRecorder()

	api_jsonRPC_handler.APIJsonRPCHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}

	var rpcResponse struct {
		Result *mcp.CallToolResult            `json:"result"`
		Error  *api_jsonRPC_protocol.RPCError `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rpcResponse); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rpcResponse.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", rpcResponse.Error)
	}
	if rpcResponse.Result == nil {
		t.Fatalf("missing result: %#v", rpcResponse)
	}
	contentText := resultText(t, rpcResponse.Result)
	if rpcResponse.Result.IsError || rpcResponse.Result.StructuredContent != nil ||
		contentText != "this is test method, request is success" {
		t.Fatalf("unexpected result: %#v", rpcResponse.Result)
	}
	var wireResponse struct {
		Result struct {
			Meta       map[string]interface{} `json:"_meta"`
			IsError    *bool                  `json:"isError"`
			ResultType string                 `json:"resultType"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &wireResponse); err != nil {
		t.Fatalf("decode response wire fields: %v", err)
	}
	serverInfo, ok := wireResponse.Result.Meta[mcp.MetaKeyServerInfo].(map[string]interface{})
	if !ok || wireResponse.Result.IsError == nil ||
		*wireResponse.Result.IsError ||
		wireResponse.Result.ResultType != "complete" ||
		serverInfo["name"] != config.ProjectName ||
		serverInfo["version"] != config.ProjectVersion {
		t.Fatalf("unexpected response wire fields: %#v", wireResponse.Result)
	}
}

func TestJSONRPCBusinessErrorUsesNormalResult(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	api_config.CurrentApiCfg = &api_config.ApiConfig{}
	requestBody := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "no.such.method",
			"arguments": {
				"phone": "+8613800000000"
			}
		}
	}`)
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(requestBody))
	request.Header.Set("User-Agent", "test-client/1.0")
	response := httptest.NewRecorder()

	api_jsonRPC_handler.APIJsonRPCHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	var rpcResponse struct {
		Result *mcp.CallToolResult            `json:"result"`
		Error  *api_jsonRPC_protocol.RPCError `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rpcResponse); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rpcResponse.Result == nil {
		t.Fatalf("missing result: %#v", rpcResponse)
	}
	errorContent := decodeErrorContent(t, rpcResponse.Result)
	if rpcResponse.Error != nil || rpcResponse.Result.StructuredContent != nil ||
		!rpcResponse.Result.IsError ||
		errorContent["error_code"] != float64(api_error_code.MethodNotFound) ||
		errorContent["error_msg"] != "method not found" {
		t.Fatalf("unexpected response: %#v", rpcResponse)
	}
}

func TestJSONRPCRejectsDirectBusinessMethod(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	api_config.CurrentApiCfg = &api_config.ApiConfig{}
	requestBody := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "test",
		"params": {}
	}`)
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(requestBody))
	request.Header.Set("User-Agent", "test-client/1.0")
	response := httptest.NewRecorder()

	api_jsonRPC_handler.APIJsonRPCHandler(response, request)

	var rpcResponse struct {
		Result *mcp.CallToolResult            `json:"result"`
		Error  *api_jsonRPC_protocol.RPCError `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rpcResponse); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rpcResponse.Result != nil || rpcResponse.Error == nil ||
		rpcResponse.Error.Code != -32602 {
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

func decodeErrorContent(t *testing.T, result *mcp.CallToolResult) map[string]interface{} {
	t.Helper()
	errorContent := make(map[string]interface{})
	if err := json.Unmarshal([]byte(resultText(t, result)), &errorContent); err != nil {
		t.Fatalf("decode error content: %v", err)
	}
	return errorContent
}
