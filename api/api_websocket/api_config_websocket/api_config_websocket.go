package api_config_websocket

var CurrentAPICfgWebSocket *APIConfigWebSocket

type APIConfigWebSocket struct {
	Enabled bool `yaml:"enabled" json:"enabled" toml:"enabled"`
	// BindAddress 是监听地址（必填，无缺省）：127.0.0.1 仅本机，0.0.0.0 全部网卡。
	BindAddress    string   `yaml:"bind_address" json:"bind_address" toml:"bind_address" comment:"监听地址（必填）：127.0.0.1 仅本机；0.0.0.0 对外"`
	Port           int      `yaml:"port" json:"port" toml:"port"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins" toml:"allowed_origins"`
}
