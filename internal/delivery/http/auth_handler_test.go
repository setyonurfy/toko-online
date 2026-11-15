package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
			h := NewAuthHandler(tt.svc)
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
			h := NewAuthHandler(tt.svc)
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
	h := NewAuthHandler(&fakeAuth{})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, ingin 413", rec.Code)
	}
}
