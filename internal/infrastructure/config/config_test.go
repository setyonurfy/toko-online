package config

import (
	"testing"
	"time"
)

func aturEnv(t *testing.T, ttl, interval, batch string) {
	t.Helper()
	t.Setenv("ORDER_PAYMENT_TTL", ttl)
	t.Setenv("ORDER_EXPIRY_INTERVAL", interval)
	t.Setenv("ORDER_EXPIRY_BATCH", batch)
}

func TestLoad_NilaiBawawanKedaluwarsa(t *testing.T) {
	aturEnv(t, "", "", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load gagal: %v", err)
	}
	if cfg.PaymentTTL != 24*time.Hour {
		t.Errorf("PaymentTTL = %v, ingin 24h", cfg.PaymentTTL)
	}
	if cfg.ExpiryInterval != time.Minute {
		t.Errorf("ExpiryInterval = %v, ingin 1m", cfg.ExpiryInterval)
	}
	if cfg.ExpiryBatch != 100 {
		t.Errorf("ExpiryBatch = %d, ingin 100", cfg.ExpiryBatch)
	}
}

func TestLoad_NilaiDariEnvironment(t *testing.T) {
	aturEnv(t, "2h", "30s", "25")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load gagal: %v", err)
	}
	if cfg.PaymentTTL != 2*time.Hour {
		t.Errorf("PaymentTTL = %v, ingin 2h", cfg.PaymentTTL)
	}
	if cfg.ExpiryInterval != 30*time.Second {
		t.Errorf("ExpiryInterval = %v, ingin 30s", cfg.ExpiryInterval)
	}
	if cfg.ExpiryBatch != 25 {
		t.Errorf("ExpiryBatch = %d, ingin 25", cfg.ExpiryBatch)
	}
}

func TestLoad_NilaiTidakValid(t *testing.T) {
	tests := []struct {
		name                 string
		ttl, interval, batch string
	}{
		{"ttl bukan durasi", "abc", "", ""},
		{"ttl nol", "0s", "", ""},
		{"ttl negatif", "-1h", "", ""},
		{"interval bukan durasi", "", "abc", ""},
		{"interval nol", "", "0s", ""},
		{"batch bukan angka", "", "", "abc"},
		{"batch nol", "", "", "0"},
		{"batch negatif", "", "", "-5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aturEnv(t, tt.ttl, tt.interval, tt.batch)

			if _, err := Load(); err == nil {
				t.Error("Load harus gagal untuk nilai tidak valid")
			}
		})
	}
}
func TestLoad_LogBawaan(t *testing.T) {
	aturEnv(t, "", "", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load gagal: %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, ingin info", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, ingin text", cfg.LogFormat)
	}
}

func TestLoad_LogDariEnvironment(t *testing.T) {
	aturEnv(t, "", "", "")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load gagal: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, ingin debug", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, ingin json", cfg.LogFormat)
	}
}
func TestLoad_ShutdownBawaan(t *testing.T) {
	aturEnv(t, "", "", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load gagal: %v", err)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, ingin 15s", cfg.ShutdownTimeout)
	}
}

func TestLoad_ShutdownDariEnvironment(t *testing.T) {
	aturEnv(t, "", "", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load gagal: %v", err)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, ingin 30s", cfg.ShutdownTimeout)
	}
}

func TestLoad_ShutdownTidakValid(t *testing.T) {
	for _, nilai := range []string{"abc", "0s", "-5s"} {
		t.Run(nilai, func(t *testing.T) {
			aturEnv(t, "", "", "")
			t.Setenv("SHUTDOWN_TIMEOUT", nilai)

			if _, err := Load(); err == nil {
				t.Errorf("Load harus gagal untuk SHUTDOWN_TIMEOUT=%q", nilai)
			}
		})
	}
}
