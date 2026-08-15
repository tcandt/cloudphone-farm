package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Config struct {
	NodeID                        string
	AppEnv                        string
	Port                          string
	PostgresURL                   string
	RedisURL                      string
	CoturnSharedSecret            string
	SessionCookieSecure           bool
	SessionCookieName             string
	SessionTTLSeconds             int
	DeviceOnlineThresholdSeconds  int
	DeviceOfflineThresholdSeconds int
	CorsAllowedOrigins            []string
}

func LoadConfig() *Config {
	nodeID := getEnvOrFile("NODE_ID", "")
	if nodeID == "" {
		nodeID = fmt.Sprintf("node_%s", uuid.New().String()[:8])
	}

	appEnv := getEnvOrFile("APP_ENV", "development")
	port := getEnvOrFile("PORT", "8080")
	postgresURL := getEnvOrFile("POSTGRES_URL", "")
	if postgresURL == "" {
		postgresURL = getEnvOrFile("DATABASE_URL", "")
	}
	if postgresURL == "" {
		postgresURL = "postgres://pcp_user:pcp_secure_password_2026@localhost:5432/phone_control_platform?sslmode=disable"
	}
	redisURL := getEnvOrFile("REDIS_URL", "redis://localhost:6379/0")
	coturnSecret := getEnvOrFile("COTURN_SHARED_SECRET", "pcp_coturn_secret_key")
	cookieSecureStr := getEnvOrFile("SESSION_COOKIE_SECURE", "false")
	cookieSecure, _ := strconv.ParseBool(cookieSecureStr)

	corsEnv := getEnvOrFile("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:80")
	rawOrigins := strings.Split(corsEnv, ",")
	var corsOrigins []string
	for _, o := range rawOrigins {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			corsOrigins = append(corsOrigins, trimmed)
		}
	}

	onlineSec, _ := strconv.Atoi(getEnvOrFile("DEVICE_ONLINE_THRESHOLD_SECONDS", "30"))
	if onlineSec <= 0 {
		onlineSec = 30
	}

	offlineSec, _ := strconv.Atoi(getEnvOrFile("DEVICE_OFFLINE_THRESHOLD_SECONDS", "90"))
	if offlineSec <= 0 {
		offlineSec = 90
	}

	return &Config{
		NodeID:                        nodeID,
		AppEnv:                        appEnv,
		Port:                          port,
		PostgresURL:                   postgresURL,
		RedisURL:                      redisURL,
		CoturnSharedSecret:            coturnSecret,
		SessionCookieSecure:           cookieSecure,
		SessionCookieName:             "__Host-pcp_session",
		SessionTTLSeconds:             86400 * 7,
		DeviceOnlineThresholdSeconds:  onlineSec,
		DeviceOfflineThresholdSeconds: offlineSec,
		CorsAllowedOrigins:            corsOrigins,
	}
}

func ValidateProductionConfig(cfg *Config) error {
	if os.Getenv("E2E_AUTH_BYPASS") == "true" && strings.ToLower(cfg.AppEnv) != "test" {
		return errors.New("security boot failed: E2E_AUTH_BYPASS=true is strictly forbidden outside of APP_ENV=test")
	}

	if strings.ToLower(cfg.AppEnv) != "production" {
		return nil // Non-production environments allow default dev settings
	}

	if cfg.CoturnSharedSecret == "" || cfg.CoturnSharedSecret == "pcp_coturn_secret_key" {
		return errors.New("production boot failed: COTURN_SHARED_SECRET is missing or using insecure default key")
	}

	if !cfg.SessionCookieSecure {
		return errors.New("production boot failed: SESSION_COOKIE_SECURE must be true in production mode")
	}

	if strings.Contains(cfg.PostgresURL, "sslmode=disable") {
		return errors.New("production boot failed: PostgreSQL sslmode=disable is forbidden in production mode")
	}

	for _, origin := range cfg.CorsAllowedOrigins {
		if origin == "*" || strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			return fmt.Errorf("production boot failed: CORS_ALLOWED_ORIGINS contains insecure wildcard/localhost entry: %s", origin)
		}
	}

	return nil
}

func getEnvOrFile(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	if file := os.Getenv(key + "_FILE"); file != "" {
		if bytes, err := os.ReadFile(file); err == nil && len(bytes) > 0 {
			return strings.TrimSpace(string(bytes))
		}
	}
	return fallback
}
