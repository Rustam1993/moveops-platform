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
		Env: getEnv("APP_ENV", "dev"),
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
