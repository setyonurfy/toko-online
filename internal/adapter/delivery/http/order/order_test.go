package order

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

// ---- tiruan OrderService ----

type fakeOrders struct {
	order *domain.Order
	list  []domain.Order
	err   error

	dipanggil           bool
	gotUserID           int64
	gotOrderID          int64
	gotLines            []domain.OrderLine
	gotLimit, gotOffset int
}

func (f *fakeOrders) Place(ctx context.Context, userID int64, lines []domain.OrderLine) (*domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotLines = slices.Clone(lines)
	return f.order, f.err
}

func (f *fakeOrders) Get(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotOrderID = orderID
	return f.order, f.err
}

func (f *fakeOrders) ListMine(ctx context.Context, userID int64, limit, offset int) ([]domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotLimit, f.gotOffset = limit, offset
	return f.list, f.err
}

// kirimSebagai mengirim request dengan identitas userID di context,
// seperti yang dilakukan middleware Authenticate. userID 0 berarti tanpa identitas.
func kirimSebagai(h http.HandlerFunc, userID int64, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != 0 {
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// ---- test ----

func TestOrderHandler_Create(t *testing.T) {
	pesanan := &domain.Order{
		ID: 1, UserID: 7, Status: domain.OrderPending, Total: 150000,
		Items: []domain.OrderItem{{ProductID: 3, NamaProduk: "Kopi", Harga: 50000, Jumlah: 3, Subtotal: 150000}},
	}
	const bodyValid = `{"items":[{"product_id":3,"jumlah":3}]}`

	tests := []struct {
		name          string
		userID        int64
		body          string
		svc           *fakeOrders
		wantStatus    int
		wantBody      string
		wantLocation  string
		wantDipanggil bool
	}{
		{"sukses", 7, bodyValid, &fakeOrders{order: pesanan}, http.StatusCreated, `"total":150000`, "/orders/1", true},
		{"tanpa identitas", 0, bodyValid, &fakeOrders{}, http.StatusUnauthorized, "tidak terautentikasi", "", false},
		{"user_id dari klien ditolak", 7, `{"user_id":99,"items":[{"product_id":3,"jumlah":3}]}`, &fakeOrders{}, http.StatusBadRequest, "body JSON tidak valid", "", false},
		{"harga dari klien ditolak", 7, `{"items":[{"product_id":3,"jumlah":3,"harga":1}]}`, &fakeOrders{}, http.StatusBadRequest, "body JSON tidak valid", "", false},
		{"json rusak", 7, `{bukan json`, &fakeOrders{}, http.StatusBadRequest, "body JSON tidak valid", "", false},
		{"input tidak valid dari usecase", 7, `{"items":[]}`, &fakeOrders{err: domain.ErrInvalidInput}, http.StatusBadRequest, "input tidak valid", "", true},
		{"stok tidak mencukupi", 7, bodyValid, &fakeOrders{err: domain.ErrInsufficientStock}, http.StatusConflict, "stok tidak mencukupi", "", true},
		{"produk tidak ditemukan", 7, bodyValid, &fakeOrders{err: domain.ErrProductNotFound}, http.StatusUnprocessableEntity, "produk dalam pesanan tidak ditemukan", "", true},
		{"error tak terduga", 7, bodyValid, &fakeOrders{err: errors.New("db mati")}, http.StatusInternalServerError, "terjadi kesalahan pada server", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewOrderHandler(tt.svc)

			rec := kirimSebagai(h.Create, tt.userID, http.MethodPost, "/orders", tt.body)
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
			if got := rec.Result().Header.Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q, ingin %q", got, tt.wantLocation)
			}
			if tt.svc.dipanggil != tt.wantDipanggil {
				t.Errorf("usecase dipanggil = %v, ingin %v", tt.svc.dipanggil, tt.wantDipanggil)
			}
		})
	}
}

func TestOrderHandler_Create_IdentitasDariTokenBukanBody(t *testing.T) {
	svc := &fakeOrders{order: &domain.Order{ID: 1}}
	h := NewOrderHandler(svc)

	rec := kirimSebagai(h.Create, 7, http.MethodPost, "/orders",
		`{"items":[{"product_id":3,"jumlah":2},{"product_id":5,"jumlah":1}]}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, ingin 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.gotUserID != 7 {
		t.Errorf("userID yang diteruskan = %d, ingin 7 (dari token)", svc.gotUserID)
	}
	want := []domain.OrderLine{{ProductID: 3, Jumlah: 2}, {ProductID: 5, Jumlah: 1}}
	if !slices.Equal(svc.gotLines, want) {
		t.Errorf("barang yang diteruskan = %v, ingin %v", svc.gotLines, want)
	}
}

func TestOrderHandler_Get(t *testing.T) {
	milikSendiri := &domain.Order{ID: 5, UserID: 7, Status: domain.OrderPending, Total: 1000}

	tests := []struct {
		name          string
		userID        int64
		path          string
		svc           *fakeOrders
		wantStatus    int
		wantOrderID   int64
		wantDipanggil bool
	}{
		{"milik sendiri", 7, "/orders/5", &fakeOrders{order: milikSendiri}, http.StatusOK, 5, true},
		{"tidak ada atau bukan milik", 7, "/orders/5", &fakeOrders{err: domain.ErrOrderNotFound}, http.StatusNotFound, 5, true},
		{"tanpa identitas", 0, "/orders/5", &fakeOrders{}, http.StatusUnauthorized, 0, false},
		{"id bukan angka", 7, "/orders/abc", &fakeOrders{}, http.StatusNotFound, 0, false},
		{"id nol", 7, "/orders/0", &fakeOrders{}, http.StatusNotFound, 0, false},
		{"id berawalan plus", 7, "/orders/+5", &fakeOrders{}, http.StatusNotFound, 0, false},
		{"id berawalan nol", 7, "/orders/05", &fakeOrders{}, http.StatusNotFound, 0, false},
		{"path berlebih", 7, "/orders/5/items", &fakeOrders{}, http.StatusNotFound, 0, false},
		{"id kosong", 7, "/orders/", &fakeOrders{}, http.StatusNotFound, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewOrderHandler(tt.svc)

			rec := kirimSebagai(h.Get, tt.userID, http.MethodGet, tt.path, "")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d", rec.Code, tt.wantStatus)
			}
			if tt.svc.dipanggil != tt.wantDipanggil {
				t.Errorf("usecase dipanggil = %v, ingin %v", tt.svc.dipanggil, tt.wantDipanggil)
			}
			if tt.svc.gotOrderID != tt.wantOrderID {
				t.Errorf("ID pesanan yang diteruskan = %d, ingin %d", tt.svc.gotOrderID, tt.wantOrderID)
			}
			if tt.svc.dipanggil && tt.svc.gotUserID != tt.userID {
				t.Errorf("userID yang diteruskan = %d, ingin %d", tt.svc.gotUserID, tt.userID)
			}
		})
	}
}

func TestOrderHandler_List(t *testing.T) {
	t.Run("hasil kosong tetap berbentuk array", func(t *testing.T) {
		h := NewOrderHandler(&fakeOrders{}) // list = nil

		rec := kirimSebagai(h.List, 7, http.MethodGet, "/orders", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, ingin 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"data":[]`) {
			t.Errorf("body = %s, harus memuat \"data\":[] (bukan null)", rec.Body.String())
		}
	})

	t.Run("identitas, limit, dan offset diteruskan", func(t *testing.T) {
		svc := &fakeOrders{}
		h := NewOrderHandler(svc)

		kirimSebagai(h.List, 7, http.MethodGet, "/orders?limit=5&offset=10", "")

		if svc.gotUserID != 7 || svc.gotLimit != 5 || svc.gotOffset != 10 {
			t.Errorf("usecase menerima (user=%d, limit=%d, offset=%d), ingin (7, 5, 10)",
				svc.gotUserID, svc.gotLimit, svc.gotOffset)
		}
	})

	t.Run("tanpa identitas", func(t *testing.T) {
		svc := &fakeOrders{}
		h := NewOrderHandler(svc)

		rec := kirimSebagai(h.List, 0, http.MethodGet, "/orders", "")

		if rec.Code != http.StatusUnauthorized || svc.dipanggil {
			t.Fatalf("status = %d, dipanggil = %v; ingin 401 dan tidak dipanggil", rec.Code, svc.dipanggil)
		}
	})

	t.Run("limit bukan angka", func(t *testing.T) {
		svc := &fakeOrders{}
		h := NewOrderHandler(svc)

		rec := kirimSebagai(h.List, 7, http.MethodGet, "/orders?limit=abc", "")

		if rec.Code != http.StatusBadRequest || svc.dipanggil {
			t.Fatalf("status = %d, dipanggil = %v; ingin 400 dan tidak dipanggil", rec.Code, svc.dipanggil)
		}
	})

	t.Run("error tak terduga", func(t *testing.T) {
		h := NewOrderHandler(&fakeOrders{err: errors.New("db mati")})

		rec := kirimSebagai(h.List, 7, http.MethodGet, "/orders", "")

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, ingin 500", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "db mati") {
			t.Errorf("detail error internal bocor ke klien: %s", rec.Body.String())
		}
	})
}
