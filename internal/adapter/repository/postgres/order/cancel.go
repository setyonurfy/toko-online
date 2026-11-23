package order

import (
	"context"
	"database/sql"
	"fmt"

	"toko-online/internal/domain"
)

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

	if err := kembalikanStok(ctx, tx, id); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit pembatalan: %w", err)
	}

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

func kembalikanStok(ctx context.Context, tx *sql.Tx, orderID int64) error {
	kembali, err := barangYangDikembalikan(ctx, tx, orderID)
	if err != nil {
		return err
	}

	const q = `UPDATE products SET stok = stok + $1, updated_at = NOW() WHERE id = $2`
	for _, b := range kembali {
		res, err := tx.ExecContext(ctx, q, b.jumlah, b.productID)
		if err != nil {
			return fmt.Errorf("kembalikan stok produk %d: %w", b.productID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("cek hasil kembalikan stok produk %d: %w", b.productID, err)
		}
		if n != 1 {
			return fmt.Errorf("produk %d tidak ditemukan saat mengembalikan stok", b.productID)
		}
	}
	return nil
}
