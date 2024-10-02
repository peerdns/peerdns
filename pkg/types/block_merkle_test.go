// pkg/types/block_merkle_test.go
package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockMerkleRoot(t *testing.T) {
	tests := []struct {
		name           string
		transactions   []Transaction
		expectedMerkle [HashSize]byte
	}{
		{
			name: "Single Transaction",
			transactions: []Transaction{
				{
					ID:        [HashSize]byte{0x01},
					Sender:    generate32Bytes("sender_single"),
					Recipient: generate32Bytes("recipient_single"),
					Amount:    1000,
					Fee:       10,
					Nonce:     1,
					Timestamp: 1625097600,
					Signature: [SignatureSize]byte{0x0A},
					Payload:   []byte("Single transaction payload"),
				},
			},
			expectedMerkle: [HashSize]byte{0x01},
		},
		{
			name: "Two Transactions",
			transactions: []Transaction{
				{
					ID:        [HashSize]byte{0x02},
					Sender:    generate32Bytes("sender_two1"),
					Recipient: generate32Bytes("recipient_two1"),
					Amount:    2000,
					Fee:       20,
					Nonce:     2,
					Timestamp: 1625097600,
					Signature: [SignatureSize]byte{0x0B},
					Payload:   []byte("First of two transactions"),
				},
				{
					ID:        [HashSize]byte{0x03},
					Sender:    generate32Bytes("sender_two2"),
					Recipient: generate32Bytes("recipient_two2"),
					Amount:    3000,
					Fee:       30,
					Nonce:     3,
					Timestamp: 1625097600,
					Signature: [SignatureSize]byte{0x0C},
					Payload:   []byte("Second of two transactions"),
				},
			},
			expectedMerkle: func() [HashSize]byte {
				combined := append([]byte{0x02}, []byte{0x03}...)
				return sha256Sum(combined)
			}(),
		},
		{
			name: "Three Transactions (Odd Number)",
			transactions: []Transaction{
				{
					ID:        [HashSize]byte{0x04},
					Sender:    generate32Bytes("sender_three1"),
					Recipient: generate32Bytes("recipient_three1"),
					Amount:    4000,
					Fee:       40,
					Nonce:     4,
					Timestamp: 1625097600,
					Signature: [SignatureSize]byte{0x0D},
					Payload:   []byte("First of three transactions"),
				},
				{
					ID:        [HashSize]byte{0x05},
					Sender:    generate32Bytes("sender_three2"),
					Recipient: generate32Bytes("recipient_three2"),
					Amount:    5000,
					Fee:       50,
					Nonce:     5,
					Timestamp: 1625097600,
					Signature: [SignatureSize]byte{0x0E},
					Payload:   []byte("Second of three transactions"),
				},
				{
					ID:        [HashSize]byte{0x06},
					Sender:    generate32Bytes("sender_three3"),
					Recipient: generate32Bytes("recipient_three3"),
					Amount:    6000,
					Fee:       60,
					Nonce:     6,
					Timestamp: 1625097600,
					Signature: [SignatureSize]byte{0x0F},
					Payload:   []byte("Third of three transactions"),
				},
			},
			expectedMerkle: func() [HashSize]byte {
				// Compute first level
				combined1 := append([]byte{0x04}, []byte{0x05}...)
				hash1 := sha256Sum(combined1)

				// Second level with odd transaction
				combined2 := append(hash1[:], []byte{0x06}...)
				hash2 := sha256Sum(combined2)

				return hash2
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a dummy previous hash and validator ID
			var previousHash [HashSize]byte
			copy(previousHash[:], "previous_hash_dummy_1234")

			var validatorID [AddressSize]byte
			copy(validatorID[:], "validator_dummy_1234")

			// Create the block
			block, err := NewBlock(1, previousHash, tt.transactions, validatorID, 10)
			if len(tt.transactions) == 0 {
				// Blocks with no transactions should have errored out
				assert.Error(t, err)
				return
			}
			require.NoError(t, err, "Block creation should not fail")
			require.NotNil(t, block, "Block should not be nil")

			// Check Merkle root
			assert.Equal(t, tt.expectedMerkle, block.MerkleRoot, "Merkle root should match expected value")

			// Optionally, verify the Merkle root using the block's method
			computedMerkle := block.computeMerkleRoot()
			assert.Equal(t, tt.expectedMerkle, computedMerkle, "Computed Merkle root should match expected value")
		})
	}
}
