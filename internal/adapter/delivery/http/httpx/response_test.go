package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteRateLimited(t *testing.T) {
	tests := []struct {
		name   string
		tunggu time.Duration
		want   string
	}{
		{"tepat satu detik", time.Second, "1"},
		{"dibulatkan ke atas", 1500 * time.Millisecond, "2"},
		{"sembilan puluh detik", 90 * time.Second, "90"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			WriteRateLimited(rec, tt.tunggu, "terlalu banyak")

			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, ingin 429", rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != tt.want {
				t.Errorf("Retry-After = %q, ingin %q", got, tt.want)
			}
			if !strings.Contains(rec.Body.String(), "terlalu banyak") {
				t.Errorf("body = %s, ingin memuat pesan", rec.Body.String())
			}
		})
	}
}
