package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBlockCreation(t *testing.T) {
	tests := []struct {
		name         string
		index        uint64
		previousHash Hash
		transactions []*Transaction
		validatorID  Address
		difficulty   uint64
		expectError  bool
	}{
		{
			name:         "Valid Block with Transactions",
			index:        1,
			previousHash: Hash{0x00},
			transactions: []*Transaction{
				{
					ID:        Hash{0x01},
					Sender:    generate32Bytes("sender1"),
					Recipient: generate32Bytes("recipient1"),
					Amount:    1000,
					Fee:       10,
					Nonce:     1,
					Timestamp: time.Now().Unix(),
					Signature: []byte{0x0A},
					Payload:   []byte("Payload1"),
				},
				{
					ID:        Hash{0x02},
					Sender:    generate32Bytes("sender2"),
					Recipient: generate32Bytes("recipient2"),
					Amount:    2000,
					Fee:       20,
					Nonce:     2,
					Timestamp: time.Now().Unix(),
					Signature: []byte{0x0B},
					Payload:   []byte("Payload2"),
				},
			},
			validatorID: Address{0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x31},
			difficulty:  10,
			expectError: false,
		},
		{
			name:         "Invalid Block with No Transactions",
			index:        2,
			previousHash: Hash{0x03},
			transactions: []*Transaction{},
			validatorID:  Address{0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x32},
			difficulty:   15,
			expectError:  true,
		},
		{
			name:         "Block with Maximum Transactions",
			index:        3,
			previousHash: Hash{0x04},
			transactions: func() []*Transaction {
				txs := make([]*Transaction, MaximumPayloadSize)
				for i := 0; i < MaximumPayloadSize; i++ {
					txs[i] = &Transaction{
						ID:        Hash{byte(i)},
						Sender:    generate32Bytes("sender_max"),
						Recipient: generate32Bytes("recipient_max"),
						Amount:    uint64(i * 100),
						Fee:       uint64(i * 10),
						Nonce:     uint64(i),
						Timestamp: time.Now().Unix(),
						Signature: []byte{byte(0x0C + i)},
						Payload:   []byte("Max Payload"),
					}
				}
				return txs
			}(),
			validatorID: Address{0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x33},
			difficulty:  20,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := NewBlock(tt.index, tt.previousHash, tt.transactions, tt.validatorID, tt.difficulty)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, block)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, block)
				assert.Equal(t, tt.index, block.Index)
				assert.Equal(t, tt.previousHash, block.PreviousHash)
				assert.Equal(t, tt.transactions, block.Transactions)
				assert.Equal(t, tt.validatorID, block.ValidatorID)
				assert.Equal(t, tt.difficulty, block.Difficulty)
				assert.Equal(t, uint32(1), block.Version)
				assert.NotEmpty(t, block.Hash, "Block hash should not be empty")
				assert.NotEmpty(t, block.MerkleRoot, "Merkle root should not be empty")
			}
		})
	}
}
