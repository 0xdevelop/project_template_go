// Package ability ability/ability.go
package ability

import (
	"context"

	"github.com/0xYeah/project_template_go/ability/ability_user/ability_user_profile"
	"github.com/0xYeah/project_template_go/api/api_auth"
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
			Public:      true,
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
			},
			Execute: Test,
		})
	api_auth.LoadAPIMethods()
	// user_profile 调用父包 ability_user 的数据方法，父包带子包装配会成 import 环
	//（user → profile → user），故作为契约明记的唯一例外由顶层装配。
	ability_user_profile.LoadAPIMethods()
}
