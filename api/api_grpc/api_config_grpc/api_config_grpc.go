package api_config_grpc

var CurrentAPICfgGRPC *APIConfigGRPC

type APIConfigGRPC struct {
	Enabled bool `yaml:"enabled" json:"enabled" toml:"enabled"`
	Port    int  `yaml:"port" json:"port" toml:"port"`
}
