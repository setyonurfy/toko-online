package payment

import (
	"context"

	"toko-online/internal/domain"
)

type fakeOrderReader struct {
	order *domain.Order
	err   error
}

func (f *fakeOrderReader) Get(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	return f.order, f.err
}

type mockPaymentRepo struct {
	pending *domain.Payment // tagihan pending yang sudah ada (nil = belum ada)
	getErr  error

	createErr   error
	createCalls int
	gotOrderID  int64
	gotRef      string

	applyOutcome domain.PaymentOutcome
	applyErr     error
	applyCalls   int
	gotApplyRef  string
	gotApplyAmt  int64
}

var _ domain.PaymentRepository = (*mockPaymentRepo)(nil)

func (m *mockPaymentRepo) GetPendingByOrder(ctx context.Context, orderID int64) (*domain.Payment, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.pending == nil {
		return nil, domain.ErrPaymentNotFound
	}
	return m.pending, nil
}

func (m *mockPaymentRepo) CreatePending(ctx context.Context, orderID int64, reference string) (*domain.Payment, error) {
	m.createCalls++
	m.gotOrderID = orderID
	m.gotRef = reference
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &domain.Payment{ID: 1, OrderID: orderID, Reference: reference, Amount: 1000, Status: domain.PaymentPending}, nil
}

func (m *mockPaymentRepo) ApplyPaid(ctx context.Context, reference string, amount int64) (domain.PaymentOutcome, error) {
	m.applyCalls++
	m.gotApplyRef = reference
	m.gotApplyAmt = amount
	return m.applyOutcome, m.applyErr
}

type fakeGateway struct {
	chargeErr   error
	chargeCalls int
	gotAmount   int64

	event    domain.GatewayEvent
	parseErr error
}

func (g *fakeGateway) CreateCharge(ctx context.Context, orderID, amount int64) (domain.Charge, error) {
	g.chargeCalls++
	g.gotAmount = amount
	if g.chargeErr != nil {
		return domain.Charge{}, g.chargeErr
	}
	return domain.Charge{Reference: "SIM-uji"}, nil
}

func (g *fakeGateway) ParseWebhook(body []byte, timestamp, signature string) (domain.GatewayEvent, error) {
	if g.parseErr != nil {
		return domain.GatewayEvent{}, g.parseErr
	}
	return g.event, nil
}

func newTestPaymentUsecase(order *domain.Order, orderErr error) (*PaymentUsecase, *mockPaymentRepo, *fakeGateway) {
	repo := &mockPaymentRepo{}
	gw := &fakeGateway{}
	return NewPaymentUsecase(&fakeOrderReader{order: order, err: orderErr}, repo, gw), repo, gw
}
