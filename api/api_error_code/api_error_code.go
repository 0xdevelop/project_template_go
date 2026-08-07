// Package api_error_code defines stable business errors returned by API methods.
package api_error_code

import "errors"

const (
	Success = 0

	MethodNotFound           = 10001
	MethodNotSupported       = 10002
	InvalidArguments         = 10003
	PermissionDenied         = 10004
	VerifyCodeDeliveryFailed = 10005
)

type Error struct {
	Code    int
	Message string
}

func (err *Error) Error() string {
	return err.Message
}

func As(err error) (*Error, bool) {
	var businessError *Error
	if !errors.As(err, &businessError) {
		return nil, false
	}
	return businessError, true
}

var (
	ErrMethodNotFound = &Error{
		Code:    MethodNotFound,
		Message: "method not found",
	}
	ErrMethodNotSupported = &Error{
		Code:    MethodNotSupported,
		Message: "method is not supported",
	}
	ErrInvalidArguments = &Error{
		Code:    InvalidArguments,
		Message: "invalid arguments",
	}
	ErrPermissionDenied = &Error{
		Code:    PermissionDenied,
		Message: "permission denied",
	}
	ErrVerifyCodeDeliveryFailed = &Error{
		Code:    VerifyCodeDeliveryFailed,
		Message: "verification code delivery failed",
	}
)
