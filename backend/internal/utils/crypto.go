package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
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

// AESKeyFromSecret derives a 32-byte AES key from the JWT secret by taking
// the first 32 bytes of the secret. Caller must ensure the secret is at
// least 32 bytes before calling this.
func AESKeyFromSecret(jwtSecret string) []byte {
	if len(jwtSecret) < 32 {
		// Pad or handle — but per our design, JWT_SECRET is always >= 32 bytes.
		// This is a safety fallback; we pad with zeros.
		key := make([]byte, 32)
		copy(key, jwtSecret)
		return key
	}
	return []byte(jwtSecret[:32])
}
