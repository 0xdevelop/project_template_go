package api_websocket

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/ability"
	"github.com/0xdevelop/project_template_go/api/api_config"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_protocol"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/coder/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWebSocketCarriesTheUnifiedResponse(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	previousAPICfg := api_config.CurrentApiCfg
	api_config.CurrentApiCfg = &api_config.ApiConfig{}
	t.Cleanup(func() {
		api_config.CurrentApiCfg = previousAPICfg
	})

	clientConnection, serverConnection := newWebSocketPipe(t)
	defer clientConnection.CloseNow()
	defer serverConnection.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go handleWebSocketConnection(ctx, serverConnection, "")

	payload := []byte(`{
		"jsonrpc": "2.0",
		"id": "1753421234567A9K2",
		"method": "tools/call",
		"params": {
			"name": "no.such.method",
			"arguments": {
				"phone": "+8613800000000"
			}
		}
	}`)
	if err := clientConnection.Write(
		ctx,
		websocket.MessageText,
		payload,
	); err != nil {
		t.Fatalf("write WebSocket request: %v", err)
	}

	messageType, responsePayload, err := clientConnection.Read(ctx)
	if err != nil {
		t.Fatalf("read WebSocket response: %v", err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("unexpected WebSocket message type: %v", messageType)
	}

	var envelope struct {
		Result *mcp.CallToolResult            `json:"result"`
		Error  *api_jsonRPC_protocol.RPCError `json:"error"`
	}
	if err = json.Unmarshal(responsePayload, &envelope); err != nil {
		t.Fatalf("decode WebSocket response: %v", err)
	}
	if envelope.Result == nil {
		t.Fatalf("missing result: %#v", envelope)
	}
	errorContent := decodeErrorContent(t, envelope.Result)
	if envelope.Error != nil || envelope.Result.StructuredContent != nil ||
		!envelope.Result.IsError ||
		errorContent["error_code"] != float64(api_error_code.MethodNotFound) ||
		errorContent["error_msg"] != "method not found" {
		t.Fatalf("unexpected business response: %#v", envelope)
	}
	var wireEnvelope struct {
		Result struct {
			Meta       map[string]interface{} `json:"_meta"`
			ResultType string                 `json:"resultType"`
		} `json:"result"`
	}
	if err = json.Unmarshal(responsePayload, &wireEnvelope); err != nil {
		t.Fatalf("decode WebSocket wire fields: %v", err)
	}
	serverInfo, ok := wireEnvelope.Result.Meta[mcp.MetaKeyServerInfo].(map[string]interface{})
	if !ok || wireEnvelope.Result.ResultType != "complete" ||
		serverInfo["name"] != config.ProjectName ||
		serverInfo["version"] != config.ProjectVersion {
		t.Fatalf("unexpected WebSocket wire fields: %#v", wireEnvelope.Result)
	}

	successPayload := []byte(`{
		"jsonrpc": "2.0",
		"id": "1753421234567b8l3",
		"method": "tools/call",
		"params": {
			"name": "test",
			"arguments": {}
		}
	}`)
	if err = clientConnection.Write(ctx, websocket.MessageText, successPayload); err != nil {
		t.Fatalf("write successful WebSocket request: %v", err)
	}
	_, successResponsePayload, err := clientConnection.Read(ctx)
	if err != nil {
		t.Fatalf("read successful WebSocket response: %v", err)
	}
	var successEnvelope struct {
		Result *mcp.CallToolResult `json:"result"`
	}
	var successWire struct {
		Result struct {
			IsError *bool `json:"isError"`
		} `json:"result"`
	}
	if err = json.Unmarshal(successResponsePayload, &successEnvelope); err != nil {
		t.Fatalf("decode successful WebSocket response: %v", err)
	}
	if err = json.Unmarshal(successResponsePayload, &successWire); err != nil {
		t.Fatalf("decode successful WebSocket wire response: %v", err)
	}
	if successWire.Result.IsError == nil || *successWire.Result.IsError ||
		resultText(t, successEnvelope.Result) != "this is test method, request is success" {
		t.Fatalf("unexpected successful WebSocket response: %s", successResponsePayload)
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

func newWebSocketPipe(t *testing.T) (
	clientConnection *websocket.Conn,
	serverConnection *websocket.Conn,
) {
	t.Helper()

	transport := webSocketPipeTransport{
		handler: func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			var err error
			serverConnection, err = websocket.Accept(writer, request, nil)
			if err != nil {
				t.Errorf("accept in-memory WebSocket: %v", err)
			}
		},
	}
	connection, _, err := websocket.Dial(
		context.Background(),
		"ws://example.com",
		&websocket.DialOptions{
			HTTPClient: &http.Client{Transport: transport},
		},
	)
	if err != nil {
		t.Fatalf("dial in-memory WebSocket: %v", err)
	}
	return connection, serverConnection
}

type webSocketPipeTransport struct {
	handler http.HandlerFunc
}

func (transport webSocketPipeTransport) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	clientConnection, serverConnection := net.Pipe()
	hijacker := webSocketTestHijacker{
		ResponseRecorder: httptest.NewRecorder(),
		serverConnection: serverConnection,
	}
	transport.handler.ServeHTTP(hijacker, request)

	response := hijacker.Result()
	if response.StatusCode == http.StatusSwitchingProtocols {
		response.Body = clientConnection
	}
	return response, nil
}

type webSocketTestHijacker struct {
	*httptest.ResponseRecorder
	serverConnection net.Conn
}

func (hijacker webSocketTestHijacker) Hijack() (
	net.Conn,
	*bufio.ReadWriter,
	error,
) {
	return hijacker.serverConnection, bufio.NewReadWriter(
		bufio.NewReader(hijacker.serverConnection),
		bufio.NewWriter(hijacker.serverConnection),
	), nil
}
