package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"toko-online/internal/domain"
)

const paymentColumns = `id, order_id, reference, amount, status, created_at, paid_at`

func scanPayment(s scanner, p *domain.Payment) error {
	return s.Scan(&p.ID, &p.OrderID, &p.Reference, &p.Amount, &p.Status, &p.CreatedAt, &p.PaidAt)
}

type paymentPostgres struct {
	db *sql.DB
}

func NewPaymentPostgres(db *sql.DB) domain.PaymentRepository {
	return &paymentPostgres{db: db}
}

func (r *paymentPostgres) GetPendingByOrder(ctx context.Context, orderID int64) (*domain.Payment, error) {
	const q = `SELECT ` + paymentColumns + ` FROM payments WHERE order_id = $1 AND status = $2`

	var p domain.Payment
	err := scanPayment(r.db.QueryRowContext(ctx, q, orderID, string(domain.PaymentPending)), &p)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("get tagihan pending: %w", err)
	}
	return &p, nil
}

func (r *paymentPostgres) CreatePending(ctx context.Context, orderID int64, reference string) (*domain.Payment, error) {
	const q = `
		INSERT INTO payments (order_id, reference, amount)
		SELECT id, $2::text, total FROM orders WHERE id = $1 AND status = $3
		RETURNING ` + paymentColumns

	var p domain.Payment
	err := scanPayment(r.db.QueryRowContext(ctx, q, orderID, reference, string(domain.OrderPending)), &p)
	if err == nil {
		return &p, nil
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "uq_payments_order_pending" {
		return r.GetPendingByOrder(ctx, orderID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("simpan tagihan: %w", err)
	}

	var ada bool
	const qAda = `SELECT EXISTS (SELECT 1 FROM orders WHERE id = $1)`
	if err := r.db.QueryRowContext(ctx, qAda, orderID).Scan(&ada); err != nil {
		return nil, fmt.Errorf("cek pesanan: %w", err)
	}
	if !ada {
		return nil, domain.ErrOrderNotFound
	}
	return nil, domain.ErrOrderNotPayable
}

func (r *paymentPostgres) ApplyPaid(ctx context.Context, reference string, amount int64) (domain.PaymentOutcome, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("mulai transaksi: %w", err)
	}
	defer tx.Rollback()

	var (
		paymentID int64
		orderID   int64
		tagihan   int64
		status    domain.PaymentStatus
	)
	const qKunci = `SELECT id, order_id, amount, status FROM payments WHERE reference = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, qKunci, reference).Scan(&paymentID, &orderID, &tagihan, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrPaymentNotFound
		}
		return "", fmt.Errorf("kunci pembayaran: %w", err)
	}

	switch status {
	case domain.PaymentPaid:
		return domain.OutcomeAlreadyPaid, nil
	case domain.PaymentRefundNeeded:
		return domain.OutcomeRefundNeeded, nil
	}

	if amount != tagihan {
		return "", domain.ErrAmountMismatch
	}

	const qPesanan = `UPDATE orders SET status = $1 WHERE id = $2 AND status = $3`
	res, err := tx.ExecContext(ctx, qPesanan, string(domain.OrderPaid), orderID, string(domain.OrderPending))
	if err != nil {
		return "", fmt.Errorf("ubah status pesanan: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("cek hasil ubah status: %w", err)
	}

	hasil := domain.OutcomePaid
	if n == 1 {
		_, err = tx.ExecContext(ctx,
			`UPDATE payments SET status = $1, paid_at = NOW() WHERE id = $2`,
			string(domain.PaymentPaid), paymentID)
	} else {
		hasil = domain.OutcomeRefundNeeded
		_, err = tx.ExecContext(ctx,
			`UPDATE payments SET status = $1 WHERE id = $2`,
			string(domain.PaymentRefundNeeded), paymentID)
	}
	if err != nil {
		return "", fmt.Errorf("ubah status pembayaran: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit pembayaran: %w", err)
	}
	return hasil, nil
}
