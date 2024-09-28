package encryption

import (
	"crypto/sha256"
	"fmt"
	"github.com/herumi/bls-go-binary/bls"
	"github.com/pkg/errors"
)

// InitBLS initializes the BLS library.
func InitBLS() error {
	if err := bls.Init(bls.BLS12_381); err != nil {
		return errors.Wrap(err, "bls init failed")
	}
	return nil
}

// BLSSignature represents a BLS signature.
type BLSSignature struct {
	Signature []byte // Raw BLS signature bytes
}

// Sign signs the given data using the provided BLS private key.
func Sign(data []byte, privateKey *BLSPrivateKey) (*BLSSignature, error) {
	hash := sha256.Sum256(data)
	sig := privateKey.Key.SignHash(hash[:])
	return &BLSSignature{Signature: sig.Serialize()}, nil
}

// Verify verifies the signature for the given data and public key.
func Verify(data []byte, signature *BLSSignature, publicKey *BLSPublicKey) bool {
	var sig bls.Sign
	if err := sig.Deserialize(signature.Signature); err != nil {
		return false
	}
	hash := sha256.Sum256(data)
	return sig.VerifyHash(publicKey.Key, hash[:])
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
