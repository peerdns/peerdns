// pkg/types/transaction_test.go
package types

import (
	"bytes"
	"testing"
	"time"

	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionCreation(t *testing.T) {
	tests := []struct {
		name        string
		sender      [AddressSize]byte
		recipient   [AddressSize]byte
		amount      uint64
		fee         uint64
		nonce       uint64
		payload     []byte
		expectError bool
	}{
		{
			name:        "Valid Transaction with payload",
			sender:      generate32Bytes("sender_address_123456789012"),
			recipient:   generate32Bytes("recipient_address_12345"),
			amount:      1000,
			fee:         10,
			nonce:       1,
			payload:     []byte("Test payload"),
			expectError: false,
		},
		{
			name:        "Valid Transaction with empty payload",
			sender:      generate32Bytes("sender_address_123456789012"),
			recipient:   generate32Bytes("recipient_address_12345"),
			amount:      500,
			fee:         5,
			nonce:       2,
			payload:     []byte{},
			expectError: false,
		},
		{
			name:        "Invalid Transaction with oversized payload",
			sender:      generate32Bytes("sender_address_123456789012"),
			recipient:   generate32Bytes("recipient_address_12345"),
			amount:      1500,
			fee:         15,
			nonce:       3,
			payload:     bytes.Repeat([]byte("a"), MaximumPayloadSize+1),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := NewTransaction(tt.sender, tt.recipient, tt.amount, tt.fee, tt.nonce, tt.payload)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, tx)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tx)
				assert.Equal(t, tt.amount, tx.Amount)
				assert.Equal(t, tt.fee, tx.Fee)
				assert.Equal(t, tt.nonce, tx.Nonce)
				assert.Equal(t, tt.payload, tx.Payload)
				assert.WithinDuration(t, time.Now(), time.Unix(tx.Timestamp, 0), time.Second, "Timestamp should be close to current time")
				// Verify that ID is correctly computed
				hash := tx.computeHash()
				assert.Equal(t, hash, tx.ID, "Transaction ID should match computed hash")
			}
		})
	}
}

func TestTransactionSerialization(t *testing.T) {
	// Initialize BLS
	err := encryption.InitBLS()
	require.NoError(t, err, "Failed to initialize BLS")

	tests := []struct {
		name        string
		transaction Transaction
		expectError bool
	}{
		{
			name: "Serialize and Deserialize Valid Transaction",
			transaction: Transaction{
				ID:        [HashSize]byte{1, 2, 3},
				Sender:    generate32Bytes("sender_address_123456789012"),
				Recipient: generate32Bytes("recipient_address_12345"),
				Amount:    1000,
				Fee:       10,
				Nonce:     1,
				Timestamp: 1625097600,
				Signature: [SignatureSize]byte{4, 5, 6},
				Payload:   []byte("Test payload"),
			},
			expectError: false,
		},
		{
			name: "Serialize Transaction with Empty Payload",
			transaction: Transaction{
				ID:        [HashSize]byte{7, 8, 9},
				Sender:    generate32Bytes("sender_address_abcdef123456"),
				Recipient: generate32Bytes("recipient_address_xyz789"),
				Amount:    500,
				Fee:       5,
				Nonce:     2,
				Timestamp: 1625097600,
				Signature: [SignatureSize]byte{10, 11, 12},
				Payload:   []byte{},
			},
			expectError: false,
		},
		{
			name: "Deserialize Invalid Transaction Data",
			transaction: Transaction{
				// Intentionally incomplete
				ID:     [HashSize]byte{13, 14, 15},
				Sender: generate32Bytes("sender_incomplete"),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serialized, err := tt.transaction.Serialize()
			if tt.expectError {
				// For invalid data, serialization might still pass, but deserialization should fail
				// Adjust based on your implementation
				// Here, we assume serialization of incomplete data does not return an error
				// Thus, we proceed to attempt deserialization and expect an error
				assert.NoError(t, err, "Serialization should not fail even for incomplete data")
			} else {
				assert.NoError(t, err, "Serialization should not fail")
			}

			deserializedTx, err := DeserializeTransaction(serialized)
			if tt.expectError {
				assert.Error(t, err, "Deserialization should fail for incomplete data")
				return
			}
			require.NoError(t, err, "Deserialization should not fail")
			require.NotNil(t, deserializedTx, "Deserialized transaction should not be nil")

			assert.Equal(t, tt.transaction.ID, deserializedTx.ID, "IDs should match")
			assert.Equal(t, tt.transaction.Sender, deserializedTx.Sender, "Senders should match")
			assert.Equal(t, tt.transaction.Recipient, deserializedTx.Recipient, "Recipients should match")
			assert.Equal(t, tt.transaction.Amount, deserializedTx.Amount, "Amounts should match")
			assert.Equal(t, tt.transaction.Fee, deserializedTx.Fee, "Fees should match")
			assert.Equal(t, tt.transaction.Nonce, deserializedTx.Nonce, "Nonces should match")
			assert.Equal(t, tt.transaction.Timestamp, deserializedTx.Timestamp, "Timestamps should match")
			assert.Equal(t, tt.transaction.Signature, deserializedTx.Signature, "Signatures should match")
			assert.Equal(t, tt.transaction.Payload, deserializedTx.Payload, "Payloads should match")
		})
	}
}

func TestTransactionSigningAndVerification(t *testing.T) {
	// Initialize BLS
	err := encryption.InitBLS()
	require.NoError(t, err, "Failed to initialize BLS")

	// Generate BLS keys
	privKey, pubKey, err := encryption.GenerateBLSKeys()
	require.NoError(t, err, "Failed to generate BLS keys")

	tests := []struct {
		name            string
		transaction     Transaction
		modifySignature bool
		expectValid     bool
	}{
		{
			name: "Valid Signature",
			transaction: Transaction{
				Sender:    generate32Bytes("sender_valid_signature"),
				Recipient: generate32Bytes("recipient_valid_sig"),
				Amount:    2000,
				Fee:       20,
				Nonce:     3,
				Timestamp: 1625097600,
				Payload:   []byte("Valid signature payload"),
			},
			modifySignature: false,
			expectValid:     true,
		},
		{
			name: "Invalid Signature",
			transaction: Transaction{
				Sender:    generate32Bytes("sender_invalid_sig"),
				Recipient: generate32Bytes("recipient_invalid"),
				Amount:    3000,
				Fee:       30,
				Nonce:     4,
				Timestamp: 1625097601,
				Payload:   []byte("Invalid signature payload"),
			},
			modifySignature: true,
			expectValid:     false,
		},
		{
			name: "Unsigned Transaction",
			transaction: Transaction{
				Sender:    generate32Bytes("sender_unsigned"),
				Recipient: generate32Bytes("recipient_unsigned"),
				Amount:    4000,
				Fee:       40,
				Nonce:     5,
				Timestamp: 1625097602,
				Payload:   []byte("Unsigned payload"),
			},
			modifySignature: false,
			expectValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy to avoid modifying the original test case
			tx := tt.transaction

			// Sign the transaction if not testing an unsigned transaction
			if tt.name != "Unsigned Transaction" {
				err := tx.Sign(privKey)
				require.NoError(t, err, "Failed to sign transaction")
			}

			// Optionally modify the signature to simulate tampering
			if tt.modifySignature && len(tx.Signature) > 0 {
				tx.Signature[0] ^= 0xFF // Flip some bits
			}

			// Verify the signature
			valid, err := tx.Verify(pubKey)
			if tt.expectValid {
				require.NoError(t, err, "Verification should not fail")
				assert.True(t, valid, "Signature should be valid")
			} else {
				// Either verification fails or there is an error
				if tt.name == "Unsigned Transaction" {
					assert.False(t, valid, "Unsigned transaction should not be valid")
				} else {
					if err != nil {
						assert.Error(t, err, "Expected an error during verification")
					} else {
						assert.False(t, valid, "Signature should be invalid")
					}
				}
			}
		})
	}
}
