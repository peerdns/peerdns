package encryption

import (
	"fmt"

	"github.com/herumi/bls-go-binary/bls"
)

// BLSPublicKey wraps the herumi/bls PublicKey.
type BLSPublicKey struct {
	Key *bls.PublicKey
}

// GenerateBLSKeys generates a new pair of BLS keys.
func GenerateBLSKeys() (*BLSPrivateKey, *BLSPublicKey, error) {
	/*	if bls.GetCryptoSuite() != bls.BLS12_381 {
		return nil, nil, fmt.Errorf("BLS suite not initialized")
	}*/

	var sk bls.SecretKey
	sk.SetByCSPRNG() // No error returned

	pk := sk.GetPublicKey()
	return &BLSPrivateKey{&sk}, &BLSPublicKey{pk}, nil
}

// NewBLSPublicKey creates a new BLSPublicKey from the given herumi/bls PublicKey.
func NewBLSPublicKey(pk *bls.PublicKey) *BLSPublicKey {
	return &BLSPublicKey{Key: pk}
}

// Serialize serializes the public key.
func (pk *BLSPublicKey) Serialize() ([]byte, error) {
	if pk.Key == nil {
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
