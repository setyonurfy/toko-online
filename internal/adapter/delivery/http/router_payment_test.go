package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/adapter/delivery/http/payment"
	"toko-online/internal/domain"
)

func newTestRouterBayar(svc *fakeAuth, bayar payment.PaymentService, parser middleware.TokenParser) http.Handler {
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
