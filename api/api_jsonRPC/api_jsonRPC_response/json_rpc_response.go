package api_jsonRPC_response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_config_jsonRPC"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_protocol"
	"github.com/george012/gtbox/gtbox_log"
)

func Encode(
	result interface{},
	responseErr error,
	requestID json.RawMessage,
) ([]byte, error) {
	response := &api_jsonRPC_protocol.RPCResponse{
		Result:  result,
		JsonRPC: api_config_jsonRPC.JSONRPCVersion,
		ID:      requestID,
	}
	if responseErr != nil {
		var rpcError *api_jsonRPC_protocol.RPCError
		if errors.As(responseErr, &rpcError) {
			response.Error = rpcError
		} else {
			response.Error = &api_jsonRPC_protocol.RPCError{
				Code:    -32603,
				Message: responseErr.Error(),
			}
		}
		response.Result = nil
	}
	return json.Marshal(response)
}

func HandleResponse(
	w http.ResponseWriter,
	err error,
	resp *api_jsonRPC_protocol.RPCResponse,
	reqCode json.RawMessage,
) {
	handleResponse(w, http.StatusOK, err, resp, reqCode)
}

func HandleServiceError(
	w http.ResponseWriter,
	reqCode json.RawMessage,
) {
	handleResponse(
		w,
		http.StatusInternalServerError,
		errors.New("internal service error"),
		nil,
		reqCode,
	)
}

func handleResponse(
	w http.ResponseWriter,
	statusCode int,
	err error,
	resp *api_jsonRPC_protocol.RPCResponse,
	reqCode json.RawMessage,
) {
	w.Header().Set("Content-Type", "application/json")

	var result interface{}
	if resp != nil {
		result = resp.Result
	}
	payload, encodeErr := Encode(result, err, reqCode)
	if encodeErr != nil {
		gtbox_log.LogErrorf("Failed to encode JSON response[%v]", http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(statusCode)
	if _, writeErr := w.Write(payload); writeErr != nil {
		gtbox_log.LogErrorf("Failed to write JSON response: %v", writeErr)
	}
}
