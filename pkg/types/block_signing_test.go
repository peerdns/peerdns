// pkg/types/block_signing_test.go
package types

import (
	"testing"

	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockSigningAndVerification(t *testing.T) {
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
		expectValid     bool
	}{
		{
			name: "Valid Signature",
			block: Block{
				Index:        1,
				Timestamp:    1625097600,
				PreviousHash: [HashSize]byte{0x00},
				Hash:         [HashSize]byte{0x01},
				MerkleRoot:   [HashSize]byte{0x02},
				Signature:    [SignatureSize]byte{},
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
						Signature: [SignatureSize]byte{0x0A},
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
			expectValid:     true,
		},
		{
			name: "Invalid Signature",
			block: Block{
				Index:        2,
				Timestamp:    1625097601,
				PreviousHash: [HashSize]byte{0x04},
				Hash:         [HashSize]byte{0x05},
				MerkleRoot:   [HashSize]byte{0x06},
				Signature:    [SignatureSize]byte{0x0C},
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
						Signature: [SignatureSize]byte{0x0D},
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
			expectValid:     false,
		},
		{
			name: "Unsigned Block",
			block: Block{
				Index:        3,
				Timestamp:    1625097602,
				PreviousHash: [HashSize]byte{0x08},
				Hash:         [HashSize]byte{0x09},
				MerkleRoot:   [HashSize]byte{0x0A},
				Signature:    [SignatureSize]byte{}, // No signature
				ValidatorID:  generate32Bytes("validator3"),
				Transactions: []Transaction{
					{
						ID:        [HashSize]byte{0x0B},
						Sender:    generate32Bytes("sender3"),
						Recipient: generate32Bytes("recipient3"),
						Amount:    3000,
						Fee:       30,
						Nonce:     3,
						Timestamp: 1625097602,
						Signature: [SignatureSize]byte{0x0D},
						Payload:   []byte("Payload3"),
					},
				},
				Difficulty: 30,
				Version:    1,
				FinalizedBy: [][AddressSize]byte{
					generate32Bytes("validatorD"),
				},
			},
			modifySignature: false,
			expectValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sign the block if not testing an unsigned or invalid signature block
			if tt.name != "Invalid Signature" && tt.name != "Unsigned Block" {
				err := tt.block.Sign(privKey)
				require.NoError(t, err, "Failed to sign block")
			}

			// Optionally modify the signature to simulate tampering
			if tt.modifySignature && len(tt.block.Signature) > 0 {
				tt.block.Signature[0] ^= 0xFF // Flip some bits in the signature
			}

			// Verify the block's signature
			valid, err := tt.block.Verify(pubKey)
			if tt.expectValid {
				require.NoError(t, err, "Verification should not fail")
				assert.True(t, valid, "Block signature should be valid")
			} else {
				// If signature was modified or block is invalid, verification should fail
				if tt.name == "Unsigned Block" {
					assert.False(t, valid, "Unsigned block should not be valid")
				} else {
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
