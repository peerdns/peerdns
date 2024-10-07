package config

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/peerdns/peerdns/pkg/types"
)

// SignerKey represents the private and public keys for a specific signer type.
type SignerKey struct {
	SigningPrivateKey string `yaml:"privateKey"` // Hex-encoded private key
	SigningPublicKey  string `yaml:"publicKey"`  // Hex-encoded public key
}

// Key represents a single identity key configuration with multiple signers.
type Key struct {
	Name           string                              `yaml:"name"`    // Name of the identity key
	Address        types.Address                       `yaml:"address"` // Hex-encoded Ed25519 public key
	PeerID         peer.ID                             `yaml:"peerID"`  // Associated peer.ID
	PeerPrivateKey string                              `yaml:"peerPrivateKey"`
	PeerPublicKey  string                              `yaml:"peerPublicKey"`
	Signers        map[signatures.SignerType]SignerKey `yaml:"signers"` // Map of signer types to their keys
	Comment        string                              `yaml:"comment"` // Optional comment
}

// Identity defines the configuration for identity management.
type Identity struct {
	Enabled  bool   `yaml:"enabled"`  // Enable or disable identity management
	BasePath string `yaml:"basePath"` // Base path for storing identity files
	Keys     []Key  `yaml:"keys"`     // List of keys used in identity management
}
