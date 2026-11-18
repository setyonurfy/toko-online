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

// Cancel melengkapi tiruan fakeOrders (didefinisikan di order_handler_test.go)
// agar memenuhi interface OrderService.
func (f *fakeOrders) Cancel(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotOrderID = orderID
	return f.order, f.err
}

func TestOrderHandler_Cancel(t *testing.T) {
	dibatalkan := &domain.Order{ID: 5, UserID: 7, Status: domain.OrderCancelled, Total: 1000}

	tests := []struct {
		name          string
		userID        int64
		path          string
		svc           *fakeOrders
		wantStatus    int
		wantBody      string
		wantOrderID   int64
		wantDipanggil bool
	}{
		{"sukses", 7, "/orders/5/cancel", &fakeOrders{order: dibatalkan}, http.StatusOK, `"status":"cancelled"`, 5, true},
		{"tanpa identitas", 0, "/orders/5/cancel", &fakeOrders{}, http.StatusUnauthorized, "tidak terautentikasi", 0, false},
		{"tidak ada atau bukan milik", 7, "/orders/5/cancel", &fakeOrders{err: domain.ErrOrderNotFound}, http.StatusNotFound, "pesanan tidak ditemukan", 5, true},
		{"tidak dapat dibatalkan", 7, "/orders/5/cancel", &fakeOrders{err: domain.ErrOrderNotCancellable}, http.StatusConflict, "pesanan tidak dapat dibatalkan", 5, true},
		{"error tak terduga", 7, "/orders/5/cancel", &fakeOrders{err: errors.New("db mati")}, http.StatusInternalServerError, "terjadi kesalahan pada server", 5, true},

		{"id bukan angka", 7, "/orders/abc/cancel", &fakeOrders{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id nol", 7, "/orders/0/cancel", &fakeOrders{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id berawalan plus", 7, "/orders/+5/cancel", &fakeOrders{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id berawalan nol", 7, "/orders/05/cancel", &fakeOrders{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"id hilang", 7, "/orders/cancel", &fakeOrders{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"path berlebih", 7, "/orders/5/x/cancel", &fakeOrders{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
		{"tanpa akhiran cancel", 7, "/orders/5", &fakeOrders{}, http.StatusNotFound, "pesanan tidak ditemukan", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewOrderHandler(tt.svc)

			rec := kirimSebagai(h.Cancel, tt.userID, http.MethodPost, tt.path, "")
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
