package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"toko-online/internal/adapter/delivery/http/httpx"
	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

type AuthService interface {
	Register(ctx context.Context, nama, email, password string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.LoginResult, error)
	Profile(ctx context.Context, id int64) (*domain.User, error)
}

type LoginLimiter interface {
	Allow(key string) (bool, time.Duration)
}

type AuthHandler struct {
	auth         AuthService
	loginLimiter LoginLimiter
}

func NewAuthHandler(auth AuthService, loginLimiter LoginLimiter) *AuthHandler {
	return &AuthHandler{auth: auth, loginLimiter: loginLimiter}
}

func loginKey(ip, email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return ip + "|" + hex.EncodeToString(sum[:])
}

type registerRequest struct {
	Nama     string `json:"nama"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	var req registerRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	user, err := h.auth.Register(r.Context(), req.Nama, req.Email, req.Password)
	if err != nil {
		httpx.WriteUsecaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}
	var req loginRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	ip := httpx.ClientIP(r)
	diizinkan, tunggu := h.loginLimiter.Allow(loginKey(ip, req.Email))
	if !diizinkan {
		slog.Warn("rate limit menolak percobaan login", "ip", ip)
		httpx.WriteRateLimited(w, tunggu, "terlalu banyak percobaan, coba lagi nanti")
		return
	}

	result, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteUsecaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	user, err := h.auth.Profile(r.Context(), id)
	if err != nil {
		httpx.WriteUsecaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}
