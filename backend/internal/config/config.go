package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppEnv              string
	Port                string
	PostgresURL         string
	RedisURL            string
	SessionCookieSecure bool
	SessionCookieName   string
	SessionTTLSeconds   int
	CorsAllowedOrigins  []string
}

func LoadConfig() *Config {
	appEnv := getEnv("APP_ENV", "development")
	port := getEnv("PORT", "8080")
	postgresURL := getEnv("POSTGRES_URL", "postgres://pcp_user:pcp_secure_password_2026@localhost:5432/phone_control_platform?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	cookieSecureStr := getEnv("SESSION_COOKIE_SECURE", "false")
	cookieSecure, _ := strconv.ParseBool(cookieSecureStr)

	return &Config{
		AppEnv:              appEnv,
		Port:                port,
		PostgresURL:         postgresURL,
		RedisURL:            redisURL,
		SessionCookieSecure: cookieSecure,
		SessionCookieName:   "__Host-pcp_session",
		SessionTTLSeconds:   86400 * 7,
		CorsAllowedOrigins:  []string{"http://localhost:3000", "http://localhost:80"},
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
