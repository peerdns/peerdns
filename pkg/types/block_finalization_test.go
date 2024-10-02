// pkg/types/block_finalization_test.go
package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockFinalizedBy(t *testing.T) {
	tests := []struct {
		name             string
		initialFinalized []string
		addFinalizer     string
		expectFinalized  []string
	}{
		{
			name:             "Add Single Finalizer",
			initialFinalized: []string{},
			addFinalizer:     "validator1",
			expectFinalized:  []string{"validator1"},
		},
		{
			name:             "Add Multiple Finalizers",
			initialFinalized: []string{"validator1"},
			addFinalizer:     "validator2",
			expectFinalized:  []string{"validator1", "validator2"},
		},
		{
			name:             "Add Duplicate Finalizer",
			initialFinalized: []string{"validator1"},
			addFinalizer:     "validator1",
			expectFinalized:  []string{"validator1", "validator1"},
		},
		{
			name:             "Add Finalizer to Empty List",
			initialFinalized: []string{},
			addFinalizer:     "validator3",
			expectFinalized:  []string{"validator3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a dummy previous hash and validator ID
			var previousHash [HashSize]byte
			copy(previousHash[:], "previous_hash_dummy_1234")

			var validatorID [AddressSize]byte
			copy(validatorID[:], "validator_main")

			// Create initial transactions
			transactions := []Transaction{
				{
					ID:        [HashSize]byte{0x10},
					Sender:    generate32Bytes("sender_finalizer"),
					Recipient: generate32Bytes("recipient_finalizer"),
					Amount:    4000,
					Fee:       40,
					Nonce:     4,
					Timestamp: 1625097600,
					Signature: [SignatureSize]byte{0x1A},
					Payload:   []byte("Finalization payload"),
				},
			}

			// Create the block
			block, err := NewBlock(4, previousHash, transactions, validatorID, 40)
			require.NoError(t, err, "Block creation should not fail")

			// Add initial finalized validators
			for _, v := range tt.initialFinalized {
				var addr [AddressSize]byte
				copy(addr[:], []byte(v))
				block.AddFinalizer(addr)
			}

			// Add the new finalizer
			var newFinalizer [AddressSize]byte
			copy(newFinalizer[:], []byte(tt.addFinalizer))
			block.AddFinalizer(newFinalizer)

			// Check if the FinalizedBy list matches expected
			finalizedStrs := make([]string, len(block.FinalizedBy))
			for i, addr := range block.FinalizedBy {
				finalizedStrs[i] = string(addr[:])
			}
			assert.Equal(t, tt.expectFinalized, finalizedStrs, "FinalizedBy list should match expected")
		})
	}
}
