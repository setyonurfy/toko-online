package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"toko-online/internal/domain"
)

func WriteUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyUsed):
		WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		WriteError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrProductNotFound):
		WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrProductInUse):
		WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrOrderNotFound):
		WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInsufficientStock):
		WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrOrderNotCancellable):
		WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrOrderNotPayable):
		WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrPaymentNotFound):
		WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrAmountMismatch):
		WriteError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domain.ErrInvalidSignature), errors.Is(err, domain.ErrStaleWebhook):
		WriteError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrMalformedWebhook):
		WriteError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("error internal", "error", err)
		WriteError(w, http.StatusInternalServerError, "terjadi kesalahan pada server")
	}
}
