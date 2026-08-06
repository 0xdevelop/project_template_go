package api_jsonRPC_request

import (
	"encoding/json"

	"github.com/0xYeah/project_template_go/api/api_jsonRPC/api_config_jsonRPC"
	"github.com/0xYeah/project_template_go/api/api_jsonRPC/api_jsonRPC_protocol"
)

func ParserRequest(body []byte, reqModel *api_jsonRPC_protocol.RPCRequest) error {
	var tmpMap map[string]json.RawMessage
	err := json.Unmarshal(body, &tmpMap)
	if err != nil {
		return &api_jsonRPC_protocol.RPCError{
			Code:    -32700,
			Message: "parse error",
			Data:    err.Error(),
		}
	}

	if id, ok := tmpMap["id"]; ok {
		if !validRequestID(id) {
			return &api_jsonRPC_protocol.RPCError{
				Code:    -32600,
				Message: "invalid request id",
			}
		}
		reqModel.ID = append(reqModel.ID[:0], id...)
	}

	jsonrpcRaw, ok := tmpMap["jsonrpc"]
	if !ok || json.Unmarshal(jsonrpcRaw, &reqModel.JsonRPC) != nil ||
		reqModel.JsonRPC != api_config_jsonRPC.JSONRPCVersion {
		return &api_jsonRPC_protocol.RPCError{
			Code:    -32600,
			Message: "invalid or missing 'jsonrpc' field",
		}
	}

	methodRaw, ok := tmpMap["method"]
	if !ok || json.Unmarshal(methodRaw, &reqModel.Method) != nil || reqModel.Method == "" {
		return &api_jsonRPC_protocol.RPCError{
			Code:    -32600,
			Message: "invalid or missing 'method' field",
		}
	}

	if paramsRaw, ok := tmpMap["params"]; ok {
		if err = json.Unmarshal(paramsRaw, &reqModel.Params); err != nil {
			return &api_jsonRPC_protocol.RPCError{
				Code:    -32602,
				Message: "invalid params",
				Data:    err.Error(),
			}
		}
	}

	return nil
}

func validRequestID(id json.RawMessage) bool {
	if len(id) == 0 {
		return false
	}

	var value interface{}
	if err := json.Unmarshal(id, &value); err != nil {
		return false
	}

	switch value.(type) {
	case nil, string, float64:
		return true
	default:
		return false
	}
}
