// Package ability_user_profile 注册用户资料自管理的对外方法（昵称修改等）。
// UserName 是注册必填主标识，不提供修改接口；NickName 创建时默认取 UserName，可改。
package ability_user_profile

import (
	"context"
	"errors"

	"github.com/0xYeah/project_template_go/ability/ability_user"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_session"
	"github.com/0xYeah/project_template_go/api/api_error_code"
	"github.com/0xYeah/project_template_go/api/api_supported_methods"
)

const MethodNicknameChange = "user.nickname.change"

func LoadAPIMethods() {
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodNicknameChange,
		Description: "修改当前用户昵称",
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"jwt_token": api_auth_common.StringSchema("", 1, 8192),
				"nick_name": api_auth_common.StringSchema("", 1, 32),
			},
			"jwt_token",
			"nick_name",
		),
		Execute: UserNicknameChange,
	})
}

func UserNicknameChange(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "jwt_token", "nick_name") {
		return nil, api_error_code.ErrInvalidArguments
	}
	nickName, nickOK := api_auth_common.RequiredRawString(params, "nick_name")
	if !nickOK {
		return nil, api_error_code.ErrInvalidArguments
	}
	user, _, err := api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	updatedUser, err := ability_user.NicknameChangeByUserID(ctx, user.UserID, nickName)
	if errors.Is(err, ability_user.ErrInvalidNickName) {
		return nil, api_error_code.ErrInvalidArguments
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"user": updatedUser}, nil
}
