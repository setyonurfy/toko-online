package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"toko-online/internal/domain"
)

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
		wantAllow  string
	}{
		{"GET sukses", http.MethodGet, http.StatusOK, `{"status":"ok"}`, ""},
		{"POST ditolak", http.MethodPost, http.StatusMethodNotAllowed, "method tidak diizinkan", http.MethodGet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()

			healthHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, harus memuat %q", rec.Body.String(), tt.wantBody)
			}
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Errorf("header Allow = %q, ingin %q", got, tt.wantAllow)
			}
		})
	}
}
func newTestRouter(svc *fakeAuth, produk ProductService, parser TokenParser) http.Handler {
	return newTestRouterLengkap(svc, produk, &fakeOrders{}, parser)
}

func newTestRouterLengkap(svc *fakeAuth, produk ProductService, pesanan OrderService, parser TokenParser) http.Handler {
	return NewRouter(Dependencies{
		Auth:        NewAuthHandler(svc),
		Products:    NewProductHandler(produk),
		Orders:      NewOrderHandler(pesanan),
		Tokens:      parser,
		Users:       svc,
		AuthLimiter: NewRateLimiter(6000, 1000),
	})
}

type fakeParser struct {
	id  int64
	err error
}

func (f fakeParser) Parse(token string) (int64, error) { return f.id, f.err }

func TestRouter_MeButuhToken(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Nama: "Siti", Email: "siti@mail.com"}}
	router := newTestRouter(svc, &fakeProducts{}, fakeParser{id: 7})

	t.Run("tanpa token ditolak", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, ingin 401", rec.Code)
		}
	})

	t.Run("dengan token diterima", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer token-apa-saja")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, ingin 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Siti") {
			t.Errorf("body = %s, harus memuat nama Siti", rec.Body.String())
		}
	})
}

func TestRouter_LoginSampaiKeHandler(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Email: "siti@mail.com"}}
	router := newTestRouter(svc, &fakeProducts{}, fakeParser{id: 7})

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"email":"siti@mail.com","password":"rahasia123"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"token":"token-palsu"`) {
		t.Errorf("body = %q, harus memuat token (handler login harus benar-benar jalan)", rec.Body.String())
	}
}

func TestRouter_AksesProduk(t *testing.T) {
	admin := &domain.User{ID: 7, Role: domain.RoleAdmin}
	pembeli := &domain.User{ID: 7, Role: domain.RoleCustomer}
	kopi := &domain.Product{ID: 1, Nama: "Kopi", Harga: 55000, Stok: 10}
	const body = `{"nama":"Kopi","harga":55000,"stok":10}`

	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		token         bool
		user          *domain.User
		wantStatus    int
		wantDipanggil bool
	}{
		{"daftar produk terbuka", http.MethodGet, "/products", "", false, nil, http.StatusOK, true},
		{"detail produk terbuka", http.MethodGet, "/products/1", "", false, nil, http.StatusOK, true},

		{"tambah tanpa token", http.MethodPost, "/products", body, false, nil, http.StatusUnauthorized, false},
		{"tambah sebagai pembeli", http.MethodPost, "/products", body, true, pembeli, http.StatusForbidden, false},
		{"tambah sebagai admin", http.MethodPost, "/products", body, true, admin, http.StatusCreated, true},

		{"ubah tanpa token", http.MethodPut, "/products/1", body, false, nil, http.StatusUnauthorized, false},
		{"ubah sebagai pembeli", http.MethodPut, "/products/1", body, true, pembeli, http.StatusForbidden, false},
		{"ubah sebagai admin", http.MethodPut, "/products/1", body, true, admin, http.StatusOK, true},

		{"hapus tanpa token", http.MethodDelete, "/products/1", "", false, nil, http.StatusUnauthorized, false},
		{"hapus sebagai pembeli", http.MethodDelete, "/products/1", "", true, pembeli, http.StatusForbidden, false},
		{"hapus sebagai admin", http.MethodDelete, "/products/1", "", true, admin, http.StatusNoContent, true},

		{"PATCH tidak didukung", http.MethodPatch, "/products/1", body, true, admin, http.StatusMethodNotAllowed, false},
		{"POST ke detail tidak didukung", http.MethodPost, "/products/1", body, true, admin, http.StatusMethodNotAllowed, false},
		{"DELETE ke koleksi tidak didukung", http.MethodDelete, "/products", "", true, admin, http.StatusMethodNotAllowed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			produk := &fakeProducts{product: kopi}
			router := newTestRouter(&fakeAuth{user: tt.user}, produk, fakeParser{id: 7})

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.token {
				req.Header.Set("Authorization", "Bearer token-apa-saja")
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if produk.dipanggil != tt.wantDipanggil {
				t.Errorf("usecase produk dipanggil = %v, ingin %v", produk.dipanggil, tt.wantDipanggil)
			}
		})
	}
}

func TestRouter_AksesPesanan(t *testing.T) {
	pembeli := &domain.User{ID: 7, Role: domain.RoleCustomer}
	pesanan := &domain.Order{ID: 1, UserID: 7, Status: domain.OrderPending, Total: 50000}
	const body = `{"items":[{"product_id":1,"jumlah":1}]}`

	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		token         bool
		wantStatus    int
		wantDipanggil bool
	}{
		{"daftar tanpa token", http.MethodGet, "/orders", "", false, http.StatusUnauthorized, false},
		{"buat tanpa token", http.MethodPost, "/orders", body, false, http.StatusUnauthorized, false},
		{"detail tanpa token", http.MethodGet, "/orders/1", "", false, http.StatusUnauthorized, false},

		{"daftar dengan token", http.MethodGet, "/orders", "", true, http.StatusOK, true},
		{"buat dengan token", http.MethodPost, "/orders", body, true, http.StatusCreated, true},
		{"detail dengan token", http.MethodGet, "/orders/1", "", true, http.StatusOK, true},

		{"PUT ke koleksi tidak didukung", http.MethodPut, "/orders", body, true, http.StatusMethodNotAllowed, false},
		{"POST ke detail tidak didukung", http.MethodPost, "/orders/1", body, true, http.StatusMethodNotAllowed, false},
		{"DELETE ke detail tidak didukung", http.MethodDelete, "/orders/1", "", true, http.StatusMethodNotAllowed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcPesanan := &fakeOrders{order: pesanan}
			router := newTestRouterLengkap(&fakeAuth{user: pembeli}, &fakeProducts{}, svcPesanan, fakeParser{id: 7})

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.token {
				req.Header.Set("Authorization", "Bearer token-apa-saja")
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if svcPesanan.dipanggil != tt.wantDipanggil {
				t.Errorf("usecase pesanan dipanggil = %v, ingin %v", svcPesanan.dipanggil, tt.wantDipanggil)
			}
			if svcPesanan.dipanggil && svcPesanan.gotUserID != 7 {
				t.Errorf("userID yang sampai ke usecase = %d, ingin 7 (dari token)", svcPesanan.gotUserID)
			}
		})
	}
}
