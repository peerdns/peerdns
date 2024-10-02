package chain

// Block represents a basic block structure
type Block struct {
	Hash         []byte
	PreviousHash []byte
	Data         []byte
	// Add other necessary fields, e.g., timestamp, nonce
}

// Serialize serializes the block
func (b *Block) Serialize() ([]byte, error) {
	// Implement serialization logic (e.g., using gob or protobuf)
	return b.Data, nil
}

// DeserializeBlock deserializes data into a Block
func DeserializeBlock(data []byte) (*Block, error) {
	// Implement deserialization logic
	return &Block{Data: data}, nil
}
