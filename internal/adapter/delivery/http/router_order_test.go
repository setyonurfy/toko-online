package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"toko-online/internal/domain"
)

func TestRouter_BatalkanPesanan(t *testing.T) {
	pembeli := &domain.User{ID: 7, Role: domain.RoleCustomer}
	dibatalkan := &domain.Order{ID: 1, UserID: 7, Status: domain.OrderCancelled, Total: 50000}

	tests := []struct {
		name          string
		method        string
		path          string
		token         bool
		wantStatus    int
		wantAllow     string
		wantDipanggil bool
	}{
		{"batal tanpa token", http.MethodPost, "/orders/1/cancel", false, http.StatusUnauthorized, "", false},
		{"batal dengan token", http.MethodPost, "/orders/1/cancel", true, http.StatusOK, "", true},

		{"GET ke cancel tidak didukung", http.MethodGet, "/orders/1/cancel", true, http.StatusMethodNotAllowed, "POST", false},
		{"PUT ke cancel tidak didukung", http.MethodPut, "/orders/1/cancel", true, http.StatusMethodNotAllowed, "POST", false},
		{"DELETE ke cancel tidak didukung", http.MethodDelete, "/orders/1/cancel", true, http.StatusMethodNotAllowed, "POST", false},

		{"detail tetap bisa dibaca", http.MethodGet, "/orders/1", true, http.StatusOK, "", true},
		{"POST ke detail tidak membatalkan apa pun", http.MethodPost, "/orders/1", true, http.StatusMethodNotAllowed, "GET", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeOrders{order: dibatalkan}
			router := newTestRouterLengkap(&fakeAuth{user: pembeli}, &fakeProducts{}, svc, fakeParser{id: 7})

			req := httptest.NewRequest(tt.method, tt.path, nil)
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
				t.Errorf("usecase pesanan dipanggil = %v, ingin %v", svc.dipanggil, tt.wantDipanggil)
			}
			if svc.dipanggil && svc.gotUserID != 7 {
				t.Errorf("userID yang sampai ke usecase = %d, ingin 7 (dari token)", svc.gotUserID)
			}
		})
	}
}
