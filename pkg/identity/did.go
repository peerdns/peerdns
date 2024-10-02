package identity

import (
	"fmt"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/pkg/errors"
)

// DID represents a Decentralized Identifier with associated cryptographic keys.
type DID struct {
	ID                string                    `yaml:"id"`      // String representation of the peer ID
	PeerID            peer.ID                   `yaml:"-"`       // Associated peer.ID object for easier manipulation
	Name              string                    `yaml:"name"`    // Name of the DID for descriptive purposes
	PeerPrivateKey    crypto.PrivKey            `yaml:"-"`       // Private key associated with libp2p PeerID
	PeerPublicKey     crypto.PubKey             `yaml:"-"`       // Public key associated with libp2p PeerID
	SigningPrivateKey *encryption.BLSPrivateKey `yaml:"-"`       // BLS Private key used for signing
	SigningPublicKey  *encryption.BLSPublicKey  `yaml:"-"`       // BLS Public key used for signing
	Comment           string                    `yaml:"comment"` // Optional comment or description for the DID
}

// NewDID initializes a new DID with all the cryptographic keys and metadata.
func NewDID(peerID peer.ID, peerSk crypto.PrivKey, peerPk crypto.PubKey,
	signingSk *encryption.BLSPrivateKey, signingPk *encryption.BLSPublicKey, name, comment string) *DID {

	return &DID{
		ID:                peerID.String(),
		PeerID:            peerID,    // Store the peer.ID directly
		Name:              name,      // Assign the provided name to the DID
		PeerPrivateKey:    peerSk,    // Store the libp2p private key for PeerID
		PeerPublicKey:     peerPk,    // Store the libp2p public key for PeerID
		SigningPrivateKey: signingSk, // Store the BLS private key for signing
		SigningPublicKey:  signingPk, // Store the BLS public key for signing
		Comment:           comment,   // Store the optional comment
	}
}

// Sign signs the given data using the DID's BLS signing private key.
func (d *DID) Sign(data []byte) (*encryption.BLSSignature, error) {
	if d.SigningPrivateKey == nil {
		return nil, errors.New("missing signing private key in DID")
	}

	// Sign the data using the BLS private key
	signature, err := encryption.Sign(data, d.SigningPrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign data")
	}

	return signature, nil
}

// Verify verifies the given signature for the data using the BLS public key.
func (d *DID) Verify(data []byte, signature *encryption.BLSSignature) (bool, error) {
	if d.SigningPublicKey == nil {
		return false, fmt.Errorf("public key is nil in DID: %s", d.Name)
	}

	valid, err := encryption.Verify(data, signature, d.SigningPublicKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to verify signature")
	}
	return valid, nil
}
