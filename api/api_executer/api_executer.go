// Package api_executer api/api_executer/api_executer.go
package api_executer

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xYeah/project_template_go/ability/ability_task"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_session"
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
	if !supportedMethod.Public {
		if ctx, err = api_auth_session.AuthenticateRequest(ctx, abilityParams); err != nil {
			return finish(nil, err, encryptionKey)
		}
		// 门禁参数生命周期到此终结：业务 Execute 与 Async 落库只见业务参数
		delete(abilityParams, "jwt_token")
	}
	// Async 受理语义的一次性接入（AGENTS 契约预留）：事务写任务记录并返回 task_id，
	// Worker 后续调用同一注册项的 Execute。
	if supportedMethod.Async {
		acceptedValue, acceptErr := ability_task.AcceptAsyncTask(ctx, methodName, abilityParams)
		return finish(acceptedValue, acceptErr, encryptionKey)
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
