package signatures

import (
	"crypto/sha256"
	"fmt"

	"github.com/cloudflare/circl/sign/ed25519"
)

// Ed25519KeyPair represents an Ed25519 key pair.
type Ed25519KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// GenerateKey generates a new Ed25519 key pair.
func (kp *Ed25519KeyPair) GenerateKey() error {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}
	kp.PrivateKey = privateKey
	kp.PublicKey = publicKey
	return nil
}

// SerializePrivate serializes the private key to bytes.
func (kp *Ed25519KeyPair) SerializePrivate() ([]byte, error) {
	if kp.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return kp.PrivateKey, nil
}

// SerializePublic serializes the public key to bytes.
func (kp *Ed25519KeyPair) SerializePublic() ([]byte, error) {
	if kp.PublicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return kp.PublicKey, nil
}

// DeserializePrivate deserializes bytes into the private key.
func (kp *Ed25519KeyPair) DeserializePrivate(data []byte) error {
	if data == nil || len(data) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key data")
	}
	kp.PrivateKey = data
	// The public key is the last 32 bytes of the private key.
	kp.PublicKey = data[32:]
	return nil
}

// DeserializePublic deserializes bytes into the public key.
func (kp *Ed25519KeyPair) DeserializePublic(data []byte) error {
	if data == nil || len(data) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key data")
	}
	kp.PublicKey = data
	return nil
}

// GetPublic returns the public key.
func (kp *Ed25519KeyPair) GetPublic() interface{} {
	return kp.PublicKey
}

// Ed25519Signer implements the Signer interface using Ed25519 signatures.
type Ed25519Signer struct {
	keyPair *Ed25519KeyPair
}

// NewEd25519Signer creates a new Ed25519Signer with a generated key pair.
func NewEd25519Signer() (*Ed25519Signer, error) {
	kp := &Ed25519KeyPair{}
	if err := kp.GenerateKey(); err != nil {
		return nil, err
	}
	return &Ed25519Signer{keyPair: kp}, nil
}

// NewEd25519SignerWithKeys creates an Ed25519Signer using provided serialized keys.
func NewEd25519SignerWithKeys(privateKeyData, publicKeyData []byte) (*Ed25519Signer, error) {
	kp := &Ed25519KeyPair{}
	if err := kp.DeserializePrivate(privateKeyData); err != nil {
		return nil, err
	}
	if err := kp.DeserializePublic(publicKeyData); err != nil {
		return nil, err
	}
	return &Ed25519Signer{keyPair: kp}, nil
}

func (e *Ed25519Signer) Pair() *Ed25519KeyPair {
	return e.keyPair
}

func (e *Ed25519Signer) Type() SignerType {
	return Ed25519SignerType
}

// Sign signs the given data using Ed25519.
func (e *Ed25519Signer) Sign(data []byte) ([]byte, error) {
	if e.keyPair.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	if data == nil || len(data) == 0 {
		return nil, fmt.Errorf("data to sign is empty")
	}
	hash := sha256.Sum256(data)
	signature := ed25519.Sign(e.keyPair.PrivateKey, hash[:])
	return signature, nil
}

// Verify verifies the Ed25519 signature for the given data.
func (e *Ed25519Signer) Verify(data []byte, signature []byte) (bool, error) {
	if e.keyPair.PublicKey == nil {
		return false, fmt.Errorf("public key is nil")
	}
	if data == nil || len(data) == 0 {
		return false, fmt.Errorf("data to verify is empty")
	}
	if signature == nil || len(signature) != ed25519.SignatureSize {
		return false, fmt.Errorf("invalid signature")
	}
	hash := sha256.Sum256(data)
	isValid := ed25519.Verify(e.keyPair.PublicKey, hash[:], signature)
	return isValid, nil
}
