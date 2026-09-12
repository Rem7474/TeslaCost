package config

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config stores application configuration loaded from environment variables.
type Config struct {
	Port                 string
	AppBaseURL           string
	Environment          string
	DatabaseURL          string
	AppEncryptionKey     string
	JWTSecret            string
	JWTExpirationHours   int
	DisableRegistration  bool
	InitialAdminEmail    string
	InitialAdminPassword string
	AllowedOrigins       []string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := getEnv("PORT", "8080")
	appBaseURL := getEnv("APP_BASE_URL", "http://localhost:8080")
	env := getEnv("ENVIRONMENT", "development")

	// Determine database URL:
	// If DB_HOST is set, build a safely URL-encoded connection string (preventing issues with special characters in passwords).
	var dbURL string
	if dbHost := getEnv("DB_HOST", ""); dbHost != "" {
		dbPort := getEnv("DB_PORT", "5432")
		dbUser := getEnv("DB_USER", "teslacost")
		dbPass := getEnv("DB_PASSWORD", "teslacost_dev_secret")
		dbName := getEnv("DB_NAME", "teslacost")
		dbSSL := getEnv("DB_SSLMODE", "disable")

		u := &url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(dbUser, dbPass),
			Host:     net.JoinHostPort(dbHost, dbPort),
			Path:     "/" + dbName,
			RawQuery: "sslmode=" + dbSSL,
		}
		dbURL = u.String()
	} else {
		rawURL := getEnv("DATABASE_URL", "postgres://teslacost:teslacost_dev_secret@localhost:5432/teslacost?sslmode=disable")
		dbURL = NormalizeDatabaseURL(rawURL)
	}

	encKey := getEnv("APP_ENCRYPTION_KEY", "dev-default-32-byte-secret-key!!")
	jwtSecret := getEnv("JWT_SECRET", "super_secret_jwt_signing_key_for_teslacost_app")
	jwtExpHours, _ := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "72"))
	if jwtExpHours <= 0 {
		jwtExpHours = 72
	}

	disableRegistration := getEnvBool("DISABLE_REGISTRATION", false)
	initialAdminEmail := getEnv("INITIAL_ADMIN_EMAIL", "")
	initialAdminPassword := getEnv("INITIAL_ADMIN_PASSWORD", "")

	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:8080")
	var allowedOrigins []string
	for _, origin := range strings.Split(originsRaw, ",") {
		o := strings.TrimSpace(origin)
		if o != "" {
			allowedOrigins = append(allowedOrigins, o)
		}
	}

	return &Config{
		Port:                 port,
		AppBaseURL:           appBaseURL,
		Environment:          env,
		DatabaseURL:          dbURL,
		AppEncryptionKey:     encKey,
		JWTSecret:            jwtSecret,
		JWTExpirationHours:   jwtExpHours,
		DisableRegistration:  disableRegistration,
		InitialAdminEmail:    initialAdminEmail,
		InitialAdminPassword: initialAdminPassword,
		AllowedOrigins:       allowedOrigins,
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	val, exists := os.LookupEnv(key)
	if !exists || val == "" {
		return defaultVal
	}
	val = strings.ToLower(strings.TrimSpace(val))
	return val == "true" || val == "1" || val == "yes"
}

// NormalizeDatabaseURL ensures that any special characters in the password component
// of a database URL (like '@', '&', '!', '#', '%') are properly percent-encoded,
// preventing host parsing errors in libpq/pgx when passwords contain '@'.
func NormalizeDatabaseURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return rawURL
	}

	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd == -1 {
		return rawURL
	}
	scheme := rawURL[:schemeEnd]
	rest := rawURL[schemeEnd+3:]

	authEnd := len(rest)
	if idx := strings.IndexAny(rest, "/?"); idx != -1 {
		authEnd = idx
	}
	authority := rest[:authEnd]
	pathAndQuery := rest[authEnd:]

	lastAt := strings.LastIndex(authority, "@")
	if lastAt == -1 {
		return rawURL
	}

	userInfo := authority[:lastAt]
	hostPort := authority[lastAt+1:]

	colonIdx := strings.Index(userInfo, ":")
	if colonIdx == -1 {
		return rawURL
	}

	user := userInfo[:colonIdx]
	pass := userInfo[colonIdx+1:]

	u := &url.URL{
		Scheme: scheme,
		User:   url.UserPassword(user, pass),
		Host:   hostPort,
	}

	return u.String() + pathAndQuery
}
