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
	ServerPort string

	AdminUser         string
	AdminPasswordHash string
	AdminSessionTTL   time.Duration

	DB DBConfig

	Redis RedisConfig

	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	DevMode bool
	DevIPs  []string
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Addr      string
	Username  string
	Password  string
	DB        int
	KeyPrefix string
}

func Load(envPath string) (*Config, error) {
	_ = godotenv.Load(envPath)

	dbPort, err := envInt("DB_PORT", 5432)
	if err != nil {
		return nil, err
	}
	redisDB, err := envInt("REDIS_DB", 0)
	if err != nil {
		return nil, err
	}
	accessTTL, err := envDuration("ACCESS_TOKEN_TTL", 24*time.Hour)
	if err != nil {
		return nil, err
	}
	refreshTTL, err := envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	adminSessionTTL, err := envDuration("ADMIN_SESSION_TTL", 12*time.Hour)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ServerPort: envString("SERVER_PORT", "8080"),
		AdminUser: envString("ADMIN_USER", "admin"),

		AdminPasswordHash: envString("ADMIN_PASSWORD_HASH", ""),
		AdminSessionTTL:   adminSessionTTL,

		DB: DBConfig{
			Host:     envString("DB_HOST", "127.0.0.1"),
			Port:     dbPort,
			User:     envString("DB_USER", "postgres"),
			Password: envString("DB_PASSWORD", ""),
			Name:     envString("DB_NAME", "simple_auth"),
			SSLMode:  envString("DB_SSL_MODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:      envString("REDIS_ADDR", "127.0.0.1:6379"),
			Username:  envString("REDIS_USERNAME", ""),
			Password:  envString("REDIS_PASSWORD", ""),
			DB:        redisDB,
			KeyPrefix: envString("REDIS_KEY_PREFIX", "simple-auth:"),
		},
		JWTSecret:  envString("JWT_SECRET", ""),
		AccessTTL:  accessTTL,
		RefreshTTL: refreshTTL,
		DevMode:    envBoolAny([]string{"dev_model", "DEV_MODEL"}, false),
		DevIPs:     splitCSV(envString("DEV_IP", "127.0.0.1,::1")),
	}

	if cfg.AdminPasswordHash == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD_HASH is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

func envString(key string, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return strings.TrimSpace(value)
}

func envInt(key string, fallback int) (int, error) {
	raw := envString(key, "")
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be integer: %w", key, err)
	}
	return value, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := envString(key, "")
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be duration, example 24h: %w", key, err)
	}
	return value, nil
}

func envBoolAny(keys []string, fallback bool) bool {
	for _, key := range keys {
		raw, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		raw = strings.TrimSpace(strings.ToLower(raw))
		return raw == "true" || raw == "1" || raw == "yes" || raw == "on"
	}
	return fallback
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}
