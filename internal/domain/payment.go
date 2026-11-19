package domain

import (
	"context"
	"errors"
	"time"
)

const EventPaymentPaid = "payment.paid"

type Charge struct {
	Reference string
}

type GatewayEvent struct {
	Type      string
	Reference string
	Amount    int64 // dalam rupiah
}

var (
	ErrInvalidSignature = errors.New("tanda tangan webhook tidak valid")
	ErrStaleWebhook     = errors.New("webhook kedaluwarsa")
	ErrMalformedWebhook = errors.New("isi webhook tidak valid")
	ErrPaymentNotFound  = errors.New("pembayaran tidak ditemukan")
	ErrOrderNotPayable  = errors.New("pesanan tidak dapat dibayar")
	ErrAmountMismatch   = errors.New("jumlah pembayaran tidak sesuai")
)

type PaymentStatus string

const (
	PaymentPending      PaymentStatus = "pending"
	PaymentPaid         PaymentStatus = "paid"
	PaymentRefundNeeded PaymentStatus = "refund_needed"
)

type Payment struct {
	ID        int64         `json:"id"`
	OrderID   int64         `json:"order_id"`
	Reference string        `json:"reference"`
	Amount    int64         `json:"amount"`
	Status    PaymentStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	PaidAt    *time.Time    `json:"paid_at,omitempty"`
}

type PaymentOutcome string

const (
	OutcomePaid         PaymentOutcome = "paid"
	OutcomeAlreadyPaid  PaymentOutcome = "already_paid"
	OutcomeRefundNeeded PaymentOutcome = "refund_needed"
	OutcomeIgnored      PaymentOutcome = "ignored"
)

type PaymentRepository interface {
	GetPendingByOrder(ctx context.Context, orderID int64) (*Payment, error)
	CreatePending(ctx context.Context, orderID int64, reference string) (*Payment, error)
	ApplyPaid(ctx context.Context, reference string, amount int64) (PaymentOutcome, error)
}
