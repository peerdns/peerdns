package signatures

// Signer defines the common interface for signing data.
type Signer interface {
	Sign(data []byte) ([]byte, error)
	Type() SignerType
	Verify(data []byte, signature []byte) (bool, error)
}

// KeyPair defines the interface for key management.
type KeyPair interface {
	GenerateKey() error
	SerializePrivate() ([]byte, error)
	SerializePublic() ([]byte, error)
	DeserializePrivate(data []byte) error
	DeserializePublic(data []byte) error
	GetPublic() any
}
