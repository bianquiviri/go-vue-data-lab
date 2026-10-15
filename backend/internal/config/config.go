package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
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
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	LogLevel               string
	LogPath                string
	CORSAllowedOrigins     []string
}

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

	cfg.CORSAllowedOrigins = parseOrigins(getEnv("CORS_ALLOWED_ORIGINS", ""), cfg.AppURL)

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
