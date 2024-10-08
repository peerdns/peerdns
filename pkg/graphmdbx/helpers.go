package graphmdbx

import (
	"encoding/hex"
	"fmt"
	"github.com/peerdns/peerdns/pkg/types"
)

// decodeID converts a string to type K.
func (s *Store[K, T]) decodeID(idStr string) (K, error) {
	var id K
	switch any(id).(type) {
	case string:
		return any(idStr).(K), nil
	case types.Hash:
		// Decode the hex string to bytes
		hashBytes, err := hex.DecodeString(idStr)
		if err != nil {
			return id, fmt.Errorf("failed to decode hex string: %w", err)
		}

		// Check if the decoded hashBytes has the expected length
		if len(hashBytes) != types.HashSize {
			return id, fmt.Errorf("invalid hash length: expected %d bytes, got %d", types.HashSize, len(hashBytes))
		}

		// Unmarshal the hash
		var hash types.Hash
		if err := hash.UnmarshalBinary(hashBytes); err != nil {
			return id, fmt.Errorf("failed to unmarshal hash from bytes: %w", err)
		}
		return any(hash).(K), nil
	default:
		return id, fmt.Errorf("unsupported type for vertex ID")
	}
}
