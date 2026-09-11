// Package api_supported_methods stores protocol-neutral API method definitions.
// 方法注册表同时是 API 文档的唯一事实源：新增方法后执行根目录 gen_api_docs.sh 重新生成方法清单。
package api_supported_methods

import "context"

type SupportedMethod struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Async       bool
	// Public 为 true 的方法免统一准入门禁（如 test、验证码、注册、登录）；
	// 零值 false = 受保护，APIExecuter 在 Execute 前验证凭证（fail-closed）：
	// 凭证来自 HTTP Authorization: Bearer 头，或显式 arguments.jwt_token（后者优先），
	// 验证后即从 arguments 移除，业务 Execute 只见业务参数；
	// jwt_token 入参 schema 由注册表按非 Public 自动注入为可选项，业务注册禁止声明。
	Public  bool
	Execute func(context.Context, interface{}) (interface{}, error)
}

var currentSupportedMethods []*SupportedMethod

func SupportedMethodsSetup() {
	currentSupportedMethods = nil
}

func AddMethod(method *SupportedMethod) {
	if method == nil || method.Name == "" || method.Execute == nil {
		panic("supported API method requires name and execute function")
	}
	for _, currentMethod := range currentSupportedMethods {
		if currentMethod.Name == method.Name {
			panic("duplicate supported API method: " + method.Name)
		}
	}
	injectGateTokenSchema(method)
	currentSupportedMethods = append(currentSupportedMethods, method)
}

// GateTokenArgument 是门禁凭证入参名；调用方带 HTTP Authorization: Bearer 头时可省略。
const GateTokenArgument = "jwt_token"

// jwtTokenSchema 是统一准入门禁的 wire 契约形状，与 AuthenticateRequest 的 1..8192 校验对齐。
// 字面量内联在注册表：门禁 wire 契约归 API 层，注册表保持零业务包依赖。
func jwtTokenSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"minLength":   1,
		"maxLength":   8192,
		"description": "门禁凭证（JWT 或 API key）。请求已带 HTTP Authorization: Bearer 头时省略；显式传入则优先于头。",
	}
}

// injectGateTokenSchema 给非 Public 方法注入可选的 jwt_token 入参 schema；
// 业务注册禁止自带 jwt_token，违者启动即 panic（fail-fast，防回退旧写法）。
func injectGateTokenSchema(method *SupportedMethod) {
	if method.Public {
		return
	}
	schema := method.InputSchema
	if schema == nil {
		panic("protected API method requires input schema: " + method.Name)
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		panic("protected API method input schema missing properties object: " + method.Name)
	}
	if _, exists := properties[GateTokenArgument]; exists {
		panic("jwt_token schema is gate-injected, business registration must not declare it: " + method.Name)
	}
	if rawRequired, exists := schema["required"]; exists {
		if _, typedOK := rawRequired.([]string); !typedOK {
			panic("protected API method input schema malformed required list: " + method.Name)
		}
	}
	properties[GateTokenArgument] = jwtTokenSchema()
}

// InputSchemaWithoutGateToken 返回去掉 jwt_token 入参的 schema 浅拷贝，供只走
// HTTP Authorization 头的 Adapter（MCP）对外描述工具；原 schema 不被修改。
func (method SupportedMethod) InputSchemaWithoutGateToken() map[string]interface{} {
	if method.Public || method.InputSchema == nil {
		return method.InputSchema
	}
	properties, ok := method.InputSchema["properties"].(map[string]interface{})
	if !ok {
		return method.InputSchema
	}
	copiedProperties := make(map[string]interface{}, len(properties))
	for key, value := range properties {
		if key != GateTokenArgument {
			copiedProperties[key] = value
		}
	}
	copiedSchema := make(map[string]interface{}, len(method.InputSchema))
	for key, value := range method.InputSchema {
		copiedSchema[key] = value
	}
	copiedSchema["properties"] = copiedProperties
	return copiedSchema
}

func Methods() []SupportedMethod {
	methods := make([]SupportedMethod, 0, len(currentSupportedMethods))
	for _, method := range currentSupportedMethods {
		methods = append(methods, *method)
	}
	return methods
}

func Method(name string) (*SupportedMethod, bool) {
	for _, method := range currentSupportedMethods {
		if method.Name == name {
			return method, true
		}
	}
	return nil, false
}
