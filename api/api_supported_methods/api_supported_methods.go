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
	// 零值 false = 受保护，APIExecuter 在 Execute 前验证 arguments.jwt_token（fail-closed）。
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
	currentSupportedMethods = append(currentSupportedMethods, method)
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
