// pkg/consensus/utils.go
package consensus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HashData creates a SHA-256 hash of the input data.
func HashData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// BytesToHex converts a byte slice to a hexadecimal string.
func BytesToHex(data []byte) string {
	return hex.EncodeToString(data)
}

// HexToBytes converts a hexadecimal string to a byte slice.
func HexToBytes(hexString string) ([]byte, error) {
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}
	return data, nil
}
