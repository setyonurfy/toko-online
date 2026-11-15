package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"toko-online/internal/domain"
)

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
		wantAllow  string
	}{
		{"GET sukses", http.MethodGet, http.StatusOK, `{"status":"ok"}`, ""},
		{"POST ditolak", http.MethodPost, http.StatusMethodNotAllowed, "method tidak diizinkan", http.MethodGet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()

			healthHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, harus memuat %q", rec.Body.String(), tt.wantBody)
			}
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Errorf("header Allow = %q, ingin %q", got, tt.wantAllow)
			}
		})
	}
}

type fakeParser struct {
	id  int64
	err error
}

func (f fakeParser) Parse(token string) (int64, error) { return f.id, f.err }

func TestRouter_MeButuhToken(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Nama: "Siti", Email: "siti@mail.com"}}
	router := NewRouter(NewAuthHandler(svc), fakeParser{id: 7}, NewRateLimiter(6000, 1000))

	t.Run("tanpa token ditolak", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, ingin 401", rec.Code)
		}
	})

	t.Run("dengan token diterima", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer token-apa-saja")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, ingin 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Siti") {
			t.Errorf("body = %s, harus memuat nama Siti", rec.Body.String())
		}
	})
}

func TestRouter_LoginSampaiKeHandler(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Email: "siti@mail.com"}}
	router := NewRouter(NewAuthHandler(svc), fakeParser{}, NewRateLimiter(6000, 1000))

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"email":"siti@mail.com","password":"rahasia123"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"token":"token-palsu"`) {
		t.Errorf("body = %q, harus memuat token (handler login harus benar-benar jalan)", rec.Body.String())
	}
}
