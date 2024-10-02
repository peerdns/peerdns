package types

const (
	// HashSize defines the size of a SHA-256 hash in bytes.
	HashSize = 32

	// SignatureSize defines the size of a T-BLS signature in bytes.
	SignatureSize = 96 // Adjust based on your BLS library's signature size

	// AddressSize defines the size of an address or public key in bytes.
	AddressSize = 32

	// MaximumPayloadSize defines the maximum allowed payload size in bytes.
	MaximumPayloadSize = 4096 // Example value, adjust as needed
)
