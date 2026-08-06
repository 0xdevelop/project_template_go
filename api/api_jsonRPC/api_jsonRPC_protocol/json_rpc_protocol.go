package api_jsonRPC_protocol

import "encoding/json"

// RPCRequest JSON-RPC 请求和响应结构
type RPCRequest struct {
	Method  string          `json:"method"`
	Params  interface{}     `json:"params"`
	JsonRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
}

func (r *RPCRequest) IsNotification() bool {
	return len(r.ID) == 0
}

func (r *RPCRequest) IDString() string {
	if len(r.ID) == 0 {
		return ""
	}

	var idString string
	if err := json.Unmarshal(r.ID, &idString); err == nil {
		return idString
	}
	return string(r.ID)
}

type RPCResponse struct {
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	JsonRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
}

type RPCError struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return e.Message
}
