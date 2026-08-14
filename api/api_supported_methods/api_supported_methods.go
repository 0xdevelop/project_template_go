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
	// 零值 false = 受保护，APIExecuter 在 Execute 前验证 arguments.jwt_token（fail-closed），
	// 验证后即从 arguments 移除，业务 Execute 只见业务参数；
	// jwt_token 入参 schema 由注册表按非 Public 自动注入，业务注册禁止声明。
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

// jwtTokenSchema 是统一准入门禁的 wire 契约形状，与 AuthenticateRequest 的 1..8192 校验对齐。
// 字面量内联在注册表：门禁 wire 契约归 API 层，注册表保持零业务包依赖。
func jwtTokenSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":      "string",
		"minLength": 1,
		"maxLength": 8192,
	}
}

// injectGateTokenSchema 给非 Public 方法注入 jwt_token 入参 schema；
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
	if _, exists := properties["jwt_token"]; exists {
		panic("jwt_token schema is gate-injected, business registration must not declare it: " + method.Name)
	}
	// required 键缺席 = 零业务必填（InputSchema 省略空 required），jwt_token 注入后成为唯一必填；键在但类型不对才是坏 schema。
	var required []string
	if rawRequired, exists := schema["required"]; exists {
		typedRequired, typedOK := rawRequired.([]string)
		if !typedOK {
			panic("protected API method input schema malformed required list: " + method.Name)
		}
		required = typedRequired
	}
	properties["jwt_token"] = jwtTokenSchema()
	schema["required"] = append([]string{"jwt_token"}, required...)
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
