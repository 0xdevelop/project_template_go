package api_grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/0xYeah/project_template_go/api/api_executer"
	"github.com/0xYeah/project_template_go/api/api_grpc/api_config_grpc"
	api_grpc_protobuf "github.com/0xYeah/project_template_go/api/api_grpc/protobuf"
	"github.com/george012/gtbox/gtbox_log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	grpcMaxReceiveMessageSize = 4 << 20
	grpcMaxSendMessageSize    = 8 << 20
)

var grpcServer *grpc.Server

type apiServer struct {
	api_grpc_protobuf.UnimplementedAPIServiceServer
}

func (server *apiServer) Call(ctx context.Context, request *api_grpc_protobuf.CallRequest) (*api_grpc_protobuf.CallResponse, error) {
	if request == nil || request.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}

	var params interface{}
	if request.GetParams() != nil {
		params = request.GetParams().AsMap()
	}
	encryptionKey := fmt.Sprintf(
		"%s/%s",
		grpcUserAgent(ctx),
		request.GetRequestId(),
	)
	result, err := api_executer.APIExecuter(
		ctx,
		request.GetMethod(),
		params,
		encryptionKey,
	)
	if err != nil {
		if errors.Is(err, api_executer.ErrInvalidCall) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "API execution failed")
	}
	resultStruct, err := callToolResultStruct(result)
	if err != nil {
		return nil, status.Error(codes.Internal, "API result encode failed")
	}
	return &api_grpc_protobuf.CallResponse{
		RequestId: request.GetRequestId(),
		Result:    resultStruct,
	}, nil
}

func callToolResultStruct(result *api_executer.CallToolResult) (*structpb.Struct, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	value := make(map[string]interface{})
	if err = json.Unmarshal(encoded, &value); err != nil {
		return nil, err
	}
	return structpb.NewStruct(value)
}

func grpcUserAgent(ctx context.Context) string {
	incomingMetadata, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	userAgents := incomingMetadata.Get("user-agent")
	if len(userAgents) == 0 {
		return ""
	}
	return userAgents[0]
}

func newGRPCServer() *grpc.Server {
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(grpcMaxReceiveMessageSize),
		grpc.MaxSendMsgSize(grpcMaxSendMessageSize),
	)
	api_grpc_protobuf.RegisterAPIServiceServer(server, &apiServer{})
	return server
}

func StartAPIServiceWithGRPC(apiCfgGRPC *api_config_grpc.APIConfigGRPC) {
	if apiCfgGRPC == nil {
		gtbox_log.LogErrorf("gRPC API config is nil")
		return
	}
	api_config_grpc.CurrentAPICfgGRPC = apiCfgGRPC
	if apiCfgGRPC.Port < 1 || apiCfgGRPC.Port > 65535 {
		gtbox_log.LogErrorf("gRPC API port must be between 1 and 65535")
		return
	}

	addr := fmt.Sprintf("0.0.0.0:%d", apiCfgGRPC.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		gtbox_log.LogErrorf("Failed to start gRPC server: %v", err)
		return
	}

	server := newGRPCServer()
	grpcServer = server
	go func() {
		gtbox_log.LogInfof("gRPC server Run On  [tcp://127.0.0.1:%d]", apiCfgGRPC.Port)
		if serveErr := server.Serve(listener); serveErr != nil &&
			!errors.Is(serveErr, grpc.ErrServerStopped) {
			gtbox_log.LogErrorf(
				"gRPC server stopped unexpectedly: %v",
				serveErr,
			)
		}
	}()
}

func StopApiServiceWithGRPC() {
	server := grpcServer
	if server == nil {
		gtbox_log.LogInfof("gRPC server is not running")
		return
	}

	gtbox_log.LogInfof("Shutting down gRPC server...")
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		gtbox_log.LogInfof("gRPC server stopped successfully")
	case <-time.After(5 * time.Second):
		server.Stop()
		gtbox_log.LogErrorf("gRPC server forced to stop after timeout")
	}
	grpcServer = nil
}
