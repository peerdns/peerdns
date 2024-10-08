package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

var (
	ZeroHash = Hash{}
)

// Hash represents a 32-byte SHA-256 hash.
type Hash [HashSize]byte

// SumHash computes the SHA-256 hash of the input data.
func SumHash(data []byte) Hash {
	return sha256.Sum256(data)
}

// HashEqual compares two hashes for equality.
func HashEqual(a, b Hash) bool {
	return bytes.Equal(a[:], b[:])
}

// HashData creates a SHA-256 hash of the input data.
func HashData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func IsZeroHash(h Hash) bool {
	zeroHash := make([]byte, len(h))
	return bytes.Equal(h[:], zeroHash)
}

// HashFromBytes creates a Hash from a byte slice.
// Returns an error if the slice is not exactly HashSize bytes.
func HashFromBytes(data []byte) (Hash, error) {
	var h Hash
	if len(data) != HashSize {
		return h, fmt.Errorf("invalid hash length: expected %d bytes, got %d", HashSize, len(data))
	}
	copy(h[:], data)
	return h, nil
}

// Bytes returns the byte slice representation of the Hash.
func (h Hash) Bytes() []byte {
	return h[:]
}

// String returns the hexadecimal string representation of the Hash.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// Hex returns the hexadecimal string representation of the Hash.
func (h Hash) Hex() string {
	return hex.EncodeToString(h[:])
}

// Equal compares two Hashes for equality.
func (h Hash) Equal(other Hash) bool {
	return h == other
}

// MarshalText implements the encoding.TextMarshaler interface.
func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.Hex()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (h *Hash) UnmarshalText(text []byte) error {
	bytes, err := hex.DecodeString(string(text))
	if err != nil {
		return err
	}
	if len(bytes) != HashSize {
		return fmt.Errorf("invalid hash length: expected %d bytes, got %d", HashSize, len(bytes))
	}
	copy(h[:], bytes)
	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (h Hash) MarshalBinary() ([]byte, error) {
	return h[:], nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (h *Hash) UnmarshalBinary(data []byte) error {
	if len(data) != HashSize {
		return fmt.Errorf("invalid hash length: expected %d bytes, got %d", HashSize, len(data))
	}
	copy(h[:], data)
	return nil
}
