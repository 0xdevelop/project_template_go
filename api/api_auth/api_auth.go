// Package api_auth implements protocol-neutral Auth abilities.
package api_auth

import (
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_register"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_session"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_verify_code"
)

func LoadAPIMethods() {
	api_auth_verify_code.LoadAPIMethods()
	api_auth_register.LoadAPIMethods()
	api_auth_session.LoadAPIMethods()
}
