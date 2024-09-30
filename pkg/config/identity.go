package config

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// Identity defines the configuration for identity management,
// including the base path for storing identities and a list of identity keys.
type Identity struct {
	Enabled  bool   `yaml:"enabled"`  // Enable or disable identity management
	BasePath string `yaml:"basePath"` // Base path for storing identity files
	Keys     []Key  `yaml:"keys"`     // List of keys used in identity management
}

// Key represents a single identity key configuration.
// It holds references to both private and public keys for each identity.
type Key struct {
	Name              string  `yaml:"name"`              // Name of the identity key (e.g., "main", "validator1")
	PeerID            peer.ID `yaml:"peerID"`            // Associated peer.ID for this identity key
	PeerPrivateKey    string  `yaml:"peerPrivateKey"`    // File name for the peer private key, relative to BasePath
	PeerPublicKey     string  `yaml:"peerPublicKey"`     // File name for the peer public key, relative to BasePath
	SigningPrivateKey string  `yaml:"signingPrivateKey"` // File name for the BLS private key, relative to BasePath
	SigningPublicKey  string  `yaml:"signingPublicKey"`  // File name for the BLS public key, relative to BasePath
	Comment           string  `yaml:"comment"`           // Optional comment for describing the purpose of this key
}
