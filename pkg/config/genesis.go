package config

// Genesis holds the configuration for the genesis block.
type Genesis struct {
	Path string `yaml:"path"` // Predefined hash for the genesis block
}
