package signatures

// SignerType represents the type of a signer.
type SignerType string

// String returns the string representation of the SignerType.
func (t SignerType) String() string {
	return string(t)
}

// Uint32 returns the uint32 representation of the SignerType.
func (t SignerType) Uint32() uint32 {
	switch t {
	case BlsSignerType:
		return 0
	case Ed25519SignerType:
		return 1
	case DleqSignerType:
		return 2
	default:
		return 0xFFFFFFFF // Represents an unknown SignerType
	}
}

// SignerTypeFromUint32 converts a uint32 to a SignerType.
func SignerTypeFromUint32(u uint32) SignerType {
	switch u {
	case 0:
		return BlsSignerType
	case 1:
		return Ed25519SignerType
	case 2:
		return DleqSignerType
	default:
		return UnknownSignerType
	}
}

const (
	BlsSignerType     SignerType = "bls"
	Ed25519SignerType SignerType = "ed25519"
	DleqSignerType    SignerType = "dleq"
	UnknownSignerType SignerType = "unknown"
)
