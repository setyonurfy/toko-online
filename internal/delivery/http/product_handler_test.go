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

// ---- tiruan ProductService ----

type fakeProducts struct {
	product *domain.Product
	list    []domain.Product
	err     error

	dipanggil           bool
	gotID               int64
	gotLimit, gotOffset int
	gotStok             int
}

func (f *fakeProducts) Create(ctx context.Context, nama, deskripsi string, harga int64, stok int) (*domain.Product, error) {
	f.dipanggil = true
	f.gotStok = stok
	return f.product, f.err
}

func (f *fakeProducts) Get(ctx context.Context, id int64) (*domain.Product, error) {
	f.dipanggil = true
	f.gotID = id
	return f.product, f.err
}

func (f *fakeProducts) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	f.dipanggil = true
	f.gotLimit, f.gotOffset = limit, offset
	return f.list, f.err
}

func (f *fakeProducts) Update(ctx context.Context, id int64, nama, deskripsi string, harga int64, stok int) (*domain.Product, error) {
	f.dipanggil = true
	f.gotID = id
	f.gotStok = stok
	return f.product, f.err
}

func (f *fakeProducts) Delete(ctx context.Context, id int64) error {
	f.dipanggil = true
	f.gotID = id
	return f.err
}

func kirim(h http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// ---- test ----

func TestProductHandler_Create(t *testing.T) {
	kopi := &domain.Product{ID: 1, Nama: "Kopi", Harga: 55000, Stok: 10}
	const bodyValid = `{"nama":"Kopi","deskripsi":"Enak","harga":55000,"stok":10}`

	tests := []struct {
		name          string
		body          string
		svc           *fakeProducts
		wantStatus    int
		wantBody      string
		wantLocation  string
		wantDipanggil bool
	}{
		{"sukses", bodyValid, &fakeProducts{product: kopi}, http.StatusCreated, `"nama":"Kopi"`, "/products/1", true},
		{"stok nol eksplisit diterima", `{"nama":"Kopi","harga":55000,"stok":0}`, &fakeProducts{product: kopi}, http.StatusCreated, `"id":1`, "/products/1", true},
		{"stok tidak dikirim ditolak", `{"nama":"Kopi","harga":55000}`, &fakeProducts{}, http.StatusBadRequest, "stok wajib diisi", "", false},
		{"harga bukan angka", `{"nama":"Kopi","harga":"murah","stok":1}`, &fakeProducts{}, http.StatusBadRequest, "body JSON tidak valid", "", false},
		{"harga pecahan ditolak", `{"nama":"Kopi","harga":55000.5,"stok":1}`, &fakeProducts{}, http.StatusBadRequest, "body JSON tidak valid", "", false},
		{"field asing ditolak", `{"nama":"Kopi","harga":1,"stok":1,"id":99}`, &fakeProducts{}, http.StatusBadRequest, "body JSON tidak valid", "", false},
		{"json rusak", `{bukan json`, &fakeProducts{}, http.StatusBadRequest, "body JSON tidak valid", "", false},
		{"input tidak valid dari usecase", bodyValid, &fakeProducts{err: domain.ErrInvalidInput}, http.StatusBadRequest, "input tidak valid", "", true},
		{"error tak terduga", bodyValid, &fakeProducts{err: errors.New("db mati")}, http.StatusInternalServerError, "terjadi kesalahan pada server", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewProductHandler(tt.svc)

			rec := kirim(h.Create, http.MethodPost, "/products", tt.body)
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

func TestProductHandler_List(t *testing.T) {
	t.Run("hasil kosong tetap berbentuk array", func(t *testing.T) {
		h := NewProductHandler(&fakeProducts{}) // list = nil

		rec := kirim(h.List, http.MethodGet, "/products", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, ingin 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"data":[]`) {
			t.Errorf("body = %s, harus memuat \"data\":[] (bukan null)", rec.Body.String())
		}
	})

	t.Run("limit dan offset diteruskan ke usecase", func(t *testing.T) {
		svc := &fakeProducts{}
		h := NewProductHandler(svc)

		kirim(h.List, http.MethodGet, "/products?limit=5&offset=10", "")

		if svc.gotLimit != 5 || svc.gotOffset != 10 {
			t.Errorf("usecase menerima (limit=%d, offset=%d), ingin (5, 10)", svc.gotLimit, svc.gotOffset)
		}
	})

	t.Run("limit bukan angka", func(t *testing.T) {
		svc := &fakeProducts{}
		h := NewProductHandler(svc)

		rec := kirim(h.List, http.MethodGet, "/products?limit=abc", "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, ingin 400", rec.Code)
		}
		if svc.dipanggil {
			t.Error("usecase tidak boleh dipanggil saat parameter salah")
		}
	})

	t.Run("error tak terduga", func(t *testing.T) {
		h := NewProductHandler(&fakeProducts{err: errors.New("db mati")})

		rec := kirim(h.List, http.MethodGet, "/products", "")

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, ingin 500", rec.Code)
		}
	})
}

func TestProductHandler_Get(t *testing.T) {
	kopi := &domain.Product{ID: 5, Nama: "Kopi"}

	tests := []struct {
		name          string
		path          string
		svc           *fakeProducts
		wantStatus    int
		wantID        int64
		wantDipanggil bool
	}{
		{"ada", "/products/5", &fakeProducts{product: kopi}, http.StatusOK, 5, true},
		{"tidak ada", "/products/5", &fakeProducts{err: domain.ErrProductNotFound}, http.StatusNotFound, 5, true},
		{"id bukan angka", "/products/abc", &fakeProducts{}, http.StatusNotFound, 0, false},
		{"id nol", "/products/0", &fakeProducts{}, http.StatusNotFound, 0, false},
		{"id negatif", "/products/-3", &fakeProducts{}, http.StatusNotFound, 0, false},
		{"id berawalan plus", "/products/+5", &fakeProducts{}, http.StatusNotFound, 0, false},
		{"id berawalan nol", "/products/05", &fakeProducts{}, http.StatusNotFound, 0, false},
		{"path berlebih", "/products/5/extra", &fakeProducts{}, http.StatusNotFound, 0, false},
		{"id kosong", "/products/", &fakeProducts{}, http.StatusNotFound, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewProductHandler(tt.svc)

			rec := kirim(h.Get, http.MethodGet, tt.path, "")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d", rec.Code, tt.wantStatus)
			}
			if tt.svc.dipanggil != tt.wantDipanggil {
				t.Errorf("usecase dipanggil = %v, ingin %v", tt.svc.dipanggil, tt.wantDipanggil)
			}
			if tt.svc.gotID != tt.wantID {
				t.Errorf("ID yang diteruskan = %d, ingin %d", tt.svc.gotID, tt.wantID)
			}
		})
	}
}

func TestProductHandler_Update(t *testing.T) {
	kopi := &domain.Product{ID: 5, Nama: "Kopi Gayo"}
	const body = `{"nama":"Kopi Gayo","harga":60000,"stok":8}`

	t.Run("sukses", func(t *testing.T) {
		svc := &fakeProducts{product: kopi}
		rec := kirim(NewProductHandler(svc).Update, http.MethodPut, "/products/5", body)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, ingin 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if svc.gotID != 5 || svc.gotStok != 8 {
			t.Errorf("usecase menerima (id=%d, stok=%d), ingin (5, 8)", svc.gotID, svc.gotStok)
		}
	})

	t.Run("stok tidak dikirim tidak boleh menjadi nol diam-diam", func(t *testing.T) {
		svc := &fakeProducts{product: kopi}
		rec := kirim(NewProductHandler(svc).Update, http.MethodPut, "/products/5", `{"nama":"Kopi","harga":60000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, ingin 400", rec.Code)
		}
		if svc.dipanggil {
			t.Error("usecase tidak boleh dipanggil")
		}
	})

	t.Run("produk tidak ada", func(t *testing.T) {
		svc := &fakeProducts{err: domain.ErrProductNotFound}
		rec := kirim(NewProductHandler(svc).Update, http.MethodPut, "/products/5", body)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, ingin 404", rec.Code)
		}
	})

	t.Run("id tidak sah", func(t *testing.T) {
		svc := &fakeProducts{}
		rec := kirim(NewProductHandler(svc).Update, http.MethodPut, "/products/abc", body)

		if rec.Code != http.StatusNotFound || svc.dipanggil {
			t.Fatalf("status = %d, dipanggil = %v; ingin 404 dan tidak dipanggil", rec.Code, svc.dipanggil)
		}
	})
}

func TestProductHandler_Delete(t *testing.T) {
	t.Run("sukses tanpa body", func(t *testing.T) {
		svc := &fakeProducts{}
		rec := kirim(NewProductHandler(svc).Delete, http.MethodDelete, "/products/5", "")

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, ingin 204", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("body = %q, 204 tidak boleh punya body", rec.Body.String())
		}
		if svc.gotID != 5 {
			t.Errorf("ID = %d, ingin 5", svc.gotID)
		}
	})

	t.Run("produk tidak ada", func(t *testing.T) {
		svc := &fakeProducts{err: domain.ErrProductNotFound}
		rec := kirim(NewProductHandler(svc).Delete, http.MethodDelete, "/products/5", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, ingin 404", rec.Code)
		}
	})
}
