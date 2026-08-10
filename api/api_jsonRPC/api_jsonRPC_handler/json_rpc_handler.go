package api_jsonRPC_handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/0xdevelop/project_template_go/api/api_executer"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_protocol"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_request"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_response"
)

// APIJsonRPCHandler 处理 JSON-RPC HTTP 请求。
func APIJsonRPCHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		bodyErr := errors.New("body read error")
		api_jsonRPC_response.HandleResponse(w, bodyErr, nil, nil)
		return
	}

	request := &api_jsonRPC_protocol.RPCRequest{}
	err = api_jsonRPC_request.ParserRequest(body, request)
	if err != nil {
		api_jsonRPC_response.HandleResponse(w, err, nil, request.ID)
		return
	}

	encryptionKey := fmt.Sprintf("%s/%s", r.UserAgent(), request.IDString())
	result, err := api_executer.APIExecuter(
		r.Context(),
		request.Method,
		request.Params,
		encryptionKey,
	)
	if err != nil {
		if !errors.Is(err, api_executer.ErrInvalidCall) {
			api_jsonRPC_response.HandleServiceError(w, request.ID)
			return
		}
		api_jsonRPC_response.HandleResponse(w, &api_jsonRPC_protocol.RPCError{
			Code:    -32602,
			Message: err.Error(),
		}, nil, request.ID)
		return
	}
	if request.IsNotification() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	api_jsonRPC_response.HandleResponse(w, nil, &api_jsonRPC_protocol.RPCResponse{
		Result: result,
	}, request.ID)
}
