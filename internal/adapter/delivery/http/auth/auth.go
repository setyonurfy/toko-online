package auth

import (
	"context"
	"net/http"

	"toko-online/internal/adapter/delivery/http/httpx"
	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

type AuthService interface {
	Register(ctx context.Context, nama, email, password string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.LoginResult, error)
	Profile(ctx context.Context, id int64) (*domain.User, error)
}

type AuthHandler struct {
	auth AuthService
}

func NewAuthHandler(auth AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
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
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method tdak diizinkan")
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
