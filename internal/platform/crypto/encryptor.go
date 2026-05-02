package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with a random salt and nonce.
// The masterKey should be a 32-byte string (or longer; it is used directly if exactly 32 bytes,
// otherwise it is truncated/padded — but caller should provide a 32-byte key).
// Output format: base64(salt + nonce + ciphertext + tag)
func Encrypt(plaintext string, masterKey string) (string, error) {
	key := deriveKey(masterKey)

	// Generate random salt (16 bytes)
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	// Create AES block cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), salt)
	// ciphertext now contains nonce + encrypted data + tag

	// Prepend salt
	data := append(salt, ciphertext...)

	return base64.StdEncoding.EncodeToString(data), nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
// The masterKey must be the same key used during encryption.
func Decrypt(ciphertext string, masterKey string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	if len(data) < 16 {
		return "", fmt.Errorf("ciphertext too short")
	}

	key := deriveKey(masterKey)

	// Extract salt (first 16 bytes)
	salt := data[:16]
	data = data[16:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short for nonce")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, salt)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// deriveKey ensures the key is exactly 32 bytes for AES-256.
// If the key is shorter, it is padded with zeros.
// If longer, it is truncated.
func deriveKey(masterKey string) []byte {
	key := make([]byte, 32)
	copy(key, masterKey)
	return key
}
