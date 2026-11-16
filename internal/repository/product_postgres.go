package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"toko-online/internal/domain"
)

const productColumns = `id, nama, deskripsi, harga, stok, created_at, updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(s scanner, p *domain.Product) error {
	return s.Scan(&p.ID, &p.Nama, &p.Deskripsi, &p.Harga, &p.Stok, &p.CreatedAt, &p.UpdatedAt)
}

type productPostgres struct {
	db *sql.DB
}

func NewProductPostgres(db *sql.DB) domain.ProductRepository {
	return &productPostgres{db: db}
}

func (r *productPostgres) Create(ctx context.Context, p *domain.Product) error {
	const q = `INSERT INTO products (nama, deskripsi, harga, stok)
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, q, p.Nama, p.Deskripsi, p.Harga, p.Stok).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inser produk: %w", err)
	}
	return nil
}

func (r *productPostgres) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	const q = `SELECT ` + productColumns + ` FROM products WHERE id = $1`

	var p domain.Product
	if err := scanProduct(r.db.QueryRowContext(ctx, q, id), &p); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, fmt.Errorf("get product: %w", err)
	}
	return &p, nil
}

func (r *productPostgres) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	const q = `SELECT ` + productColumns + ` FROM products ORDER BY id LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query product: %w", err)
	}
	defer rows.Close()

	produk := make([]domain.Product, 0, limit)
	for rows.Next() {
		var p domain.Product
		if err := scanProduct(rows, &p); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		produk = append(produk, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi product: %w", err)
	}
	return produk, nil
}

func (r *productPostgres) Update(ctx context.Context, p *domain.Product) error {
	const q = `UPDATE products
	SET nama = $1, deskripsi = $2, harga = $3, stok = $4, updated_at = NOW()
	WHERE id = $5
	RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, q, p.Nama, p.Deskripsi, p.Harga, p.Stok, p.ID).Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrProductNotFound
		}
		return fmt.Errorf("update product: %w", err)
	}
	return nil
}

func (r *productPostgres) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete products: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("cek hasil hapus: %w", err)
	}
	if n == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}
