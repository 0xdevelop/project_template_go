package api_config_jsonRPC

const (
	JSONRPCVersion = "2.0"
)

var (
	CurrentAPICfgJsonRPC *APIConfigJsonRPC
)

type APIConfigJsonRPC struct {
	Enabled bool `yaml:"enabled" json:"enabled" toml:"enabled"`
	Port    int  `yaml:"port" json:"port" toml:"port"`
	// EncryptionEnabled is kept for config compatibility. Use api_cfg.need_encryption.
	EncryptionEnabled bool `yaml:"encryption_enabled" json:"encryption_enabled" toml:"encryption_enabled"`
}
