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
