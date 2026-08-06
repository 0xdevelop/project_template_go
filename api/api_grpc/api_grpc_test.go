package api_grpc

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/0xYeah/project_template_go/ability"
	"github.com/0xYeah/project_template_go/api/api_config"
	"github.com/0xYeah/project_template_go/api/api_error_code"
	api_grpc_protobuf "github.com/0xYeah/project_template_go/api/api_grpc/protobuf"
	"github.com/0xYeah/project_template_go/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestGRPCCarriesTheUnifiedResponse(t *testing.T) {
	ability.LoadAbilityAPIMethods()
	previousAPICfg := api_config.CurrentApiCfg
	api_config.CurrentApiCfg = &api_config.ApiConfig{}
	t.Cleanup(func() {
		api_config.CurrentApiCfg = previousAPICfg
	})

	listener := bufconn.Listen(grpcMaxReceiveMessageSize)
	server := newGRPCServer()
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.NewClient(
		"passthrough:///template-buffer",
		grpc.WithContextDialer(func(
			context.Context,
			string,
		) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create gRPC client: %v", err)
	}
	defer connection.Close()

	params, err := structpb.NewStruct(map[string]interface{}{
		"name": "no.such.method",
		"arguments": map[string]interface{}{
			"phone": "+8613800000000",
		},
	})
	if err != nil {
		t.Fatalf("create gRPC params: %v", err)
	}
	const requestID = "1753421234567a9k2"
	response, err := api_grpc_protobuf.NewAPIServiceClient(connection).Call(
		ctx,
		&api_grpc_protobuf.CallRequest{
			RequestId: requestID,
			Method:    "tools/call",
			Params:    params,
		},
	)
	if err != nil {
		t.Fatalf("call gRPC API: %v", err)
	}
	if response.GetRequestId() != requestID || response.GetResult() == nil {
		t.Fatalf("unexpected gRPC response: %#v", response)
	}

	result := response.GetResult().AsMap()
	errorContent := decodeGRPCErrorContent(t, result)
	_, hasStructuredContent := result["structuredContent"]
	meta, metaOK := result["_meta"].(map[string]interface{})
	serverInfo, serverInfoOK := meta[mcp.MetaKeyServerInfo].(map[string]interface{})
	if hasStructuredContent || !metaOK || !serverInfoOK || result["isError"] != true ||
		result["resultType"] != "complete" ||
		serverInfo["name"] != config.ProjectName ||
		serverInfo["version"] != config.ProjectVersion ||
		errorContent["error_code"] != float64(api_error_code.MethodNotFound) ||
		errorContent["error_msg"] != "method not found" {
		t.Fatalf("unexpected business response: %#v", result)
	}

	successParams, err := structpb.NewStruct(map[string]interface{}{
		"name":      "test",
		"arguments": map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create successful gRPC params: %v", err)
	}
	successResponse, err := api_grpc_protobuf.NewAPIServiceClient(connection).Call(
		ctx,
		&api_grpc_protobuf.CallRequest{
			RequestId: "1753421234567b8l3",
			Method:    "tools/call",
			Params:    successParams,
		},
	)
	if err != nil {
		t.Fatalf("call successful gRPC API: %v", err)
	}
	successResult := successResponse.GetResult().AsMap()
	if successResult["isError"] != false ||
		grpcContentText(t, successResult) != "this is test method, request is success" {
		t.Fatalf("unexpected successful gRPC response: %#v", successResult)
	}
}

func decodeGRPCErrorContent(t *testing.T, result map[string]interface{}) map[string]interface{} {
	t.Helper()
	errorContent := make(map[string]interface{})
	if err := json.Unmarshal([]byte(grpcContentText(t, result)), &errorContent); err != nil {
		t.Fatalf("decode error content: %v", err)
	}
	return errorContent
}

func grpcContentText(t *testing.T, result map[string]interface{}) string {
	t.Helper()
	contents, ok := result["content"].([]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("unexpected content: %#v", result["content"])
	}
	content, ok := contents[0].(map[string]interface{})
	if !ok || content["type"] != "text" {
		t.Fatalf("unexpected content item: %#v", contents[0])
	}
	text, ok := content["text"].(string)
	if !ok {
		t.Fatalf("unexpected text content: %#v", content)
	}
	return text
}
