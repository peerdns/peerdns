package encryption

import (
	"fmt"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

	"github.com/herumi/bls-go-binary/bls"
)

// BLSPublicKey wraps the herumi/bls PublicKey.
type BLSPublicKey struct {
	Key *bls.PublicKey
}

// GenerateBLSKeys generates a new pair of BLS keys.
func GenerateBLSKeys() (*BLSPrivateKey, *BLSPublicKey, error) {
	// Initialize the BLS library if not already done
	if err := InitBLS(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize BLS library")
	}

	var sk bls.SecretKey
	sk.SetByCSPRNG()

	pk := sk.GetPublicKey()
	return &BLSPrivateKey{&sk}, &BLSPublicKey{pk}, nil
}

// NewBLSPublicKey creates a new BLSPublicKey from the given herumi/bls PublicKey.
func NewBLSPublicKey(pk *bls.PublicKey) *BLSPublicKey {
	return &BLSPublicKey{Key: pk}
}

// Serialize serializes the public key.
func (pk *BLSPublicKey) Serialize() ([]byte, error) {
	if pk == nil || pk.Key == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return pk.Key.Serialize(), nil
}

// DeserializePublicKey deserializes bytes into a BLSPublicKey.
func DeserializePublicKey(data []byte) (*BLSPublicKey, error) {
	pk := new(bls.PublicKey)
	err := pk.Deserialize(data)
	if err != nil {
		return nil, err
	}
	return NewBLSPublicKey(pk), nil
}

// BLSPrivateKey wraps the herumi/bls SecretKey.
type BLSPrivateKey struct {
	Key *bls.SecretKey
}

// NewBLSPrivateKey creates a new BLSPrivateKey from the given herumi/bls SecretKey.
func NewBLSPrivateKey(sk *bls.SecretKey) *BLSPrivateKey {
	return &BLSPrivateKey{Key: sk}
}

// Serialize serializes the private key.
func (sk *BLSPrivateKey) Serialize() ([]byte, error) {
	if sk.Key == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return sk.Key.Serialize(), nil
}

// DeserializePrivateKey deserializes bytes into a BLSPrivateKey.
func DeserializePrivateKey(data []byte) (*BLSPrivateKey, error) {
	sk := new(bls.SecretKey)
	err := sk.Deserialize(data)
	if err != nil {
		return nil, err
	}
	return NewBLSPrivateKey(sk), nil
}

// ConvertBLSKeyToLibp2pPrivKey converts a BLS private key to a Libp2p-compatible private key.
func ConvertBLSKeyToLibp2pPrivKey(blsPrivKey *BLSPrivateKey) (crypto.PrivKey, error) {
	serializedPrivKey, err := blsPrivKey.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize BLS private key: %w", err)
	}

	// Unmarshal the serialized private key into a Libp2p-compatible key
	libp2pPrivKey, err := crypto.UnmarshalPrivateKey(serializedPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert BLS private key to libp2p private key: %w", err)
	}

	return libp2pPrivKey, nil
}

// ConvertBLSKeyToLibp2pPubKey converts a BLS public key to a Libp2p-compatible public key.
func ConvertBLSKeyToLibp2pPubKey(blsPubKey *BLSPublicKey) (crypto.PubKey, error) {
	serializedPubKey, err := blsPubKey.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize BLS public key: %w", err)
	}

	// Unmarshal the serialized public key into a Libp2p-compatible key
	libp2pPubKey, err := crypto.UnmarshalPublicKey(serializedPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert BLS public key to libp2p public key: %w", err)
	}

	return libp2pPubKey, nil
}

// DerivePeerIDFromPublicKey derives a peer.ID from a BLS public key.
func DerivePeerIDFromPublicKey(blsPubKey *BLSPublicKey) (peer.ID, error) {
	libp2pPubKey, err := ConvertBLSKeyToLibp2pPubKey(blsPubKey)
	if err != nil {
		return "", fmt.Errorf("failed to convert BLS key to libp2p public key: %w", err)
	}

	// Generate the peer ID from the Libp2p public key
	peerID, err := peer.IDFromPublicKey(libp2pPubKey)
	if err != nil {
		return "", fmt.Errorf("failed to derive peer ID from public key: %w", err)
	}

	return peerID, nil
}
