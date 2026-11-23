//go:build !linux && !darwin

package resource

import (
	"context"
	"errors"
)

func lock(context.Context, string) (func(), error) {
	return nil, errors.New("cross-process resource locks unsupported on this platform")
}
