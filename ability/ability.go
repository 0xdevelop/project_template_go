// Package ability ability/ability.go
package ability

import (
	"context"

	"github.com/0xYeah/project_template_go/api/api_supported_methods"
)

const MethodTest = "test"

func Test(context.Context, interface{}) (interface{}, error) {
	return "this is test method, request is success", nil
}

func LoadAbilityAPIMethods() {
	api_supported_methods.SupportedMethodsSetup()
	api_supported_methods.AddMethod(
		&api_supported_methods.SupportedMethod{
			Name:        MethodTest,
			Description: "检查统一 API 调用链是否可用",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
			},
			Execute: Test,
		})
}
