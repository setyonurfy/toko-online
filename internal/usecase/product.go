package usecase

import (
	"context"
	"strings"
	"toko-online/internal/domain"
	"unicode/utf8"
)

const (
	maxNamaProduk      = 150
	maxDeskripsiProduk = 2000
	maxHargaProduk     = 1_000_000_000
	maxStokProduk      = 1_000_000

	defaultListLimit = 20
	maxListLimit     = 100
)

type ProductUsecase struct {
	products domain.ProductRepository
}

func NewProductUsecase(products domain.ProductRepository) *ProductUsecase {
	return &ProductUsecase{products: products}
}

func (u *ProductUsecase) Create(ctx context.Context, nama, deskripsi string, harga int64, stok int) (*domain.Product, error) {
	p := &domain.Product{
		Nama:      strings.TrimSpace(nama),
		Deskripsi: strings.TrimSpace(deskripsi),
		Harga:     harga,
		Stok:      stok,
	}
	if err := validateProduct(p); err != nil {
		return nil, err
	}
	if err := u.products.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (u *ProductUsecase) Get(ctx context.Context, id int64) (*domain.Product, error) {
	if id <= 0 {
		return nil, domain.ErrProductNotFound
	}
	return u.products.GetByID(ctx, id)
}

func (u *ProductUsecase) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	limit, offset = normalizePage(limit, offset)
	return u.products.List(ctx, limit, offset)
}

func (u *ProductUsecase) Update(ctx context.Context, id int64, nama, deskripsi string, harga int64, stok int) (*domain.Product, error) {
	if id <= 0 {
		return nil, domain.ErrProductNotFound
	}
	p := &domain.Product{
		ID:        id,
		Nama:      strings.TrimSpace(nama),
		Deskripsi: strings.TrimSpace(deskripsi),
		Harga:     harga,
		Stok:      stok,
	}
	if err := validateProduct(p); err != nil {
		return nil, err
	}
	if err := u.products.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (u *ProductUsecase) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return domain.ErrProductNotFound
	}
	return u.products.Delete(ctx, id)
}

func validateProduct(p *domain.Product) error {
	if p.Nama == "" || utf8.RuneCountInString(p.Nama) > maxNamaProduk {
		return domain.ErrInvalidInput
	}
	if utf8.RuneCountInString(p.Deskripsi) > maxDeskripsiProduk {
		return domain.ErrInvalidInput
	}
	if p.Harga <= 0 || p.Harga > maxHargaProduk {
		return domain.ErrInvalidInput
	}
	if p.Stok < 0 || p.Stok > maxStokProduk {
		return domain.ErrInvalidInput
	}
	return nil
}
