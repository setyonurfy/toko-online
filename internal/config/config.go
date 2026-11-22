package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	JWTTTL         time.Duration
	WebhookSecret  string
	PaymentTTL     time.Duration
	ExpiryInterval time.Duration
	ExpiryBatch    int
	LogLevel       string
	LogFormat      string
}

func Load() (Config, error) {
	ttl, err := getDuration("ORDER_PAYMENT_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	interval, err := getDuration("ORDER_EXPIRY_INTERVAL", time.Minute)
	if err != nil {
		return Config{}, err
	}
	batch, err := getPositiveInt("ORDER_EXPIRY_BATCH", 100)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://localhost:5432/toko_online?sslmode=disable"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTTTL:         24 * time.Hour,
		WebhookSecret:  os.Getenv("PAYMENT_WEBHOOK_SECRET"),
		PaymentTTL:     ttl,
		ExpiryInterval: interval,
		ExpiryBatch:    batch,
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		LogFormat:      getEnv("LOG_FORMAT", "text"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s tidak valid: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s harus lebih dari nol", key)
	}
	return d, nil
}

func getPositiveInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s tidak valid: %w", key, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s harus lebih dari nol", key)
	}
	return n, nil
}
