package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"toko-online/internal/domain"
)

func (r *orderPostgres) ExpirePending(ctx context.Context, cutoff time.Time, batch int) (int, error) {
	if batch <= 0 {
		return 0, domain.ErrInvalidInput
	}

	const q = `
		SELECT id FROM orders
		WHERE status = $1 AND created_at < $2
		ORDER BY created_at, id
		LIMIT $3`
	rows, err := r.db.QueryContext(ctx, q, string(domain.OrderPending), cutoff, batch)
	if err != nil {
		return 0, fmt.Errorf("cari pesanan kedaluwarsa: %w", err)
	}
	ids := make([]int64, 0, batch)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan pesanan kedaluwarsa: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterasi pesanan kedaluwarsa: %w", err)
	}
	rows.Close()

	var galat []error
	kedaluwarsa := 0
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			galat = append(galat, err)
			break
		}
		ok, err := r.expireOne(ctx, id, cutoff)
		if err != nil {
			galat = append(galat, fmt.Errorf("pesanan %d: %w", id, err))
			continue
		}
		if ok {
			kedaluwarsa++
		}
	}
	return kedaluwarsa, errors.Join(galat...)
}

func (r *orderPostgres) expireOne(ctx context.Context, id int64, cutoff time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("mulai transaksi: %w", err)
	}
	defer tx.Rollback()

	const q = `UPDATE orders SET status = $1 WHERE id = $2 AND status = $3 AND created_at < $4`
	res, err := tx.ExecContext(ctx, q, string(domain.OrderExpired), id, string(domain.OrderPending), cutoff)
	if err != nil {
		return false, fmt.Errorf("ubah status pesanan: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("cek hasil ubah status: %w", err)
	}
	if n == 0 {
		return false, nil
	}

	if err := kembalikanStok(ctx, tx, id); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit kedaluwarsa: %w", err)
	}
	return true, nil
}
