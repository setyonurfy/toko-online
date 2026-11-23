package order

import (
	"context"
	"slices"
	"time"

	"toko-online/internal/domain"
)

type mockOrderRepo struct {
	orders map[int64]*domain.Order
	nextID int64

	createErr    error
	createCalled bool
	gotUserID    int64
	gotLines     []domain.OrderLine

	cancelErr    error
	cancelCalled bool
	gotCancelID  int64

	expireResults []int
	expireErr     error
	expireCalls   int
	gotCutoff     time.Time
	gotBatch      int

	lastLimit, lastOffset int
}

var _ domain.OrderRepository = (*mockOrderRepo)(nil)

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{orders: make(map[int64]*domain.Order)}
}

func (m *mockOrderRepo) Create(ctx context.Context, userID int64, lines []domain.OrderLine) (*domain.Order, error) {
	m.createCalled = true
	m.gotUserID = userID
	m.gotLines = slices.Clone(lines)
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.nextID++
	o := &domain.Order{ID: m.nextID, UserID: userID, Status: domain.OrderPending, Total: 1000}
	m.orders[o.ID] = o
	salinan := *o
	return &salinan, nil
}

func (m *mockOrderRepo) GetByID(ctx context.Context, id int64) (*domain.Order, error) {
	o, ada := m.orders[id]
	if !ada {
		return nil, domain.ErrOrderNotFound
	}
	salinan := *o
	return &salinan, nil
}

func (m *mockOrderRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Order, error) {
	m.lastLimit, m.lastOffset = limit, offset
	hasil := make([]domain.Order, 0)
	for _, o := range m.orders {
		if o.UserID == userID {
			hasil = append(hasil, *o)
		}
	}
	return hasil, nil
}

func (m *mockOrderRepo) Cancel(ctx context.Context, id int64) (*domain.Order, error) {
	m.cancelCalled = true
	m.gotCancelID = id
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	o, ada := m.orders[id]
	if !ada {
		return nil, domain.ErrOrderNotFound
	}
	o.Status = domain.OrderCancelled
	salinan := *o
	return &salinan, nil
}

func (m *mockOrderRepo) ExpirePending(ctx context.Context, cutoff time.Time, batch int) (int, error) {
	m.expireCalls++
	m.gotCutoff = cutoff
	m.gotBatch = batch
	if m.expireErr != nil {
		return 0, m.expireErr
	}
	if len(m.expireResults) == 0 {
		return 0, nil
	}
	n := m.expireResults[0]
	m.expireResults = m.expireResults[1:]
	return n, nil
}

func newTestOrderUsecase() (*OrderUsecase, *mockOrderRepo) {
	repo := newMockOrderRepo()
	return NewOrderUsecase(repo), repo
}
