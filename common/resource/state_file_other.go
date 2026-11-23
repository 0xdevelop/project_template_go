//go:build !unix

package resource

import (
	"errors"
	"os"
)

func openStateFile(name string, flags int) (*os.File, error) {
	return nil, errors.New("resource staging writes unsupported on this platform")
}

func openResourceFile(name string) (*os.File, error) { return os.Open(name) }
