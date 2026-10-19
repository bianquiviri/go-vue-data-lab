package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Default JWT lifetimes used when the corresponding environment variables are
// absent or invalid. Access tokens are short-lived; refresh tokens last longer.
const (
	DefaultJWTAccessTTL  = 15 * time.Minute
	DefaultJWTRefreshTTL = 7 * 24 * time.Hour
)

var (
	// ErrInvalidSecureIDKey is returned when SECURE_ID_KEY is missing or invalid.
	ErrInvalidSecureIDKey = errors.New("config: invalid or missing SECURE_ID_KEY (must be 32 bytes, base64 or hex)")
)

type Config struct {
	DBHost                 string
	DBPort                 string
	DBUser                 string
	DBPassword             string
	DBSourceName           string
	DBTargetName           string
	ServerPort             string
	ServerHost             string
	AppEnv                 string
	AppURL                 string
	JWTSecret              string
	JWTExpiryHours         int
	JWTAccessTTL           time.Duration
	JWTRefreshTTL          time.Duration
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	LogLevel               string
	LogPath                string
	CORSAllowedOrigins     []string
	SecureIDKey            []byte
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "postgres"),
		DBPassword:             getEnv("DB_PASSWORD", "postgres"),
		DBSourceName:           getEnv("DB_SOURCE_NAME", "lab_source"),
		DBTargetName:           getEnv("DB_TARGET_NAME", "lab_target"),
		ServerPort:             getEnv("SERVER_PORT", "8080"),
		ServerHost:             getEnv("SERVER_HOST", "0.0.0.0"),
		AppEnv:                 getEnv("APP_ENV", "development"),
		AppURL:                 getEnv("APP_URL", "https://godata.josebianco.local"),
		JWTSecret:              getEnv("JWT_SECRET", "your_jwt_secret_here_change_in_production"),
		BootstrapAdminEmail:    getEnv("BOOTSTRAP_ADMIN_EMAIL", "root@gmail.com"),
		BootstrapAdminPassword: getEnv("BOOTSTRAP_ADMIN_PASSWORD", "change_me_123!"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		LogPath:                getEnv("LOG_PATH", "logs/app.log"),
	}

	jwtExpiry := getEnv("JWT_EXPIRY_HOURS", "24")
	if h, err := strconv.Atoi(jwtExpiry); err == nil {
		cfg.JWTExpiryHours = h
	} else {
		cfg.JWTExpiryHours = 24
	}

	cfg.JWTAccessTTL = getEnvDuration("JWT_ACCESS_TTL", DefaultJWTAccessTTL)
	cfg.JWTRefreshTTL = getEnvDuration("JWT_REFRESH_TTL", DefaultJWTRefreshTTL)

	cfg.CORSAllowedOrigins = parseOrigins(getEnv("CORS_ALLOWED_ORIGINS", ""), cfg.AppURL)

	key, err := parseSecureIDKey(getEnv("SECURE_ID_KEY", ""))
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg.SecureIDKey = key

	return cfg, nil
}

// parseOrigins resolves the allowed CORS origins: an explicit comma-separated
// env value wins; otherwise the app URL plus the local dev-server origins are
// allowed. Duplicates are removed while preserving order.
func parseOrigins(raw, appURL string) []string {
	var input string
	if raw != "" {
		input = raw
	} else {
		input = strings.Join([]string{
			appURL,
			"http://localhost:5173",
			"http://127.0.0.1:5173",
		}, ",")
	}

	seen := make(map[string]bool)
	out := make([]string, 0, 4)
	for _, part := range strings.Split(input, ",") {
		origin := strings.TrimSpace(part)
		if origin == "" || seen[origin] {
			continue
		}
		seen[origin] = true
		out = append(out, origin)
	}
	return out
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvDuration parses a duration environment variable (e.g. "15m", "168h").
// It falls back to the provided default when the variable is unset or invalid.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

// parseSecureIDKey parses SECURE_ID_KEY as base64 or hex (32 bytes).
func parseSecureIDKey(v string) ([]byte, error) {
	if strings.TrimSpace(v) == "" {
		return nil, ErrInvalidSecureIDKey
	}
	s := strings.TrimSpace(v)
	// Try hex first if clean hex chars
	if isHex(s) {
		b, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("%w", ErrInvalidSecureIDKey)
		}
		if len(b) != 32 {
			return nil, ErrInvalidSecureIDKey
		}
		return b, nil
	}
	// Try base64 (standard)
	b, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		if len(b) != 32 {
			return nil, ErrInvalidSecureIDKey
		}
		return b, nil
	}
	// Try base64 raw URL if needed? standard expected; but be flexible? spec says standard
	return nil, ErrInvalidSecureIDKey
}

func isHex(s string) bool {
	if len(s)%2 != 0 {
		return false
	}
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}
