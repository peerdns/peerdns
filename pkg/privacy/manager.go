// pkg/privacy/privacy_manager.go
package privacy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"github.com/peerdns/peerdns/pkg/identity"
)

// PrivacyManager handles encryption and decryption of messages.
type PrivacyManager struct {
	key []byte // AES-256 key
}

// NewPrivacyManager initializes a new PrivacyManager with a generated AES-256 key.
func NewPrivacyManager() (*PrivacyManager, error) {
	key := make([]byte, 32) // AES-256
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	return &PrivacyManager{
		key: key,
	}, nil
}

// Encrypt encrypts the given plaintext using AES-256-GCM.
func (pm *PrivacyManager) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(pm.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES-GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts the given ciphertext using AES-256-GCM.
func (pm *PrivacyManager) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(pm.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES-GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ciphertext: %w", err)
	}

	return plaintext, nil
}

// EncryptMessage encrypts a message for a specific DID using their public key.
func (pm *PrivacyManager) EncryptMessage(recipient *identity.DID, message []byte) ([]byte, error) {
	// For simplicity, use symmetric encryption.
	// In production, consider using asymmetric encryption or hybrid encryption.
	return pm.Encrypt(message)
}

// DecryptMessage decrypts a message received from a specific DID using their private key.
func (pm *PrivacyManager) DecryptMessage(sender *identity.DID, ciphertext []byte) ([]byte, error) {
	// For simplicity, use symmetric decryption.
	// In production, consider using asymmetric encryption or hybrid encryption.
	return pm.Decrypt(ciphertext)
}
