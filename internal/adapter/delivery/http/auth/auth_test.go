package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

type fakeAuth struct {
	user *domain.User
	err  error
}

func (f *fakeAuth) Register(ctx context.Context, nama, email, password string) (*domain.User, error) {
	return f.user, f.err
}
func (f *fakeAuth) Login(ctx context.Context, email, password string) (*domain.LoginResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.LoginResult{User: f.user, Token: "token-palsu"}, nil
}
func (f *fakeAuth) Profile(ctx context.Context, id int64) (*domain.User, error) {
	return f.user, f.err
}

type fakeLimiter struct {
	diizinkan bool
	tunggu    time.Duration
	keys      []string
}

func (f *fakeLimiter) Allow(key string) (bool, time.Duration) {
	f.keys = append(f.keys, key)
	return f.diizinkan, f.tunggu
}

func limiterBebas() *fakeLimiter { return &fakeLimiter{diizinkan: true} }

func TestRegisterHandler(t *testing.T) {
	const bodyValid = `{"nama":"jhondoe", "email":"jhondoe@mail.com","password":"pass12345"}`
	userOK := &domain.User{ID: 1, Nama: "jhondoe", Email: "jhondoe@mail.com", PasswordHash: "hash"}

	tests := []struct {
		alert      string
		method     string
		body       string
		svc        *fakeAuth
		wantStatus int
	}{
		{"sukses", http.MethodPost, bodyValid, &fakeAuth{user: userOK}, http.StatusCreated},
		{"email sudah terdaftar", http.MethodPost, bodyValid, &fakeAuth{err: domain.ErrEmailAlreadyUsed}, http.StatusConflict},
		{"input tidak valid", http.MethodPost, bodyValid, &fakeAuth{err: domain.ErrInvalidInput}, http.StatusBadRequest},
		{"json rusak", http.MethodPost, `{bukan json`, &fakeAuth{}, http.StatusBadRequest},
		{"method salah", http.MethodGet, "", &fakeAuth{}, http.StatusMethodNotAllowed},
		{"error tak terduga", http.MethodPost, bodyValid, &fakeAuth{err: errors.New("db mati")}, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.alert, func(t *testing.T) {
			h := NewAuthHandler(tt.svc, limiterBebas())
			req := httptest.NewRequest(tt.method, "/auth/register", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusCreated && strings.Contains(rec.Body.String(), "hash") {
				t.Error("response tidak boleh memuat password hash")
			}
		})
	}
}

func TestLoginHandler(t *testing.T) {
	const bodyValid = `{"email":"budi@mail.com","password":"rahasia123"}`
	userOK := &domain.User{ID: 1, Nama: "Budi", Email: "budi@mail.com", PasswordHash: "hash-rahasia"}

	tests := []struct {
		alert      string
		method     string
		body       string
		svc        *fakeAuth
		wantStatus int
		wantBody   string // potongan teks yang HARUS ada di respons
		notBody    string // potongan teks yang TIDAK BOLEH ada di respons
	}{
		{
			alert: "sukses", method: http.MethodPost, body: bodyValid,
			svc:        &fakeAuth{user: userOK},
			wantStatus: http.StatusOK,
			wantBody:   `"token":"token-palsu"`,
			notBody:    "hash",
		},
		{
			alert: "password salah", method: http.MethodPost, body: bodyValid,
			svc:        &fakeAuth{err: domain.ErrInvalidCredentials},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "email atau password salah",
		},
		{
			alert: "json rusak", method: http.MethodPost, body: `{bukan json`,
			svc:        &fakeAuth{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "body JSON tidak valid",
		},
		{
			alert: "field tidak dikenal", method: http.MethodPost,
			body:       `{"email":"budi@mail.com","pasword":"salah-ketik"}`,
			svc:        &fakeAuth{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "body JSON tidak valid",
		},
		{
			alert: "method salah", method: http.MethodGet, body: "",
			svc:        &fakeAuth{},
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "method tidak diizinkan",
		},
		{
			alert: "error tak terduga", method: http.MethodPost, body: bodyValid,
			svc:        &fakeAuth{err: errors.New("db mati")},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "terjadi kesalahan pada server",
			notBody:    "db mati",
		},
	}

	for _, tt := range tests {
		t.Run(tt.alert, func(t *testing.T) {
			h := NewAuthHandler(tt.svc, limiterBebas())
			req := httptest.NewRequest(tt.method, "/auth/login", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.Login(rec, req)

			body := rec.Body.String()

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d (body: %s)", rec.Code, tt.wantStatus, body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, ingin application/json", ct)
			}
			if !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %s, harus memuat %q", body, tt.wantBody)
			}
			if tt.notBody != "" && strings.Contains(body, tt.notBody) {
				t.Errorf("body = %s, tidak boleh memuat %q", body, tt.notBody)
			}
			if strings.Contains(strings.TrimSpace(rec.Body.String()), "\n") {
				t.Errorf("respons ditulis lebih dari sekali: %q", rec.Body.String())
			}
		})
	}
}
func TestRegisterHandler_BodyTerlaluBesar(t *testing.T) {
	body := `{"nama":"` + strings.Repeat("a", 20000) + `"}`
	h := NewAuthHandler(&fakeAuth{}, limiterBebas())
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, ingin 413", rec.Code)
	}
}

func TestMeHandler_MethodSalah(t *testing.T) {
	h := NewAuthHandler(&fakeAuth{}, limiterBebas())
	req := httptest.NewRequest(http.MethodPost, "/me", nil)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, ingin %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if !strings.Contains(rec.Body.String(), "method tidak diizinkan") {
		t.Errorf("body = %s, ingin memuat %q", rec.Body.String(), "method tidak diizinkan")
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Errorf("Allow = %q, ingin %q", got, http.MethodGet)
	}
}

func TestLoginHandler_BatasPercobaanDitolak(t *testing.T) {
	limiter := &fakeLimiter{tunggu: 90 * time.Second}
	h := NewAuthHandler(&fakeAuth{user: &domain.User{ID: 1}}, limiter)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"budi@mail.com","password":"rahasia123"}`))
	req.RemoteAddr = "9.9.9.9:1234"
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, ingin 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "90" {
		t.Errorf("Retry-After = %q, ingin 90", got)
	}
	if !strings.Contains(rec.Body.String(), "terlalu banyak percobaan") {
		t.Errorf("body = %s, ingin memuat pesan batas percobaan", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "token-palsu") {
		t.Error("login tidak boleh diproses ketika batas terlampaui")
	}
	if len(limiter.keys) != 1 {
		t.Fatalf("limiter dipanggil %d kali, ingin 1", len(limiter.keys))
	}
	if !strings.HasPrefix(limiter.keys[0], "9.9.9.9|") {
		t.Errorf("kunci = %q, ingin diawali IP klien", limiter.keys[0])
	}
	if strings.Contains(limiter.keys[0], "budi") {
		t.Errorf("kunci = %q, tidak boleh memuat email apa adanya", limiter.keys[0])
	}
}

func TestLoginHandler_KunciPerIPDanEmail(t *testing.T) {
	h := NewAuthHandler(&fakeAuth{user: &domain.User{ID: 1}}, middleware.NewRateLimiter(1, 1))

	kirim := func(remoteAddr, email string) int {
		body := `{"email":"` + email + `","password":"rahasia123"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		h.Login(rec, req)
		return rec.Code
	}

	if got := kirim("1.1.1.1:1000", "Budi@Mail.com"); got != http.StatusOK {
		t.Fatalf("percobaan pertama: status = %d, ingin 200", got)
	}
	if got := kirim("1.1.1.1:2000", " budi@mail.com "); got != http.StatusTooManyRequests {
		t.Errorf("email sama beda huruf dan spasi, IP sama: status = %d, ingin 429", got)
	}
	if got := kirim("1.1.1.1:3000", "siti@mail.com"); got != http.StatusOK {
		t.Errorf("email lain, IP sama: status = %d, ingin 200", got)
	}
	if got := kirim("2.2.2.2:1000", "budi@mail.com"); got != http.StatusOK {
		t.Errorf("email sama, IP lain: status = %d, ingin 200", got)
	}
}
