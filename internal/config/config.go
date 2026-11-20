package config

import (
	"os"
	"time"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	JWTTTL        time.Duration
	WebhookSecret string
}

func Load() Config {
	return Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://localhost:5432/toko_online?sslmode=disable"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTTTL:        24 * time.Hour,
		WebhookSecret: os.Getenv("PAYMENT_WEBHOOK_SECRET"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
