package signatures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZKProofSigner_SignAndVerify(t *testing.T) {
	signer, err := NewZKProofSigner()
	require.NoError(t, err, "Failed to initialize ZKProofSigner")

	tests := []struct {
		name        string
		data        []byte
		alterData   bool
		expectValid bool
	}{
		{
			name:        "Valid proof",
			data:        []byte("This is a sample message for signing."),
			alterData:   false,
			expectValid: true,
		},
		{
			name:        "Invalid proof with altered data",
			data:        []byte("This is a sample message for signing."),
			alterData:   true,
			expectValid: false,
		},
		{
			name:        "Empty data",
			data:        []byte{},
			alterData:   false,
			expectValid: true, // Empty data is acceptable in this context
		},
		{
			name:        "Nil data",
			data:        nil,
			alterData:   false,
			expectValid: true, // Nil data is acceptable in this context
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataToSign := tt.data
			if tt.alterData && len(tt.data) > 0 {
				dataToSign = append([]byte{}, tt.data...)
				dataToSign[0] ^= 0xFF // Alter the data
			}

			signature, err := signer.Sign(tt.data)
			if tt.expectValid {
				assert.NoError(t, err, "Expected signing to succeed")
				assert.NotNil(t, signature, "Proof should not be nil")
			} else {
				assert.NoError(t, err, "Expected signing to succeed even with altered data") // Sign doesn't fail on data
				assert.NotNil(t, signature, "Proof should not be nil")
			}

			isValid, err := signer.Verify(dataToSign, signature)
			if tt.expectValid {
				assert.NoError(t, err, "Verification should not error")
				assert.True(t, isValid, "Proof should be valid")
			} else {
				assert.Error(t, err, "Verification should error for invalid proof")
				assert.False(t, isValid, "Proof should be invalid")
			}
		})
	}
}

func TestZKProofSigner_ErrorCases(t *testing.T) {
	t.Run("Initialize with nil keys", func(t *testing.T) {
		_, err := NewZKProofSignerWithKeys(nil, nil)
		assert.Error(t, err, "Should fail to initialize ZKProofSigner with nil keys")
	})

	t.Run("Signing with nil private key", func(t *testing.T) {
		signer, err := NewZKProofSigner()
		require.NoError(t, err, "Failed to initialize ZKProofSigner")
		signer.keyPair.PrivateKey = nil
		_, err = signer.Sign([]byte("data"))
		assert.Error(t, err, "Signing should fail when private key is nil")
	})

	t.Run("Verifying with nil public keys", func(t *testing.T) {
		signer, err := NewZKProofSigner()
		require.NoError(t, err, "Failed to initialize ZKProofSigner")
		signer.keyPair.PublicKeyA = nil
		signer.keyPair.PublicKeyB = nil
		valid, err := signer.Verify([]byte("data"), []byte("signature"))
		assert.Error(t, err, "Verification should fail when public keys are nil")
		assert.False(t, valid, "Verification should return false when public keys are nil")
	})

	t.Run("Verifying with invalid signature", func(t *testing.T) {
		signer, err := NewZKProofSigner()
		require.NoError(t, err, "Failed to initialize ZKProofSigner")
		valid, err := signer.Verify([]byte("data"), []byte("invalid signature"))
		assert.Error(t, err, "Verification should fail with invalid signature")
		assert.False(t, valid, "Verification should return false with invalid signature")
	})
}

func TestZKProofKeySerialization(t *testing.T) {
	signer, err := NewZKProofSigner()
	require.NoError(t, err, "Failed to initialize ZKProofSigner")

	privateKeyBytes, err := signer.keyPair.SerializePrivate()
	require.NoError(t, err, "Failed to serialize private key")
	assert.NotEmpty(t, privateKeyBytes, "Private key bytes should not be empty")

	publicKeyBytes, err := signer.keyPair.SerializePublic()
	require.NoError(t, err, "Failed to serialize public keys")
	assert.NotEmpty(t, publicKeyBytes, "Public key bytes should not be empty")

	newSigner, err := NewZKProofSignerWithKeys(privateKeyBytes, publicKeyBytes)
	require.NoError(t, err, "Failed to initialize ZKProofSigner with serialized keys")

	data := []byte("Test message")
	signature, err := newSigner.Sign(data)
	require.NoError(t, err, "Failed to create DLEQ proof with new signer")

	valid, err := newSigner.Verify(data, signature)
	require.NoError(t, err, "Failed to verify DLEQ proof with new signer")
	assert.True(t, valid, "Proof should be valid")
}
