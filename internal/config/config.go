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
	JWTSecret                  string
	JWTExpirationHours         int
	JWTAccessExpirationMinutes int
	JWTRefreshExpirationDays   int
	CookieSecure               bool
	DisableRegistration        bool
	InitialAdminEmail          string
	InitialAdminPassword       string
	AllowedOrigins             []string
	SyncIntervalMinutes        int
	ReportingTimezone          string
	StorageDir                 string // Directory for document file storage (Docker volume mount point)


	// OIDC / OAuth2 SSO (optional — enabled when OIDCIssuerURL is non-empty)
	OIDCEnabled          bool
	OIDCIssuerURL        string   // e.g. https://auth.homelab.local/application/o/teslacost/
	OIDCClientID         string
	OIDCClientSecret     string
	OIDCRedirectURL      string   // e.g. https://teslacost.homelab.local/api/auth/oidc/callback
	OIDCScopes           []string // default: ["openid", "email", "profile"]
	OIDCProviderName     string   // label shown in the UI, e.g. "Authentik"
	OIDCAllowedEmails    []string // optional whitelist; empty = allow all
	OIDCDisableLocalAuth bool     // when true, /login and /register endpoints are disabled
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
		dbPass := getEnv("DB_PASSWORD", "")
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
		// No credentials in code: the password comes from DATABASE_URL, DB_PASSWORD or PGPASSWORD.
		rawURL := getEnv("DATABASE_URL", "postgres://teslacost@localhost:5432/teslacost?sslmode=disable")
		dbURL = NormalizeDatabaseURL(rawURL)
	}

	encKey := getEnv("APP_ENCRYPTION_KEY", "dev-default-32-byte-secret-key!!")
	jwtSecret := getEnv("JWT_SECRET", "super_secret_jwt_signing_key_for_teslacost_app")
	jwtExpHours, _ := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "72"))
	if jwtExpHours <= 0 {
		jwtExpHours = 72
	}

	jwtAccessExpMinutes, _ := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRATION_MINUTES", "15"))
	if jwtAccessExpMinutes <= 0 {
		jwtAccessExpMinutes = 15
	}

	jwtRefreshExpDays, _ := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRATION_DAYS", "30"))
	if jwtRefreshExpDays <= 0 {
		jwtRefreshExpDays = 30
	}

	// Default CookieSecure to true in production or if appBaseURL uses https
	defaultCookieSecure := strings.EqualFold(env, "production") || strings.HasPrefix(strings.ToLower(appBaseURL), "https://")
	cookieSecure := getEnvBool("COOKIE_SECURE", defaultCookieSecure)

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

	syncIntervalMinutes, _ := strconv.Atoi(getEnv("SYNC_INTERVAL_MINUTES", "30"))
	if syncIntervalMinutes < 0 {
		syncIntervalMinutes = 0
	}

	reportingTimezone := getEnv("APP_TIMEZONE", "Europe/Paris")
	storageDir := getEnv("STORAGE_DIR", "./data/documents")

	// OIDC configuration
	oidcIssuerURL := getEnv("OIDC_ISSUER_URL", "")
	oidcClientID := getEnv("OIDC_CLIENT_ID", "")
	oidcClientSecret := getEnv("OIDC_CLIENT_SECRET", "")
	oidcRedirectURL := getEnv("OIDC_REDIRECT_URL", "")
	oidcProviderName := getEnv("OIDC_PROVIDER_NAME", "SSO")
	oidcDisableLocalAuth := getEnvBool("OIDC_DISABLE_LOCAL_AUTH", false)

	var oidcScopes []string
	if scopesRaw := getEnv("OIDC_SCOPES", "openid email profile"); scopesRaw != "" {
		for _, s := range strings.Split(scopesRaw, " ") {
			if s = strings.TrimSpace(s); s != "" {
				oidcScopes = append(oidcScopes, s)
			}
		}
	}

	var oidcAllowedEmails []string
	if emailsRaw := getEnv("OIDC_ALLOWED_EMAILS", ""); emailsRaw != "" {
		for _, e := range strings.Split(emailsRaw, ",") {
			if e = strings.TrimSpace(e); e != "" {
				oidcAllowedEmails = append(oidcAllowedEmails, e)
			}
		}
	}

	return &Config{
		Port:                       port,
		AppBaseURL:                 appBaseURL,
		Environment:                env,
		DatabaseURL:                dbURL,
		AppEncryptionKey:           encKey,
		JWTSecret:                  jwtSecret,
		JWTExpirationHours:         jwtExpHours,
		JWTAccessExpirationMinutes: jwtAccessExpMinutes,
		JWTRefreshExpirationDays:   jwtRefreshExpDays,
		CookieSecure:               cookieSecure,
		DisableRegistration:        disableRegistration,
		InitialAdminEmail:          initialAdminEmail,
		InitialAdminPassword:       initialAdminPassword,
		AllowedOrigins:             allowedOrigins,
		SyncIntervalMinutes:        syncIntervalMinutes,
		ReportingTimezone:          reportingTimezone,
		StorageDir:                 storageDir,
		OIDCEnabled:                oidcIssuerURL != "",
		OIDCIssuerURL:              oidcIssuerURL,
		OIDCClientID:               oidcClientID,
		OIDCClientSecret:           oidcClientSecret,
		OIDCRedirectURL:            oidcRedirectURL,
		OIDCScopes:                 oidcScopes,
		OIDCProviderName:           oidcProviderName,
		OIDCAllowedEmails:          oidcAllowedEmails,
		OIDCDisableLocalAuth:       oidcDisableLocalAuth,
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
