package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"toko-online/internal/domain"
)

const orderColumns = `id, user_id, status, total, created_at`

func scanOrder(s scanner, o *domain.Order) error {
	return s.Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt)
}

type orderPostgres struct {
	db *sql.DB
}

func NewOrderPostgres(db *sql.DB) domain.OrderRepository {
	return &orderPostgres{db: db}
}

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

// stokKembali adalah jumlah barang satu produk yang harus dikembalikan ke stok.
type stokKembali struct {
	productID int64
	jumlah    int64
}

func (r *orderPostgres) Cancel(ctx context.Context, id int64) (*domain.Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mulai transaksi: %w", err)
	}
	defer tx.Rollback()

	const qStatus = `UPDATE orders SET status = $1 WHERE id = $2 AND status = $3`
	res, err := tx.ExecContext(ctx, qStatus, string(domain.OrderCancelled), id, string(domain.OrderPending))
	if err != nil {
		return nil, fmt.Errorf("ubah status pesanan: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("cek hasil ubah status: %w", err)
	}
	if n == 0 {
		// Tidak ada baris yang cocok: pesanannya tidak ada, atau statusnya bukan pending.
		var ada bool
		const qAda = `SELECT EXISTS (SELECT 1 FROM orders WHERE id = $1)`
		if err := tx.QueryRowContext(ctx, qAda, id).Scan(&ada); err != nil {
			return nil, fmt.Errorf("cek pesanan: %w", err)
		}
		if !ada {
			return nil, domain.ErrOrderNotFound
		}
		return nil, domain.ErrOrderNotCancellable
	}

	// 2. Ambil jumlah yang harus dikembalikan per produk, terurut naik menurut ID.
	kembali, err := barangYangDikembalikan(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	// 3. Kembalikan stok satu per satu, dengan urutan yang sama seperti saat
	// pesanan dibuat, supaya tidak saling mengunci dengan pesanan lain.
	const qStok = `UPDATE products SET stok = stok + $1, updated_at = NOW() WHERE id = $2`
	for _, b := range kembali {
		resStok, err := tx.ExecContext(ctx, qStok, b.jumlah, b.productID)
		if err != nil {
			return nil, fmt.Errorf("kembalikan stok produk %d: %w", b.productID, err)
		}
		nStok, err := resStok.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("cek hasil kembalikan stok produk %d: %w", b.productID, err)
		}
		if nStok != 1 {
			return nil, fmt.Errorf("produk %d tidak ditemukan saat mengembalikan stok", b.productID)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit pembatalan: %w", err)
	}

	// Pesanan yang sudah dibatalkan tidak berubah lagi, jadi aman dibaca setelah commit.
	return r.GetByID(ctx, id)
}

func barangYangDikembalikan(ctx context.Context, tx *sql.Tx, orderID int64) ([]stokKembali, error) {
	const q = `
		SELECT product_id, SUM(jumlah)
		FROM order_items
		WHERE order_id = $1
		GROUP BY product_id
		ORDER BY product_id`

	rows, err := tx.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("query barang pesanan: %w", err)
	}
	defer rows.Close()

	var hasil []stokKembali
	for rows.Next() {
		var b stokKembali
		if err := rows.Scan(&b.productID, &b.jumlah); err != nil {
			return nil, fmt.Errorf("scan barang pesanan: %w", err)
		}
		hasil = append(hasil, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi barang pesanan: %w", err)
	}
	return hasil, nil
}
