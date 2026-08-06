package api_config_websocket

var CurrentAPICfgWebSocket *APIConfigWebSocket

type APIConfigWebSocket struct {
	Enabled        bool     `yaml:"enabled" json:"enabled" toml:"enabled"`
	Port           int      `yaml:"port" json:"port" toml:"port"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins" toml:"allowed_origins"`
}
