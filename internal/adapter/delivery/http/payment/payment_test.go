package payment

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"toko-online/internal/domain"
)

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
