package handler

import (
	"context"
	"slices"

	"toko-online/internal/domain"
)

type fakeAuth struct {
	user *domain.User
	err  error
}

func (f *fakeAuth) Register(ctx context.Context, nama, email, password string) (*domain.User, error) {
	return f.user, f.err
}
func (f *fakeAuth) Login(ctx context.Context, email, password string) (*domain.LoginResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.LoginResult{User: f.user, Token: "token-palsu"}, nil
}
func (f *fakeAuth) Profile(ctx context.Context, id int64) (*domain.User, error) {
	return f.user, f.err
}

type fakeProducts struct {
	product *domain.Product
	list    []domain.Product
	err     error

	dipanggil           bool
	gotID               int64
	gotLimit, gotOffset int
	gotStok             int
}

func (f *fakeProducts) Create(ctx context.Context, nama, deskripsi string, harga int64, stok int) (*domain.Product, error) {
	f.dipanggil = true
	f.gotStok = stok
	return f.product, f.err
}

func (f *fakeProducts) Get(ctx context.Context, id int64) (*domain.Product, error) {
	f.dipanggil = true
	f.gotID = id
	return f.product, f.err
}

func (f *fakeProducts) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	f.dipanggil = true
	f.gotLimit, f.gotOffset = limit, offset
	return f.list, f.err
}

func (f *fakeProducts) Update(ctx context.Context, id int64, nama, deskripsi string, harga int64, stok int) (*domain.Product, error) {
	f.dipanggil = true
	f.gotID = id
	f.gotStok = stok
	return f.product, f.err
}

func (f *fakeProducts) Delete(ctx context.Context, id int64) error {
	f.dipanggil = true
	f.gotID = id
	return f.err
}

type fakeOrders struct {
	order *domain.Order
	list  []domain.Order
	err   error

	dipanggil           bool
	gotUserID           int64
	gotOrderID          int64
	gotLines            []domain.OrderLine
	gotLimit, gotOffset int
}

func (f *fakeOrders) Place(ctx context.Context, userID int64, lines []domain.OrderLine) (*domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotLines = slices.Clone(lines)
	return f.order, f.err
}

func (f *fakeOrders) Get(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotOrderID = orderID
	return f.order, f.err
}

func (f *fakeOrders) ListMine(ctx context.Context, userID int64, limit, offset int) ([]domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotLimit, f.gotOffset = limit, offset
	return f.list, f.err
}

func (f *fakeOrders) Cancel(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotOrderID = orderID
	return f.order, f.err
}

type fakePayments struct {
	payment *domain.Payment
	outcome domain.PaymentOutcome
	err     error

	dipanggil    bool
	gotUserID    int64
	gotOrderID   int64
	gotBody      []byte
	gotTimestamp string
	gotSignature string
}

func (f *fakePayments) Pay(ctx context.Context, userID, orderID int64) (*domain.Payment, error) {
	f.dipanggil = true
	f.gotUserID = userID
	f.gotOrderID = orderID
	return f.payment, f.err
}

func (f *fakePayments) HandleWebhook(ctx context.Context, body []byte, timestamp, signature string) (domain.PaymentOutcome, error) {
	f.dipanggil = true
	f.gotBody = append([]byte(nil), body...)
	f.gotTimestamp = timestamp
	f.gotSignature = signature
	return f.outcome, f.err
}
