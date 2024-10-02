package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

// Genesis holds the configuration for the genesis block.
type Genesis struct {
	BlockHash string `yaml:"block_hash"` // Predefined hash for the genesis block
}

func (g Genesis) Validate() error {
	return nil
}

func LoadGenesis(filename string) (*Genesis, error) {
	// Read the configuration file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal the YAML data into the Config struct
	var rawConfig Genesis
	err = yaml.Unmarshal(data, &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	// Optional: Validate the loaded configuration
	if err = rawConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &rawConfig, nil
}
