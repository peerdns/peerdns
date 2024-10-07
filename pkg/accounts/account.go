package accounts

import (
	"fmt"
	"github.com/peerdns/peerdns/pkg/types"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/signatures"
)

// Account represents a Decentralized Identifier with associated cryptographic keys.
type Account struct {
	ID             string                                      `yaml:"id"`      // String representation of the peer ID
	Address        types.Address                               `yaml:"-"`       // Derived address from Ed25519 public key
	PeerID         peer.ID                                     `yaml:"-"`       // Associated peer.ID object for easier manipulation
	Name           string                                      `yaml:"name"`    // Name of the DID for descriptive purposes
	PeerPrivateKey crypto.PrivKey                              `yaml:"-"`       // Private key associated with libp2p PeerID
	PeerPublicKey  crypto.PubKey                               `yaml:"-"`       // Public key associated with libp2p PeerID
	Signers        map[signatures.SignerType]signatures.Signer `yaml:"-"`       // BLS signer for signing and verification
	Comment        string                                      `yaml:"comment"` // Optional comment or description for the DID
}

// NewAccount initializes a new Account with all the cryptographic keys and metadata.
func NewAccount(peerID peer.ID, peerSk crypto.PrivKey, peerPk crypto.PubKey,
	signers map[signatures.SignerType]signatures.Signer, name, comment string) *Account {

	return &Account{
		ID:             peerID.String(),
		PeerID:         peerID,
		Name:           name,
		PeerPrivateKey: peerSk,
		PeerPublicKey:  peerPk,
		Signers:        signers,
		Comment:        comment,
	}
}

// Sign signs the given data using the DID's BLS signer.
func (a *Account) Sign(signerType signatures.SignerType, data []byte) ([]byte, error) {
	signer, exists := a.Signers[signerType]
	if !exists {
		return nil, fmt.Errorf("signer of type %s not found in account", signerType)
	}
	return signer.Sign(data)
}

// Verify verifies the given signature for the data using the BLS signer.
func (a *Account) Verify(signerType signatures.SignerType, data []byte, signature []byte) (bool, error) {
	signer, exists := a.Signers[signerType]
	if !exists {
		return false, fmt.Errorf("signer of type %s not found in account", signerType)
	}
	return signer.Verify(data, signature)
}
