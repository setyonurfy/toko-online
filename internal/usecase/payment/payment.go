package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"toko-online/internal/domain"
)

type PaymentGateway interface {
	CreateCharge(ctx context.Context, orderID, amount int64) (domain.Charge, error)
	ParseWebhook(body []byte, timestamp, signature string) (domain.GatewayEvent, error)
}

type OrderReader interface {
	Get(ctx context.Context, userID, orderID int64) (*domain.Order, error)
}

type PaymentUsecase struct {
	orders   OrderReader
	payments domain.PaymentRepository
	gateway  PaymentGateway
}

func NewPaymentUsecase(orders OrderReader, payments domain.PaymentRepository, gateway PaymentGateway) *PaymentUsecase {
	return &PaymentUsecase{orders: orders, payments: payments, gateway: gateway}
}

func (u *PaymentUsecase) Pay(ctx context.Context, userID, orderID int64) (*domain.Payment, error) {
	order, err := u.orders.Get(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.OrderPending {
		return nil, domain.ErrOrderNotPayable
	}

	existing, err := u.payments.GetPendingByOrder(ctx, order.ID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrPaymentNotFound) {
		return nil, err
	}

	charge, err := u.gateway.CreateCharge(ctx, order.ID, order.Total)
	if err != nil {
		return nil, fmt.Errorf("buat tagihan di gateway: %w", err)
	}
	return u.payments.CreatePending(ctx, order.ID, charge.Reference)
}

func (u *PaymentUsecase) HandleWebhook(ctx context.Context, body []byte, timestamp, signature string) (domain.PaymentOutcome, error) {
	event, err := u.gateway.ParseWebhook(body, timestamp, signature)
	if err != nil {
		return "", err
	}

	if event.Type != domain.EventPaymentPaid {
		return domain.OutcomeIgnored, nil
	}

	outcome, err := u.payments.ApplyPaid(ctx, event.Reference, event.Amount)
	if err != nil {
		return "", err
	}
	if outcome == domain.OutcomeRefundNeeded {
		slog.Warn("pembayaran diterima tetapi pesanan tidak dapat lunas, perlu pengembalian dana", "reference", event.Reference)
	}
	return outcome, nil
}
