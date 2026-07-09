package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
// Values are loaded from environment variables with sensible defaults for dev
type Config struct {
	// Server
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Database
	DatabaseURL string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret string
	JWTExpiry time.Duration

	// Security
	CORSAllowedOrigins []string

	// Feature flags
	EnableWS    bool
	EnableAudit bool
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		ReadTimeout:  parseDuration("SERVER_READ_TIMEOUT", "15s"),
		WriteTimeout: parseDuration("SERVER_WRITE_TIMEOUT", "15s"),
		IdleTimeout:  parseDuration("SERVER_IDLE_TIMEOUT", "60s"),

		DatabaseURL: getEnv("POSTGRES_URL", "control://control:postgres@localhost:5432/control?sslmode=disable"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       parseInt("REDIS_DB", 0),

		JWTSecret: getEnv("JWT_SECRET", ""),
		JWTExpiry: parseDuration("JWT_EXPIRY", "24h"),

		EnableWS:    parseBool("ENABLE_WEBSOCKET", true),
		EnableAudit: parseBool("ENABLE_AUDIT", true),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(key, fallback string) time.Duration {
	s := getEnv(key, fallback)
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(fmt.Sprintf("invalid duration for %s: %s", key, s))
	}
	return d
}

func parseInt(key string, fallback int) int {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Sprintf("invalid int for %s: %s", key, s))
	}
	return v
}

func parseBool(key string, fallback bool) bool {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		panic(fmt.Sprintf("invalid bool for %s: %s", key, s))
	}
	return v
}
