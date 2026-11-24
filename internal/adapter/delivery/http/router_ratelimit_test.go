package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

type fakeParserMap map[string]int64

func (f fakeParserMap) Parse(token string) (int64, error) {
	id, ok := f[token]
	if !ok {
		return 0, errors.New("token tidak dikenal")
	}
	return id, nil
}

func newTestRouterBatas(svc *fakeAuth, read, action, admin *middleware.RateLimiter) http.Handler {
	return NewRouter(Dependencies{
		Auth:          NewAuthHandler(svc),
		Products:      NewProductHandler(&fakeProducts{}),
		Orders:        NewOrderHandler(&fakeOrders{}),
		Payments:      NewPaymentHandler(&fakePayments{}),
		Tokens:        fakeParserMap{"a": 7, "b": 8},
		Users:         svc,
		AuthLimiter:   NewRateLimiter(6000, 1000),
		ReadLimiter:   read,
		ActionLimiter: action,
		AdminLimiter:  admin,
	})
}

func panggil(router http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func longgar() *middleware.RateLimiter { return NewRateLimiter(6000, 1000) }

func ketat() *middleware.RateLimiter { return NewRateLimiter(1, 1) }

func TestRouter_BatasPerPengguna_Baca(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Role: domain.RoleCustomer}}
	router := newTestRouterBatas(svc, ketat(), longgar(), longgar())

	if rec := panggil(router, http.MethodGet, "/orders", "a"); rec.Code == http.StatusTooManyRequests {
		t.Fatalf("permintaan baca pertama tidak boleh ditolak, status = %d", rec.Code)
	}

	rec := panggil(router, http.MethodGet, "/orders", "a")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("permintaan baca kedua: status = %d, ingin 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("header Retry-After harus ada pada respons 429")
	}

	if rec := panggil(router, http.MethodGet, "/me", "a"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("/me berbagi jatah baca: status = %d, ingin 429", rec.Code)
	}
	if rec := panggil(router, http.MethodGet, "/orders", "b"); rec.Code == http.StatusTooManyRequests {
		t.Errorf("pengguna lain tidak boleh ikut terblokir, status = %d", rec.Code)
	}
	if rec := panggil(router, http.MethodPost, "/orders", "a"); rec.Code == http.StatusTooManyRequests {
		t.Errorf("jatah baca habis tidak boleh memblokir aksi pesanan, status = %d", rec.Code)
	}
}

func TestRouter_BatasPerPengguna_Aksi(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Role: domain.RoleCustomer}}
	router := newTestRouterBatas(svc, longgar(), ketat(), longgar())

	if rec := panggil(router, http.MethodPost, "/orders/5/cancel", "a"); rec.Code == http.StatusTooManyRequests {
		t.Fatalf("aksi pertama tidak boleh ditolak, status = %d", rec.Code)
	}
	if rec := panggil(router, http.MethodPost, "/orders/5/pay", "a"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("aksi kedua (bayar) berbagi jatah dengan batal: status = %d, ingin 429", rec.Code)
	}
	if rec := panggil(router, http.MethodGet, "/orders/5", "a"); rec.Code == http.StatusTooManyRequests {
		t.Errorf("jatah aksi habis tidak boleh memblokir baca, status = %d", rec.Code)
	}
	if rec := panggil(router, http.MethodPost, "/orders/5/cancel", "b"); rec.Code == http.StatusTooManyRequests {
		t.Errorf("pengguna lain tidak boleh ikut terblokir, status = %d", rec.Code)
	}
}

func TestRouter_BatasPerPengguna_Admin(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Role: domain.RoleAdmin}}
	router := newTestRouterBatas(svc, longgar(), longgar(), ketat())

	if rec := panggil(router, http.MethodPost, "/products", "a"); rec.Code == http.StatusTooManyRequests {
		t.Fatalf("permintaan admin pertama tidak boleh ditolak, status = %d", rec.Code)
	}
	if rec := panggil(router, http.MethodPost, "/products", "a"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("permintaan admin kedua: status = %d, ingin 429", rec.Code)
	}
	if rec := panggil(router, http.MethodPost, "/products", "b"); rec.Code == http.StatusTooManyRequests {
		t.Errorf("pengguna lain tidak boleh ikut terblokir, status = %d", rec.Code)
	}
}

func TestRouter_BatasPerPengguna_JalurBebas(t *testing.T) {
	svc := &fakeAuth{user: &domain.User{ID: 7, Role: domain.RoleCustomer}}
	router := newTestRouterBatas(svc, ketat(), ketat(), ketat())

	for _, p := range []struct{ method, path string }{
		{http.MethodPost, "/webhooks/payments"},
		{http.MethodGet, "/products"},
		{http.MethodGet, "/health"},
	} {
		for i := 1; i <= 5; i++ {
			if rec := panggil(router, p.method, p.path, ""); rec.Code == http.StatusTooManyRequests {
				t.Fatalf("%s %s ke-%d tidak boleh dibatasi per pengguna, status = %d", p.method, p.path, i, rec.Code)
			}
		}
	}
}
