package config

import "github.com/0xdevelop/project_template_go/common/resource"

func ConfiguredResourceSets() map[string]*resource.Config {
	if GlobalConfig == nil {
		return nil
	}
	return GlobalConfig.ResourceCfg
}

// The template has no model-specific installed-code contract.
func VerifyResourceRuntime(id string) error { return nil }

func VerifyResourceRuntimeBinding(id, path string) error { return resource.ErrSetUnavailable }
