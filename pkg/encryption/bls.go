package encryption

import (
	"crypto/sha256"
	"fmt"
	"github.com/herumi/bls-go-binary/bls"
	"sync"
)

var (
	initOnce sync.Once
	initErr  error
)

// InitBLS initializes the BLS library.
func InitBLS() error {
	initOnce.Do(func() {
		initErr = bls.Init(bls.BLS12_381)
	})
	return initErr
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
func Verify(data []byte, signature *BLSSignature, publicKey *BLSPublicKey) (bool, error) {
	if signature == nil {
		return false, fmt.Errorf("signature is nil")
	}
	if publicKey == nil {
		return false, fmt.Errorf("public key is nil")
	}

	var sig bls.Sign
	if err := sig.Deserialize(signature.Signature); err != nil {
		return false, fmt.Errorf("failed to deserialize signature: %w", err)
	}

	hash := sha256.Sum256(data)
	return sig.VerifyHash(publicKey.Key, hash[:]), nil
}

// AggregateSignatures aggregates multiple BLS signatures into a single signature.
func AggregateSignatures(signatures []*BLSSignature) (*BLSSignature, error) {
	if len(signatures) == 0 {
		return nil, fmt.Errorf("no signatures to aggregate")
	}

	var aggSig bls.Sign
	for _, sig := range signatures {
		if sig == nil || len(sig.Signature) == 0 {
			return nil, fmt.Errorf("one of the signatures is nil or empty")
		}
		var s bls.Sign
		if err := s.Deserialize(sig.Signature); err != nil {
			return nil, fmt.Errorf("failed to deserialize signature: %w", err)
		}
		aggSig.Add(&s)
	}

	return &BLSSignature{Signature: aggSig.Serialize()}, nil
}
