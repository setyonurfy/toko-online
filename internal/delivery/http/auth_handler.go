package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"toko-online/internal/domain"
)

type AuthService interface {
	Register(ctx context.Context, nama, email, password string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, error)
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
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}

	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.auth.Register(r.Context(), req.Nama, req.Email, req.Password)
	if err != nil {
		writeUsecaseError(w, err)
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeUsecaseError(w, err)
	}
	writeJSON(w, http.StatusOK, user)
}

func writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyUsed):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		log.Printf("error internal: %v", err)
		writeError(w, http.StatusInternalServerError, "kesalahan pada server")

	}
}
