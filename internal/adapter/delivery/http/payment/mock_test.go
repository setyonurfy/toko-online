package payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

type fakePayments struct {
	payment *domain.Payment
	outcome domain.PaymentOutcome
	err     error

	dipanggil    bool
	gotUserID    int64
	gotOrderID   int64
	gotBody      []byte
	gotTimestamp string
	gotSignature string
}

func (f *fakePayments) Pay(ctx context.Context, userID, orderID int64) (*domain.Payment, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotOrderID = orderID
	return f.payment, f.err
}

func (f *fakePayments) HandleWebhook(ctx context.Context, body []byte, timestamp, signature string) (domain.PaymentOutcome, error) {
	f.dipanggil = true
	f.gotBody = append([]byte(nil), body...)
	f.gotTimestamp = timestamp
	f.gotSignature = signature
	return f.outcome, f.err
}

func kirimWebhook(h http.HandlerFunc, body, timestamp, signature string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/payments", strings.NewReader(body))
	if timestamp != "" {
		req.Header.Set(headerWebhookTimestamp, timestamp)
	}
	if signature != "" {
		req.Header.Set(headerWebhookSignature, signature)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func kirimSebagai(h http.HandlerFunc, userID int64, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != 0 {
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}
