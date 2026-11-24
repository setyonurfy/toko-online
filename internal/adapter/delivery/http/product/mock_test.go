package product

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"toko-online/internal/domain"
)

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
