package payment

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"toko-online/internal/adapter/delivery/http/httpx"
	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

const (
	headerWebhookTimestamp = "X-Webhook-Timestamp"
	headerWebhookSignature = "X-Webhook-Signature"
	batasBodyWebhook       = 1 << 14
)

type PaymentService interface {
	Pay(ctx context.Context, userID, orderID int64) (*domain.Payment, error)
	HandleWebhook(ctx context.Context, body []byte, timestamp, signature string) (domain.PaymentOutcome, error)
}

type PaymentHandler struct {
	payments PaymentService
}

func NewPaymentHandler(payments PaymentService) *PaymentHandler {
	return &PaymentHandler{payments: payments}
}

type webhookResponse struct {
	Hasil string `json:"hasil"`
}

func (h *PaymentHandler) Pay(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	orderID, ok := httpx.IDFromPathDenganAkhiran(r.URL.Path, "/orders/", "/pay")
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}

	payment, err := h.payments.Pay(r.Context(), userID, orderID)
	if err != nil {
		httpx.WriteUsecaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, payment)
}

func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, batasBodyWebhook)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var terlaluBesar *http.MaxBytesError
		if errors.As(err, &terlaluBesar) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "body terlalu besar")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "body tidak dapat dibaca")
		return
	}

	timestamp := r.Header.Get(headerWebhookTimestamp)
	signature := r.Header.Get(headerWebhookSignature)

	outcome, err := h.payments.HandleWebhook(r.Context(), body, timestamp, signature)
	if err != nil {
		slog.Warn("webhook ditolak", "ip", httpx.ClientIP(r), "error", err)
		httpx.WriteUsecaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, webhookResponse{Hasil: string(outcome)})
}
