package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"toko-online/internal/domain"
)

func (r *orderPostgres) Create(ctx context.Context, userID int64, lines []domain.OrderLine) (*domain.Order, error) {
	if len(lines) == 0 {
		return nil, domain.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mulai transaksi: %w", err)
	}
	defer tx.Rollback()

	items := make([]domain.OrderItem, 0, len(lines))
	var total int64
	for _, l := range lines {
		item, err := reserveStock(ctx, tx, l)
		if err != nil {
			return nil, err
		}

		total += item.Subtotal
		items = append(items, item)
	}

	o := &domain.Order{UserID: userID, Total: total, Items: items}

	const qOrder = `
		INSERT INTO orders (user_id, total)
		VALUES ($1, $2)
		RETURNING id, status, created_at`
	if err := tx.QueryRowContext(ctx, qOrder, userID, total).Scan(&o.ID, &o.Status, &o.CreatedAt); err != nil {
		return nil, fmt.Errorf("simpan pesanan: %w", err)
	}

	const qItem = `
		INSERT INTO order_items (order_id, product_id, nama_produk, harga, jumlah, subtotal)
		VALUES ($1, $2, $3, $4, $5, $6)`
	stmt, err := tx.PrepareContext(ctx, qItem)
	if err != nil {
		return nil, fmt.Errorf("siapkan insert barang: %w", err)
	}
	defer stmt.Close()

	for _, it := range items {
		if _, err := stmt.ExecContext(ctx, o.ID, it.ProductID, it.NamaProduk, it.Harga, it.Jumlah, it.Subtotal); err != nil {
			return nil, fmt.Errorf("simpan barang pesanan: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit pesanan: %w", err)
	}
	return o, nil
}

func reserveStock(ctx context.Context, tx *sql.Tx, l domain.OrderLine) (domain.OrderItem, error) {
	const q = `
		UPDATE products
		SET stok = stok - $1, updated_at = NOW()
		WHERE id = $2 AND stok >= $1
		RETURNING nama, harga`

	item := domain.OrderItem{ProductID: l.ProductID, Jumlah: l.Jumlah}

	err := tx.QueryRowContext(ctx, q, l.Jumlah, l.ProductID).Scan(&item.NamaProduk, &item.Harga)
	if err == nil {
		item.Subtotal = item.Harga * int64(l.Jumlah)
		return item, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return item, fmt.Errorf("kurangi stok produk %d: %w", l.ProductID, err)
	}

	var ada bool
	const qAda = `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1)`
	if err := tx.QueryRowContext(ctx, qAda, l.ProductID).Scan(&ada); err != nil {
		return item, fmt.Errorf("cek produk %d: %w", l.ProductID, err)
	}
	if !ada {
		return item, domain.ErrProductNotFound
	}
	return item, domain.ErrInsufficientStock
}
