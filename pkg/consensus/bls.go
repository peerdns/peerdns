// pkg/consensus/bls.go
package consensus

import (
	"crypto/sha256"
	"fmt"

	"github.com/herumi/bls-eth-go-binary/bls"
)

// InitBLS initializes the BLS library.
func InitBLS() {
	bls.Init(bls.BLS12_381)
}

// BLSSignature represents a BLS signature.
type BLSSignature struct {
	Signature []byte // Raw BLS signature bytes
}

// BLSPrivateKey represents a BLS private key.
type BLSPrivateKey struct {
	PrivateKey bls.SecretKey // BLS private key object
}

// BLSPublicKey represents a BLS public key.
type BLSPublicKey struct {
	PublicKey bls.PublicKey // BLS public key object
}

// GenerateBLSKeys generates a new pair of BLS keys.
func GenerateBLSKeys() (*BLSPrivateKey, *BLSPublicKey, error) {
	var sk bls.SecretKey
	if err := sk.SetByCSPRNG(); err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	pk := sk.GetPublicKey()
	return &BLSPrivateKey{sk}, &BLSPublicKey{*pk}, nil
}

// SignData signs the given data using the provided BLS private key.
func SignData(data []byte, privateKey *BLSPrivateKey) (*BLSSignature, error) {
	hash := sha256.Sum256(data)
	sig := privateKey.PrivateKey.SignHash(hash[:])
	return &BLSSignature{Signature: sig.Serialize()}, nil
}

// VerifySignature verifies the signature for the given data and public key.
func VerifySignature(data []byte, signature *BLSSignature, publicKey *BLSPublicKey) bool {
	var sig bls.Sign
	if err := sig.Deserialize(signature.Signature); err != nil {
		return false
	}
	hash := sha256.Sum256(data)
	return sig.VerifyHash(&publicKey.PublicKey, hash[:])
}

// AggregateSignatures aggregates multiple BLS signatures into a single signature.
func AggregateSignatures(signatures []*BLSSignature) (*BLSSignature, error) {
	if len(signatures) == 0 {
		return nil, fmt.Errorf("no signatures to aggregate")
	}

	var aggregated bls.Sign
	for _, sig := range signatures {
		var s bls.Sign
		if err := s.Deserialize(sig.Signature); err != nil {
			return nil, fmt.Errorf("failed to deserialize signature: %w", err)
		}
		aggregated.Add(&s)
	}

	return &BLSSignature{Signature: aggregated.Serialize()}, nil
}
