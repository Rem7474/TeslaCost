package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := "test-secret-key-12345"
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	secretToken := "tm_api_secret_token_abcdef1234567890"

	encrypted, err := enc.Encrypt(secretToken)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if encrypted == secretToken {
		t.Fatalf("Encrypted text should not equal plain text")
	}

	decrypted, err := enc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if decrypted != secretToken {
		t.Fatalf("Expected decrypted text '%s', got '%s'", secretToken, decrypted)
	}
}

func TestEmptyValues(t *testing.T) {
	enc, err := NewEncryptor("key")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	encrypted, err := enc.Encrypt("")
	if err != nil || encrypted != "" {
		t.Fatalf("Empty string should return empty string, got '%s'", encrypted)
	}

	decrypted, err := enc.Decrypt("")
	if err != nil || decrypted != "" {
		t.Fatalf("Empty string should return empty string, got '%s'", decrypted)
	}
}
