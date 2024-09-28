// pkg/encryption/encryption_test.go
package encryption

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBLSKeys(t *testing.T) {
	// Initialize BLS
	err := InitBLS()
	require.NoError(t, err, "Failed to initialize BLS")

	// Generate keys
	privKey, pubKey, err := GenerateBLSKeys()
	require.NoError(t, err, "Failed to generate BLS keys")
	assert.NotNil(t, privKey, "Private key should not be nil")
	assert.NotNil(t, pubKey, "Public key should not be nil")

	// Sign data
	data := []byte("test data for signing")
	signature, err := Sign(data, privKey)
	require.NoError(t, err, "Failed to sign data")
	assert.NotNil(t, signature, "Signature should not be nil")

	// Verify signature
	valid := Verify(data, signature, pubKey)
	assert.True(t, valid, "Signature should be valid")

	// Tamper data
	tamperedData := []byte("tampered data")
	valid = Verify(tamperedData, signature, pubKey)
	assert.False(t, valid, "Signature should be invalid for tampered data")
}
