// Package api_executer api/api_executer/api_executer.go
package api_executer

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xYeah/project_template_go/api/api_error_code"
	"github.com/0xYeah/project_template_go/api/api_supported_methods"
	"github.com/george012/gtbox/gtbox_log"
)

var (
	ErrMethodNotFound   = api_error_code.ErrMethodNotFound
	ErrInvalidArguments = api_error_code.ErrInvalidArguments
	ErrInvalidCall      = errors.New("invalid tools/call request")
)

const (
	ToolsCallMethod = "tools/call"
)

func APIExecuter(ctx context.Context, method string, params interface{}, encryptionKey string) (*CallToolResult, error) {
	methodName, arguments, err := extractCall(method, params)
	if err != nil {
		return nil, err
	}
	gtbox_log.LogInfof("API method=[%s]", methodName)

	abilityParams, err := normalizeArguments(arguments, encryptionKey)
	if err != nil {
		return finish(nil, err, encryptionKey)
	}

	supportedMethod, ok := api_supported_methods.Method(methodName)
	if !ok {
		return finish(nil, ErrMethodNotFound, encryptionKey)
	}
	value, err := supportedMethod.Execute(ctx, abilityParams)
	return finish(value, err, encryptionKey)
}

func extractCall(method string, protocolParams interface{}) (string, interface{}, error) {
	if method != ToolsCallMethod {
		return "", nil, fmt.Errorf("%w: only tools/call is supported", ErrInvalidCall)
	}
	callParams, ok := protocolParams.(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("%w: tools/call params must be an object", ErrInvalidCall)
	}
	methodName, ok := callParams["name"].(string)
	if !ok || methodName == "" {
		return "", nil, fmt.Errorf("%w: tools/call params.name is required", ErrInvalidCall)
	}
	arguments, ok := callParams["arguments"]
	if !ok {
		return "", nil, fmt.Errorf("%w: tools/call params.arguments is required", ErrInvalidCall)
	}
	return methodName, arguments, nil
}
