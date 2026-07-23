package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// EncryptAES encrypts plaintext using AES-256-GCM with the given key.
// The key must be 32 bytes (AES-256). Returns the base64-encoded ciphertext
// (nonce + encrypted data, both binary but encoded together as base64).
func EncryptAES(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", errors.New("AES key must be exactly 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAES decrypts a base64-encoded ciphertext produced by EncryptAES.
func DecryptAES(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", errors.New("AES key must be exactly 32 bytes")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GenerateRandomBytes generates n random bytes using crypto/rand.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// AESKeyFromSecret derives a 32-byte AES key from the JWT secret.
// JWT_SECRET is stored as a base64-encoded string of 32 random bytes.
// We base64-decode it to recover the original 32 bytes for full entropy.
// Falls back to taking the first 32 raw bytes if decoding fails (legacy compat).
func AESKeyFromSecret(jwtSecret string) []byte {
	// Try base64 decoding to recover the original random bytes (full entropy)
	if decoded, err := base64.RawURLEncoding.DecodeString(jwtSecret); err == nil && len(decoded) >= 32 {
		return decoded[:32]
	}
	// Fallback for legacy or non-base64 secrets: use first 32 bytes of raw string.
	key := make([]byte, 32)
	copy(key, jwtSecret)
	return key
}

// GenerateUUID creates a random UUID v4 string.
func GenerateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// GenerateToken creates a random 32-byte token encoded as base64url.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
