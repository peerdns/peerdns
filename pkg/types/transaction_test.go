package types

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTransactionCreation(t *testing.T) {
	tests := []struct {
		name      string
		sender    Address
		recipient Address
		amount    uint64
		fee       uint64
		nonce     uint64
		payload   []byte
		expectErr bool
	}{
		{
			name:      "Valid Transaction",
			sender:    Address{0x01, 0x02, 0x03, 0x04, 0x05},
			recipient: Address{0x06, 0x07, 0x08, 0x09, 0x0A},
			amount:    100,
			fee:       1,
			nonce:     1,
			payload:   []byte("Payload"),
			expectErr: false,
		},
		{
			name:      "Transaction with Zero Amount",
			sender:    Address{0x01, 0x02, 0x03, 0x04, 0x05},
			recipient: Address{0x06, 0x07, 0x08, 0x09, 0x0A},
			amount:    0,
			fee:       1,
			nonce:     1,
			payload:   []byte("Payload"),
			expectErr: false,
		},
		{
			name:      "Transaction with Zero Fee",
			sender:    Address{0x01, 0x02, 0x03, 0x04, 0x05},
			recipient: Address{0x06, 0x07, 0x08, 0x09, 0x0A},
			amount:    100,
			fee:       0,
			nonce:     1,
			payload:   []byte("Payload"),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := NewTransaction(tt.sender, tt.recipient, tt.amount, tt.fee, tt.nonce, tt.payload)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, tx)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tx)
				assert.Equal(t, tt.sender, tx.Sender)
				assert.Equal(t, tt.recipient, tx.Recipient)
				assert.Equal(t, tt.amount, tx.Amount)
				assert.Equal(t, tt.fee, tx.Fee)
				assert.Equal(t, tt.nonce, tx.Nonce)
				assert.Equal(t, tt.payload, tx.Payload)
				assert.NotEmpty(t, tx.ID, "Transaction ID should not be empty")
			}
		})
	}
}

func TestTransactionSerialization(t *testing.T) {
	sender := Address{0x01, 0x02, 0x03, 0x04, 0x05}
	recipient := Address{0x06, 0x07, 0x08, 0x09, 0x0A}
	amount := uint64(100)
	fee := uint64(1)
	nonce := uint64(1)
	payload := []byte("Payload")
	tx, err := NewTransaction(sender, recipient, amount, fee, nonce, payload)
	require.NoError(t, err)

	serialized, err := tx.Serialize()
	require.NoError(t, err)
	assert.NotEmpty(t, serialized, "Serialized transaction should not be empty")

	deserializedTx, err := DeserializeTransaction(serialized)
	require.NoError(t, err)

	// Set Signature and SignatureType to match default deserialization values
	tx.Signature = []byte{}
	tx.SignatureType = "unknown"

	assert.Equal(t, tx, deserializedTx, "Deserialized transaction should match the original")
}

func TestTransactionHash(t *testing.T) {
	sender := Address{0x01, 0x02, 0x03, 0x04, 0x05}
	recipient := Address{0x06, 0x07, 0x08, 0x09, 0x0A}
	amount := uint64(100)
	fee := uint64(1)
	nonce := uint64(1)
	payload := []byte("Payload")
	tx, err := NewTransaction(sender, recipient, amount, fee, nonce, payload)
	require.NoError(t, err)

	hash := tx.ComputeHash()
	assert.Equal(t, hash, tx.ID, "Computed hash should match the transaction ID")
}
