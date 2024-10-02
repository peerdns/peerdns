// pkg/types/block_serialization_test.go
package types

import (
	"testing"

	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockSerialization(t *testing.T) {
	// Initialize BLS
	err := encryption.InitBLS()
	require.NoError(t, err, "Failed to initialize BLS")

	// Generate BLS keys
	privKey, pubKey, err := encryption.GenerateBLSKeys()
	require.NoError(t, err, "Failed to generate BLS keys")

	tests := []struct {
		name            string
		block           Block
		modifySignature bool
		expectError     bool
		expectValid     bool
	}{
		{
			name: "Serialize and Deserialize Valid Block",
			block: Block{
				Index:        1,
				Timestamp:    1625097600,
				PreviousHash: [HashSize]byte{0x00},
				Hash:         [HashSize]byte{0x01},
				MerkleRoot:   [HashSize]byte{0x02},
				Signature:    [SignatureSize]byte{}, // Will be set after signing
				ValidatorID:  generate32Bytes("validator1"),
				Transactions: []Transaction{
					{
						ID:        [HashSize]byte{0x03},
						Sender:    generate32Bytes("sender1"),
						Recipient: generate32Bytes("recipient1"),
						Amount:    1000,
						Fee:       10,
						Nonce:     1,
						Timestamp: 1625097600,
						Signature: [SignatureSize]byte{}, // Will be set after signing
						Payload:   []byte("Payload1"),
					},
				},
				Difficulty: 10,
				Version:    1,
				FinalizedBy: [][AddressSize]byte{
					generate32Bytes("validatorA"),
					generate32Bytes("validatorB"),
				},
			},
			modifySignature: false,
			expectError:     false,
			expectValid:     true,
		},
		{
			name: "Serialize and Deserialize Block with Modified Signature",
			block: Block{
				Index:        2,
				Timestamp:    1625097601,
				PreviousHash: [HashSize]byte{0x04},
				Hash:         [HashSize]byte{0x05},
				MerkleRoot:   [HashSize]byte{0x06},
				Signature:    [SignatureSize]byte{}, // Will be set after signing
				ValidatorID:  generate32Bytes("validator2"),
				Transactions: []Transaction{
					{
						ID:        [HashSize]byte{0x07},
						Sender:    generate32Bytes("sender2"),
						Recipient: generate32Bytes("recipient2"),
						Amount:    2000,
						Fee:       20,
						Nonce:     2,
						Timestamp: 1625097601,
						Signature: [SignatureSize]byte{}, // Will be set after signing
						Payload:   []byte("Payload2"),
					},
				},
				Difficulty: 20,
				Version:    1,
				FinalizedBy: [][AddressSize]byte{
					generate32Bytes("validatorC"),
				},
			},
			modifySignature: true,
			expectError:     false,
			expectValid:     false,
		},
		{
			name: "Deserialize Invalid Block Data",
			block: Block{
				Index:     3,
				Timestamp: 1625097602,
				// Intentionally incomplete fields
			},
			modifySignature: false,
			expectError:     true,
			expectValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sign the block if it's valid and not expecting an error
			if tt.name != "Deserialize Invalid Block Data" {
				err := tt.block.Sign(privKey)
				if tt.name == "Serialize and Deserialize Block with Modified Signature" {
					require.NoError(t, err, "Failed to sign block")
				}
			}

			// Serialize the block
			serialized, err := tt.block.Serialize()
			if tt.expectError {
				// Expecting an error during serialization
				assert.Error(t, err)
				return
			}
			require.NoError(t, err, "Serialization should not fail")

			// Optionally modify the signature to simulate tampering
			if tt.modifySignature && len(tt.block.Signature) > 0 {
				serializedOffset := 8 + 8 + HashSize + HashSize + HashSize // Skip to Signature
				if len(serialized) < serializedOffset+SignatureSize {
					t.Fatalf("Serialized data too short to modify signature")
				}
				serialized[serializedOffset] ^= 0xFF // Flip some bits in the signature
			}

			// Deserialize the block
			deserializedBlock, err := DeserializeBlock(serialized)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err, "Deserialization should not fail")
			require.NotNil(t, deserializedBlock, "Deserialized block should not be nil")

			// Compare fields except for the Signature if it was modified
			assert.Equal(t, tt.block.Index, deserializedBlock.Index, "Indices should match")
			assert.Equal(t, tt.block.Timestamp, deserializedBlock.Timestamp, "Timestamps should match")
			assert.Equal(t, tt.block.PreviousHash, deserializedBlock.PreviousHash, "Previous hashes should match")
			assert.Equal(t, tt.block.Hash, deserializedBlock.Hash, "Hashes should match")
			assert.Equal(t, tt.block.MerkleRoot, deserializedBlock.MerkleRoot, "Merkle roots should match")
			assert.Equal(t, tt.block.ValidatorID, deserializedBlock.ValidatorID, "Validator IDs should match")
			assert.Equal(t, tt.block.Transactions, deserializedBlock.Transactions, "Transactions should match")
			assert.Equal(t, tt.block.Difficulty, deserializedBlock.Difficulty, "Difficulties should match")
			assert.Equal(t, tt.block.Version, deserializedBlock.Version, "Versions should match")
			assert.Equal(t, tt.block.FinalizedBy, deserializedBlock.FinalizedBy, "FinalizedBy lists should match")

			// Verify the signature if expected to be valid
			if tt.expectValid {
				valid, err := deserializedBlock.Verify(pubKey)
				require.NoError(t, err, "Verification should not fail")
				assert.True(t, valid, "Block signature should be valid")
			} else {
				// If signature was modified or block is invalid, verification should fail
				if tt.name == "Deserialize Invalid Block Data" {
					assert.Error(t, err, "Verification should fail for incomplete block")
				} else {
					valid, err := deserializedBlock.Verify(pubKey)
					if err != nil {
						assert.Error(t, err, "Expected an error during verification")
					} else {
						assert.False(t, valid, "Block signature should be invalid")
					}
				}
			}
		})
	}
}
