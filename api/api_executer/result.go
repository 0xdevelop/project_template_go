package api_executer

import (
	"encoding/json"

	"github.com/0xYeah/project_template_go/api/api_config"
	"github.com/0xYeah/project_template_go/api/api_error_code"
	"github.com/0xYeah/project_template_go/config"
	"github.com/george012/gtbox/gtbox_encryption"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CallToolResult struct {
	*mcp.CallToolResult
}

func (result *CallToolResult) MarshalJSON() ([]byte, error) {
	return MarshalCallToolResult(result.CallToolResult, nil)
}

func MarshalCallToolResult(result *mcp.CallToolResult, meta map[string]interface{}) ([]byte, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	wire := make(map[string]interface{})
	if err = json.Unmarshal(encoded, &wire); err != nil {
		return nil, err
	}
	wire["isError"] = result.IsError
	if meta != nil {
		wire["_meta"] = meta
	}
	return json.Marshal(wire)
}

func finish(value interface{}, resultErr error, encryptionKey string) (*CallToolResult, error) {
	errorCode := api_error_code.Success
	errorMessage := ""
	if resultErr != nil {
		businessError, ok := api_error_code.As(resultErr)
		if !ok {
			return nil, resultErr
		}
		errorCode = businessError.Code
		errorMessage = businessError.Message
	}

	content, err := resultContentText(value, errorCode, errorMessage, encryptionKey)
	if err != nil {
		return nil, err
	}

	// resultType is wire-only in the MCP SDK, so initialize it through the
	// public JSON representation before all protocol adapters serialize it.
	result := &mcp.CallToolResult{}
	if err = json.Unmarshal([]byte(`{"content":[],"resultType":"complete"}`), result); err != nil {
		return nil, err
	}
	result.Meta = mcp.Meta{
		mcp.MetaKeyServerInfo: &mcp.Implementation{
			Name:    config.ProjectName,
			Version: config.ProjectVersion,
		},
	}
	result.Content = []mcp.Content{
		&mcp.TextContent{Text: content},
	}
	result.IsError = errorCode != api_error_code.Success
	return &CallToolResult{CallToolResult: result}, nil
}

func resultContentText(value interface{}, errorCode int, errorMessage string, encryptionKey string) (string, error) {
	if errorCode != api_error_code.Success {
		encoded, err := json.Marshal(map[string]interface{}{
			"error_code": errorCode,
			"error_msg":  errorMessage,
		})
		return string(encoded), err
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if api_config.CurrentApiCfg != nil && api_config.CurrentApiCfg.NeedEncryption {
		return gtbox_encryption.GTEnc(string(encoded), encryptionKey), nil
	}
	if text, ok := value.(string); ok {
		return text, nil
	}
	return string(encoded), nil
}
