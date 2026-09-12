package auth

import (
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecurePassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !CheckPassword(password, hash) {
		t.Fatalf("Password verification failed for valid password")
	}

	if CheckPassword("WrongPassword", hash) {
		t.Fatalf("Password verification succeeded for invalid password")
	}

	_, err = HashPassword("short")
	if err != ErrPasswordTooShort {
		t.Fatalf("Expected ErrPasswordTooShort, got %v", err)
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	secret := "secret-jwt-key"
	userID := "user-uuid-12345"
	email := "user@example.com"

	token, err := GenerateToken(userID, email, secret, 24)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID || claims.Email != email {
		t.Fatalf("Token claims mismatch: got user %s, email %s", claims.UserID, claims.Email)
	}

	// Validate with wrong secret
	_, err = ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Fatalf("Token validation should have failed with wrong secret")
	}
}
