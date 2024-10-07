package signatures

import (
	"crypto"
	"crypto/rand"
	"fmt"

	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/zk/dleq"
)

// ZKProofKeyPair represents keys for ZKProofSigner.
type ZKProofKeyPair struct {
	PrivateKey group.Scalar
	PublicKeyA group.Element
	PublicKeyB group.Element
	Params     dleq.Params
}

// GenerateKey generates a new key pair for ZKProofSigner.
func (kp *ZKProofKeyPair) GenerateKey() error {
	// Use P256 as the elliptic curve group.
	kp.Params.G = group.P256

	// Define the domain separation string and hash function for DLEQ.
	domainSep := []byte("zkproof_domain_sep_string")
	kp.Params.H = crypto.SHA256
	kp.Params.DST = domainSep

	// Generate secret key and public keys explicitly.
	kp.PrivateKey = kp.Params.G.RandomScalar(rand.Reader)
	A := kp.Params.G.Generator() // Base point A
	kp.PublicKeyA = kp.Params.G.NewElement().Mul(A, kp.PrivateKey)
	B := kp.Params.G.Generator() // Base point B (could be different)
	kp.PublicKeyB = kp.Params.G.NewElement().Mul(B, kp.PrivateKey)
	return nil
}

// SerializePrivate serializes the private key to bytes.
func (kp *ZKProofKeyPair) SerializePrivate() ([]byte, error) {
	if kp.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return kp.PrivateKey.MarshalBinary()
}

// SerializePublic serializes the public keys to bytes.
func (kp *ZKProofKeyPair) SerializePublic() ([]byte, error) {
	if kp.PublicKeyA == nil || kp.PublicKeyB == nil {
		return nil, fmt.Errorf("public keys are nil")
	}
	dataA, err := kp.PublicKeyA.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PublicKeyA: %w", err)
	}
	dataB, err := kp.PublicKeyB.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PublicKeyB: %w", err)
	}
	// Include length prefixes for each public key
	lenA := uint16(len(dataA))
	lenB := uint16(len(dataB))
	serialized := make([]byte, 2+lenA+2+lenB)
	serialized[0] = byte(lenA >> 8)
	serialized[1] = byte(lenA)
	copy(serialized[2:], dataA)
	offset := 2 + lenA
	serialized[offset] = byte(lenB >> 8)
	serialized[offset+1] = byte(lenB)
	copy(serialized[offset+2:], dataB)
	return serialized, nil
}

// DeserializePrivate deserializes bytes into the private key.
func (kp *ZKProofKeyPair) DeserializePrivate(data []byte) error {
	if kp.Params.G == nil {
		return fmt.Errorf("group not initialized")
	}
	if kp.PrivateKey == nil {
		kp.PrivateKey = kp.Params.G.NewScalar()
	}
	if err := kp.PrivateKey.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("failed to unmarshal private key: %w", err)
	}
	return nil
}

// DeserializePublic deserializes bytes into the public keys.
func (kp *ZKProofKeyPair) DeserializePublic(data []byte) error {
	if kp.Params.G == nil {
		return fmt.Errorf("group not initialized")
	}
	if len(data) < 4 {
		return fmt.Errorf("invalid public key data length")
	}
	lenA := int(data[0])<<8 | int(data[1])
	if len(data) < 2+lenA+2 {
		return fmt.Errorf("invalid public key data length")
	}
	dataA := data[2 : 2+lenA]
	offset := 2 + lenA
	lenB := int(data[offset])<<8 | int(data[offset+1])
	if len(data) < offset+2+lenB {
		return fmt.Errorf("invalid public key data length")
	}
	dataB := data[offset+2 : offset+2+lenB]

	kp.PublicKeyA = kp.Params.G.NewElement()
	if err := kp.PublicKeyA.UnmarshalBinary(dataA); err != nil {
		return fmt.Errorf("failed to unmarshal PublicKeyA: %w", err)
	}
	kp.PublicKeyB = kp.Params.G.NewElement()
	if err := kp.PublicKeyB.UnmarshalBinary(dataB); err != nil {
		return fmt.Errorf("failed to unmarshal PublicKeyB: %w", err)
	}
	return nil
}

// GetPublic returns the public keys.
func (kp *ZKProofKeyPair) GetPublic() interface{} {
	return []group.Element{kp.PublicKeyA, kp.PublicKeyB}
}

// ZKProofSigner implements the Signer interface using DLEQ zero-knowledge proofs.
type ZKProofSigner struct {
	keyPair *ZKProofKeyPair
}

// NewZKProofSigner creates a new ZKProofSigner with generated keys.
func NewZKProofSigner() (*ZKProofSigner, error) {
	kp := &ZKProofKeyPair{}
	if err := kp.GenerateKey(); err != nil {
		return nil, err
	}
	return &ZKProofSigner{keyPair: kp}, nil
}

// NewZKProofSignerWithKeys creates a ZKProofSigner using provided serialized keys.
func NewZKProofSignerWithKeys(privateKeyData, publicKeyData []byte) (*ZKProofSigner, error) {
	kp := &ZKProofKeyPair{}
	kp.Params.G = group.P256
	kp.Params.H = crypto.SHA256
	kp.Params.DST = []byte("zkproof_domain_sep_string")

	// Deserialize private key
	if err := kp.DeserializePrivate(privateKeyData); err != nil {
		return nil, err
	}

	// Deserialize public keys
	if err := kp.DeserializePublic(publicKeyData); err != nil {
		return nil, err
	}
	return &ZKProofSigner{keyPair: kp}, nil
}

func (z *ZKProofSigner) Pair() *ZKProofKeyPair {
	return z.keyPair
}

func (z *ZKProofSigner) Type() SignerType {
	return DleqSignerType
}

// Sign creates a DLEQ proof.
func (z *ZKProofSigner) Sign(data []byte) ([]byte, error) {
	if z.keyPair.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	// Incorporate data into the proof by updating the domain separation tag.
	params := z.keyPair.Params
	params.DST = append(params.DST, data...)

	peggy := dleq.Prover{Params: params}

	proof, err := peggy.Prove(z.keyPair.PrivateKey, params.G.Generator(), z.keyPair.PublicKeyA, params.G.Generator(), z.keyPair.PublicKeyB, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create DLEQ proof: %w", err)
	}
	proofBytes, err := proof.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proof: %w", err)
	}
	return proofBytes, nil
}

// Verify verifies the DLEQ proof.
func (z *ZKProofSigner) Verify(data []byte, signature []byte) (bool, error) {
	if z.keyPair.PublicKeyA == nil || z.keyPair.PublicKeyB == nil {
		return false, fmt.Errorf("public keys are nil")
	}
	// Incorporate data into the verification by updating the domain separation tag.
	params := z.keyPair.Params
	params.DST = append(params.DST, data...)

	victor := dleq.Verifier{Params: params}

	proof := new(dleq.Proof)
	err := proof.UnmarshalBinary(params.G, signature)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal DLEQ proof: %w", err)
	}
	isValid := victor.Verify(params.G.Generator(), z.keyPair.PublicKeyA, params.G.Generator(), z.keyPair.PublicKeyB, proof)
	if !isValid {
		return false, fmt.Errorf("DLEQ proof is invalid")
	}
	return true, nil
}
