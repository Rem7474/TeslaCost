package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

var (
	ErrInvalidCiphertext = errors.New("invalid or corrupted ciphertext")
	ErrEmptyKey          = errors.New("encryption key cannot be empty")
)

// Encryptor handles AES-256-GCM encryption and decryption.
type Encryptor struct {
	key []byte
}

// NewEncryptor creates an Encryptor using the provided secret key.
// The key is derived to 32 bytes using SHA-256 to ensure a valid 256-bit AES key.
func NewEncryptor(secretKey string) (*Encryptor, error) {
	if secretKey == "" {
		return nil, ErrEmptyKey
	}
	hash := sha256.Sum256([]byte(secretKey))
	return &Encryptor{key: hash[:]}, nil
}

// Encrypt encrypts plain text into a base64 encoded ciphertext string using AES-GCM.
func (e *Encryptor) Encrypt(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Seal appends encrypted data and tag to nonce
	sealed := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64 encoded ciphertext string back to plain text.
func (e *Encryptor) Decrypt(cipherTextBase64 string) (string, error) {
	if cipherTextBase64 == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(cipherTextBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plainTextBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	return string(plainTextBytes), nil
}
