package resource

// Config is transport-independent. Missing files require an explicit confirmation
// supplied by a local CLI/frontend control path, never a hidden daemon prompt.
// Domain manifests and URLs are supplied by the instance, not the task engine.
type Config struct {
	Enabled  bool    `json:"enabled" yaml:"enabled" toml:"enabled"`
	Files    []File  `json:"files" yaml:"files" toml:"files"`
	Download Options `json:"download" yaml:"download" toml:"download"`
}
