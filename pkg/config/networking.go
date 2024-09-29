package config

// Networking holds the configuration for the P2P networking.
type Networking struct {
	ListenPort     int      `yaml:"listen_port"`
	ProtocolID     string   `yaml:"protocol_id"`
	BootstrapPeers []string `yaml:"bootstrap_peers"`
}
