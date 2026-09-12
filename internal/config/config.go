package config

import (
	"os"
	"strconv"
	"strings"
)

// Config stores application configuration loaded from environment variables.
type Config struct {
	Port               string
	AppBaseURL         string
	Environment        string
	DatabaseURL        string
	AppEncryptionKey   string
	JWTSecret          string
	JWTExpirationHours int
	AllowedOrigins     []string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := getEnv("PORT", "8080")
	appBaseURL := getEnv("APP_BASE_URL", "http://localhost:8080")
	env := getEnv("ENVIRONMENT", "development")
	dbURL := getEnv("DATABASE_URL", "postgres://teslacost:teslacost_dev_secret@localhost:5432/teslacost?sslmode=disable")
	encKey := getEnv("APP_ENCRYPTION_KEY", "dev-default-32-byte-secret-key!!")
	jwtSecret := getEnv("JWT_SECRET", "super_secret_jwt_signing_key_for_teslacost_app")
	jwtExpHours, _ := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "72"))
	if jwtExpHours <= 0 {
		jwtExpHours = 72
	}

	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:8080")
	var allowedOrigins []string
	for _, origin := range strings.Split(originsRaw, ",") {
		o := strings.TrimSpace(origin)
		if o != "" {
			allowedOrigins = append(allowedOrigins, o)
		}
	}

	return &Config{
		Port:               port,
		AppBaseURL:         appBaseURL,
		Environment:        env,
		DatabaseURL:        dbURL,
		AppEncryptionKey:   encKey,
		JWTSecret:          jwtSecret,
		JWTExpirationHours: jwtExpHours,
		AllowedOrigins:     allowedOrigins,
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}
