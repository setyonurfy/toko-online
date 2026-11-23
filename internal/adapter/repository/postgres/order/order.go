package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"toko-online/internal/domain"
)

const orderColumns = `id, user_id, status, total, created_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(s scanner, o *domain.Order) error {
	return s.Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt)
}

type orderPostgres struct {
	db *sql.DB
}

func NewOrderPostgres(db *sql.DB) domain.OrderRepository {
	return &orderPostgres{db: db}
}

func (r *orderPostgres) GetByID(ctx context.Context, id int64) (*domain.Order, error) {
	const qOrder = `SELECT ` + orderColumns + ` FROM orders WHERE id = $1`

	var o domain.Order
	if err := scanOrder(r.db.QueryRowContext(ctx, qOrder, id), &o); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("get pesanan: %w", err)
	}

	const qItems = `
		SELECT product_id, nama_produk, harga, jumlah, subtotal
		FROM order_items
		WHERE order_id = $1
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, qItems, id)
	if err != nil {
		return nil, fmt.Errorf("query barang pesanan: %w", err)
	}
	defer rows.Close()

	o.Items = make([]domain.OrderItem, 0)
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(&it.ProductID, &it.NamaProduk, &it.Harga, &it.Jumlah, &it.Subtotal); err != nil {
			return nil, fmt.Errorf("scan barang pesanan: %w", err)
		}
		o.Items = append(o.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi barang pesanan: %w", err)
	}
	return &o, nil
}

func (r *orderPostgres) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Order, error) {
	const q = `SELECT ` + orderColumns + ` FROM orders WHERE user_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query pesanan: %w", err)
	}
	defer rows.Close()

	hasil := make([]domain.Order, 0, limit)
	for rows.Next() {
		var o domain.Order
		if err := scanOrder(rows, &o); err != nil {
			return nil, fmt.Errorf("scan pesanan: %w", err)
		}
		hasil = append(hasil, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi pesanan: %w", err)
	}
	return hasil, nil
}
