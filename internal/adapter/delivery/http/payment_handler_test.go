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

func newTestRouterBayar(svc *fakeAuth, bayar PaymentService, parser TokenParser) http.Handler {
	return NewRouter(Dependencies{
		Auth:        NewAuthHandler(svc),
		Products:    NewProductHandler(&fakeProducts{}),
		Orders:      NewOrderHandler(&fakeOrders{}),
		Payments:    NewPaymentHandler(bayar),
		Tokens:      parser,
		Users:       svc,
		AuthLimiter: NewRateLimiter(6000, 1000),
	})
}

func TestPaymentHandler_Pay(t *testing.T) {
	tagihan := &domain.Payment{ID: 1, OrderID: 5, Reference: "SIM-1", Amount: 100000, Status: domain.PaymentPending}

	tests := []struct {
		name          string
		userID        int64
		path          string
		svc           *fakePayments
		wantStatus    int
		wantBody      string
		wantOrderID   int64
		wantDipanggil bool
	}{
		{"sukses", 7, "/orders/5/pay", &fakePayments{payment: tagihan}, http.StatusOK, `"reference":"SIM-1"`, 5, true},
		{"tanpa identitas", 0, "/orders/5/pay", &fakePayments{}, http.StatusUnauthorized, "tidak terautentikasi", 0, false},
		{"pesanan tidak ada atau bukan milik", 7, "/orders/5/pay", &fakePayments{err: domain.ErrOrderNotFound}, http.StatusNotFound, "pesanan tidak ditemukan", 5, true},
		{"pesanan tidak dapat dibayar", 7, "/orders/5/pay", &fakePayments{err: domain.ErrOrderNotPayable}, http.StatusConflict, "pesanan tidak dapat dibayar", 5, true},
		{"error tak terduga", 7, "/orders/5/pay", &fakePayments{err: errors.New("db mati")}, http.StatusInternalServerError, "terjadi kesalahan pada server", 5, true},

		{"id bukan angka", 7, "/orders/abc/pay", &fakePayments{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id nol", 7, "/orders/0/pay", &fakePayments{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id berawalan plus", 7, "/orders/+5/pay", &fakePayments{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id berawalan nol", 7, "/orders/05/pay", &fakePayments{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id hilang", 7, "/orders/pay", &fakePayments{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"path berlebih", 7, "/orders/5/x/pay", &fakePayments{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"tanpa akhiran pay", 7, "/orders/5", &fakePayments{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPaymentHandler(tt.svc)

			rec := kirimSebagai(h.Pay, tt.userID, http.MethodPost, tt.path, "")
			body := rec.Body.String()

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d (body: %s)", rec.Code, tt.wantStatus, body)
			}
			if !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %s, harus memuat %q", body, tt.wantBody)
			}
			if strings.Contains(body, "db mati") {
				t.Errorf("detail error internal bocor ke klien: %s", body)
			}
			if strings.Contains(strings.TrimSpace(body), "\n") {
				t.Errorf("respons ditulis lebih dari sekali: %q", body)
			}
			if tt.svc.dipanggil != tt.wantDipanggil {
				t.Errorf("usecase dipanggil = %v, ingin %v", tt.svc.dipanggil, tt.wantDipanggil)
			}
			if tt.svc.gotOrderID != tt.wantOrderID {
				t.Errorf("ID pesanan yang diteruskan = %d, ingin %d", tt.svc.gotOrderID, tt.wantOrderID)
			}
			if tt.svc.dipanggil && tt.svc.gotUserID != tt.userID {
				t.Errorf("userID yang diteruskan = %d, ingin %d (dari token)", tt.svc.gotUserID, tt.userID)
			}
		})
	}
}

func TestPaymentHandler_Webhook_Hasil(t *testing.T) {
	semua := []domain.PaymentOutcome{
		domain.OutcomePaid,
		domain.OutcomeAlreadyPaid,
		domain.OutcomeRefundNeeded,
		domain.OutcomeIgnored,
	}

	for _, hasil := range semua {
		t.Run(string(hasil), func(t *testing.T) {
			svc := &fakePayments{outcome: hasil}

			rec := kirimWebhook(NewPaymentHandler(svc).Webhook, `{}`, "123", "abc")

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, ingin 200 (body: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"hasil":"`+string(hasil)+`"`) {
				t.Errorf("body = %s, harus memuat hasil %q", rec.Body.String(), hasil)
			}
		})
	}
}

func TestPaymentHandler_Webhook_MeneruskanBodyMentah(t *testing.T) {
	const body = "{\"type\":\"payment.paid\",   \"reference\":\"SIM-1\",\n  \"amount\":100000}"
	svc := &fakePayments{outcome: domain.OutcomePaid}

	rec := kirimWebhook(NewPaymentHandler(svc).Webhook, body, "1700000000", "tanda-tangan-uji")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200", rec.Code)
	}
	if string(svc.gotBody) != body {
		t.Errorf("body yang diteruskan = %q, ingin persis %q", svc.gotBody, body)
	}
	if svc.gotTimestamp != "1700000000" || svc.gotSignature != "tanda-tangan-uji" {
		t.Errorf("header yang diteruskan = (%q, %q), ingin (1700000000, tanda-tangan-uji)", svc.gotTimestamp, svc.gotSignature)
	}
}

func TestPaymentHandler_Webhook_Penolakan(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"tanda tangan tidak valid", domain.ErrInvalidSignature, http.StatusUnauthorized, "tanda tangan webhook tidak valid"},
		{"webhook kedaluwarsa", domain.ErrStaleWebhook, http.StatusUnauthorized, "webhook kedaluwarsa"},
		{"isi tidak valid", domain.ErrMalformedWebhook, http.StatusBadRequest, "isi webhook tidak valid"},
		{"pembayaran tidak ditemukan", domain.ErrPaymentNotFound, http.StatusNotFound, "pembayaran tidak ditemukan"},
		{"jumlah tidak sesuai", domain.ErrAmountMismatch, http.StatusUnprocessableEntity, "jumlah pembayaran tidak sesuai"},
		{"error tak terduga", errors.New("db mati"), http.StatusInternalServerError, "terjadi kesalahan pada server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePayments{err: tt.err}

			rec := kirimWebhook(NewPaymentHandler(svc).Webhook, `{}`, "123", "abc")
			body := rec.Body.String()

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d (body: %s)", rec.Code, tt.wantStatus, body)
			}
			if !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %s, harus memuat %q", body, tt.wantBody)
			}
			if strings.Contains(body, "db mati") {
				t.Errorf("detail error internal bocor ke klien: %s", body)
			}
			if strings.Contains(strings.TrimSpace(body), "\n") {
				t.Errorf("respons ditulis lebih dari sekali: %q", body)
			}
		})
	}
}

func TestPaymentHandler_Webhook_BodyTerlaluBesar(t *testing.T) {
	svc := &fakePayments{}

	rec := kirimWebhook(NewPaymentHandler(svc).Webhook, strings.Repeat("a", 20000), "123", "abc")

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, ingin 413", rec.Code)
	}
	if svc.dipanggil {
		t.Error("usecase tidak boleh dipanggil untuk body yang terlalu besar")
	}
}

func TestRouter_Pembayaran(t *testing.T) {
	pembeli := &domain.User{ID: 7, Role: domain.RoleCustomer}
	tagihan := &domain.Payment{ID: 1, OrderID: 1, Reference: "SIM-1", Amount: 1000, Status: domain.PaymentPending}

	tests := []struct {
		name          string
		method        string
		path          string
		token         bool
		wantStatus    int
		wantAllow     string
		wantDipanggil bool
	}{
		{"bayar tanpa token", http.MethodPost, "/orders/1/pay", false, http.StatusUnauthorized, "", false},
		{"bayar dengan token", http.MethodPost, "/orders/1/pay", true, http.StatusOK, "", true},
		{"GET ke pay tidak didukung", http.MethodGet, "/orders/1/pay", true, http.StatusMethodNotAllowed, "POST", false},
		{"DELETE ke pay tidak didukung", http.MethodDelete, "/orders/1/pay", true, http.StatusMethodNotAllowed, "POST", false},

		{"webhook tidak membutuhkan token login", http.MethodPost, "/webhooks/payments", false, http.StatusOK, "", true},
		{"GET ke webhook tidak didukung", http.MethodGet, "/webhooks/payments", false, http.StatusMethodNotAllowed, "POST", false},
		{"PUT ke webhook tidak didukung", http.MethodPut, "/webhooks/payments", false, http.StatusMethodNotAllowed, "POST", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePayments{payment: tagihan, outcome: domain.OutcomePaid}
			router := newTestRouterBayar(&fakeAuth{user: pembeli}, svc, fakeParser{id: 7})

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			if tt.token {
				req.Header.Set("Authorization", "Bearer token-apa-saja")
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := rec.Result().Header.Get("Allow"); got != tt.wantAllow {
				t.Errorf("header Allow = %q, ingin %q", got, tt.wantAllow)
			}
			if svc.dipanggil != tt.wantDipanggil {
				t.Errorf("usecase pembayaran dipanggil = %v, ingin %v", svc.dipanggil, tt.wantDipanggil)
			}
			if svc.dipanggil && strings.HasSuffix(tt.path, "/pay") && svc.gotUserID != 7 {
				t.Errorf("userID yang sampai ke usecase = %d, ingin 7 (dari token)", svc.gotUserID)
			}
		})
	}
}
