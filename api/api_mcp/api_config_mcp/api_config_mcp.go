package api_config_mcp

var (
	CurrentAPICfgMCP *APIConfigMCP
)

type MCPTransportType string

const (
	MCPTransportTypeStdio          MCPTransportType = "stdio"           // local process via stdin/stdout
	MCPTransportTypeSSE            MCPTransportType = "sse"             // HTTP + Server-Sent Events
	MCPTransportTypeStreamableHTTP MCPTransportType = "streamable-http" // Streamable HTTP (preferred)
)

type APIConfigMCP struct {
	Enabled          bool             `yaml:"enabled" json:"enabled" toml:"enabled"`
	Port             int              `yaml:"port" json:"port" toml:"port"`
	MCPTransportType MCPTransportType `yaml:"mcp_transport_type" json:"mcp_transport_type" toml:"mcp_transport_type"`
}
