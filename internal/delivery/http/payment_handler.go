package handler

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"

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
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	orderID, ok := idFromPathDenganAkhiran(r.URL.Path, "/orders/", "/pay")
	if !ok {
		writeError(w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}

	payment, err := h.payments.Pay(r.Context(), userID, orderID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payment)
}

func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, batasBodyWebhook)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var terlaluBesar *http.MaxBytesError
		if errors.As(err, &terlaluBesar) {
			writeError(w, http.StatusRequestEntityTooLarge, "body terlalu besar")
			return
		}
		writeError(w, http.StatusBadRequest, "body tidak dapat dibaca")
		return
	}

	timestamp := r.Header.Get(headerWebhookTimestamp)
	signature := r.Header.Get(headerWebhookSignature)

	outcome, err := h.payments.HandleWebhook(r.Context(), body, timestamp, signature)
	if err != nil {
		log.Printf("webhook ditolak dari %s: %v", clientIP(r), err)
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, webhookResponse{Hasil: string(outcome)})
}
