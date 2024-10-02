package config

import (
	"fmt"
	"github.com/peerdns/peerdns/pkg/types"
	"gopkg.in/yaml.v3"
	"os"
)

var global *Config

// Config represents the overall application configuration, which includes logging,
// transports, MDBX nodes, networking, identity, sharding, pprof profiling options,
// and observability settings.
type Config struct {
	// Logger holds the configuration for the logging system, including log level and environment.
	Logger Logger `yaml:"logger"`

	// Transports is a list of various transport configurations (e.g., Dummy, UDS, QUIC).
	// Each transport has its own specific configuration settings.
	Transports []Transport `yaml:"transports"`

	// Mdbx contains the configuration for MDBX database nodes, including paths, sizes, and permissions.
	Mdbx Mdbx `yaml:"mdbx"`

	// Networking holds the configuration for the P2P networking.
	Networking Networking `yaml:"networking"`

	// Identity holds the configuration for the service identity.
	Identity Identity `yaml:"identity"`

	// Sharding contains the configuration for sharding settings.
	Sharding Sharding `yaml:"sharding"`

	// Pprof is a list of pprof profiling configurations, each tied to a specific service or subsystem.
	Pprof []Pprof `yaml:"pprof"`

	// EBPF contains the configuration for eBPF settings.
	EBPF EBPF `yaml:"ebpf"`

	// Observability holds all configurations related to metrics, tracing, and logging.
	Observability Observability `yaml:"observability"`
}

// Validate checks the integrity of the loaded configuration.
// Currently, it returns nil, but you can extend it to perform validation on
// the various configuration fields to ensure they are set correctly.
//
// Example usage:
//
//	if err := config.Validate(); err != nil {
//	    log.Fatalf("Invalid configuration: %v", err)
//	}
//
// Returns:
//
//	error: Returns nil if the configuration is valid, or an error if validation fails.
func (c Config) Validate() error {
	return nil
}

// GetTransportByType retrieves a specific transport configuration based on its type (e.g., UDS, Dummy).
// This method allows you to access a particular transport configuration when multiple transports are defined.
//
// Example usage:
//
//	transport := config.GetTransportByType(types.DummyTransportType)
//	if transport == nil {
//	    log.Fatalf("Transport not found")
//	}
//
// Parameters:
//
//	transportType (types.TransportType): The type of transport to search for.
//
// Returns:
//
//	*Transport: Returns a pointer to the matching transport configuration if found, or nil if no match is found.
func (c Config) GetTransportByType(transportType types.TransportType) *Transport {
	for _, t := range c.Transports {
		if t.Type == transportType {
			return &t
		}
	}
	return nil
}

// LoadConfig loads the configuration from a YAML file into the Config struct.
// This function reads the specified YAML configuration file and unmarshals it into
// a Config struct, enabling structured access to configuration settings.
//
// Example usage:
//
//	config, err := LoadConfig("/path/to/config.yaml")
//	if err != nil {
//	    log.Fatalf("Failed to load config: %v", err)
//	}
//
// Parameters:
//
//	filename (string): The path to the YAML configuration file.
//
// Returns:
//
//	*Config: Returns a pointer to the Config struct containing the parsed configuration.
//	error: Returns an error if reading or unmarshaling the YAML file fails.
func LoadConfig(filename string) (*Config, error) {
	// Read the configuration file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal the YAML data into the Config struct
	var rawConfig Config
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

func G() *Config {
	return global
}

func InitializeGlobalConfig(filename string) (*Config, error) {
	rawConfig, err := LoadConfig(filename)
	if err != nil {
		return nil, err
	}
	global = rawConfig
	return rawConfig, nil
}
