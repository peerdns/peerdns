// pkg/ledger/utils.go

package ledger

import (
	"github.com/peerdns/peerdns/pkg/types"
)

// blockHashMeetsTarget checks if the block's hash meets the difficulty target.
func blockHashMeetsTarget(hash, target types.Hash) bool {
	// Compare hash and target byte by byte.
	for i := 0; i < len(hash); i++ {
		if hash[i] < target[i] {
			return true
		} else if hash[i] > target[i] {
			return false
		}
	}
	return true
}

// CalculateTarget calculates the target hash value based on difficulty.
func CalculateTarget(difficulty uint64) types.Hash {
	// Implement target calculation based on difficulty.
	// This is a simplified placeholder function.
	// In a real implementation, you would compute the target based on difficulty.
	var target types.Hash
	for i := 0; i < len(target); i++ {
		target[i] = 0xFF
	}
	return target
}
