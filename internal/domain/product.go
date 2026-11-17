package domain

import (
	"context"
	"errors"
	"time"
)

type Product struct {
	ID        int64     `json:"id"`
	Nama      string    `json:"nama"`
	Deskripsi string    `json:"deskripsi"`
	Harga     int64     `json:"harga"`
	Stok      int       `json:"stok"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ErrProductNotFound = errors.New("produk tidak ditemukan")
var ErrProductInUse = errors.New("produk sudah pernah dipesan dan tidak dapat dihapus")

type ProductRepository interface {
	Create(ctx context.Context, p *Product) error
	GetByID(ctx context.Context, id int64) (*Product, error)
	List(ctx context.Context, limit, offset int) ([]Product, error)
	Update(ctx context.Context, p *Product) error
	Delete(ctx context.Context, id int64) error
}
