package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr               string
	DatabaseURL        string
	SessionCookieName  string
	SessionTTL         time.Duration
	SecureCookies      bool
	CSRFEnforce        bool
	CORSAllowedOrigins []string
	Env                string
	APIMaxBodyBytes    int64
	ImportMaxFileBytes int64
	ImportMaxRows      int
	ReadHeaderTimeout  time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	RateLimitMaxIPs    int
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Addr:              getEnv("API_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SessionCookieName: getEnv("SESSION_COOKIE_NAME", "mo_sess"),
		SessionTTL:        time.Duration(getEnvInt("SESSION_TTL_HOURS", 12)) * time.Hour,
		SecureCookies:     getEnvBool("COOKIE_SECURE", false),
		CSRFEnforce:       getEnvBool("CSRF_ENFORCE", true),
		CORSAllowedOrigins: getEnvCSV("CORS_ALLOWED_ORIGINS", []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		}),
		Env:                getEnv("APP_ENV", "dev"),
		APIMaxBodyBytes:    int64(getEnvInt("API_MAX_BODY_MB", 2)) * 1024 * 1024,
		ImportMaxFileBytes: int64(getEnvInt("IMPORT_MAX_FILE_MB", 25)) * 1024 * 1024,
		ImportMaxRows:      getEnvInt("IMPORT_MAX_ROWS", 5000),
		ReadHeaderTimeout:  time.Duration(getEnvInt("API_READ_HEADER_TIMEOUT_SEC", 5)) * time.Second,
		ReadTimeout:        time.Duration(getEnvInt("API_READ_TIMEOUT_SEC", 15)) * time.Second,
		WriteTimeout:       time.Duration(getEnvInt("API_WRITE_TIMEOUT_SEC", 30)) * time.Second,
		IdleTimeout:        time.Duration(getEnvInt("API_IDLE_TIMEOUT_SEC", 60)) * time.Second,
		RateLimitMaxIPs:    getEnvInt("RATE_LIMIT_MAX_IPS", 10000),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.Env == "prod" {
		cfg.SecureCookies = true
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvCSV(key string, fallback []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
