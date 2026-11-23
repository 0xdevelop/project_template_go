// Package app_path resolves owned resource paths without changing cwd or environment.
package app_path

import (
	"errors"
	"os"
	"path/filepath"
)

func ExecutableDir() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}
func Resolve(root, path string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", errors.New("executable resource root must be absolute")
	}
	if path == "" {
		return "", nil
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(root, path), nil
}
