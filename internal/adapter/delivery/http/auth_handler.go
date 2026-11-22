package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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

	result, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method tdak diizinkan")
		return
	}

	id, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	user, err := h.auth.Profile(r.Context(), id)
	if err != nil {
		writeUsecaseError(w, err)
		return
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
	case errors.Is(err, domain.ErrUserNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrProductNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrProductInUse):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrOrderNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInsufficientStock):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrOrderNotCancellable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrOrderNotPayable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrPaymentNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrAmountMismatch):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domain.ErrInvalidSignature), errors.Is(err, domain.ErrStaleWebhook):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrMalformedWebhook):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("error internal", "error", err)
		writeError(w, http.StatusInternalServerError, "terjadi kesalahan pada server")
	}
}
