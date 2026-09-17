package config_test

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teslacost/teslacost/internal/config"
)

func TestSpecialCharactersInPassword(t *testing.T) {
	os.Setenv("DB_HOST", "postgres")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "teslacost")
	os.Setenv("DB_PASSWORD", "ComplexP@ss!&M#123")
	os.Setenv("DB_NAME", "teslacost")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
	}()

	cfg := config.Load()

	// Parse with pgxpool to verify that pgx parses the host and password correctly
	pgxCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("Failed to parse DatabaseURL: %v", err)
	}

	if pgxCfg.ConnConfig.Host != "postgres" {
		t.Errorf("Expected host to be 'postgres', got '%s'", pgxCfg.ConnConfig.Host)
	}
	if pgxCfg.ConnConfig.Password != "ComplexP@ss!&M#123" {
		t.Errorf("Expected password to be 'ComplexP@ss!&M#123', got '%s'", pgxCfg.ConnConfig.Password)
	}
	if pgxCfg.ConnConfig.User != "teslacost" {
		t.Errorf("Expected user to be 'teslacost', got '%s'", pgxCfg.ConnConfig.User)
	}
}

func TestInsecureDefaultsDetectsKnownPlaceholders(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:        "super_secret_jwt_signing_key_for_teslacost_app",
		AppEncryptionKey: "generate_a_random_32_characters_key_here!",
		DatabaseURL:      "postgres://teslacost:teslacost_dev_secret@postgres:5432/teslacost?sslmode=disable",
	}

	warnings := cfg.InsecureDefaults()
	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings for JWT secret, encryption key and DB password, got %d: %v", len(warnings), warnings)
	}
}

func TestInsecureDefaultsClearOnCustomSecrets(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:        "a-real-random-secret",
		AppEncryptionKey: "another-real-random-key-value-xx",
		DatabaseURL:      "postgres://teslacost:S0m3R34lPassw0rd@postgres:5432/teslacost?sslmode=disable",
	}

	if warnings := cfg.InsecureDefaults(); len(warnings) != 0 {
		t.Fatalf("expected no warnings for custom secrets, got: %v", warnings)
	}
}

func TestNormalizeDatabaseURLWithUnescapedPassword(t *testing.T) {
	raw := "postgres://teslacost:MyPasswordM&!@123@postgres:5432/teslacost?sslmode=disable"
	normalized := config.NormalizeDatabaseURL(raw)

	pgxCfg, err := pgxpool.ParseConfig(normalized)
	if err != nil {
		t.Fatalf("Failed to parse normalized DatabaseURL: %v", err)
	}

	if pgxCfg.ConnConfig.Host != "postgres" {
		t.Errorf("Expected host to be 'postgres', got '%s'", pgxCfg.ConnConfig.Host)
	}
	if pgxCfg.ConnConfig.Password != "MyPasswordM&!@123" {
		t.Errorf("Expected password to be 'MyPasswordM&!@123', got '%s'", pgxCfg.ConnConfig.Password)
	}
	if pgxCfg.ConnConfig.User != "teslacost" {
		t.Errorf("Expected user to be 'teslacost', got '%s'", pgxCfg.ConnConfig.User)
	}
}
