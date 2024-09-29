package consensus

import (
	"crypto/sha256"
)

// HashData creates a SHA-256 hash of the input data.
func HashData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}
