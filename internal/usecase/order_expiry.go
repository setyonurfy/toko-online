package usecase

import (
	"context"
	"time"

	"toko-online/internal/domain"
)

type OrderExpiryUsecase struct {
	orders domain.OrderRepository
	ttl    time.Duration
	batch  int
	now    func() time.Time
}

func NewOrderExpiryUsecase(orders domain.OrderRepository, ttl time.Duration, batch int) *OrderExpiryUsecase {
	return &OrderExpiryUsecase{orders: orders, ttl: ttl, batch: batch, now: time.Now}
}

func (u *OrderExpiryUsecase) ExpireDue(ctx context.Context) (int, error) {
	if u.ttl <= 0 || u.batch <= 0 {
		return 0, domain.ErrInvalidInput
	}

	cutoff := u.now().Add(-u.ttl)
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := u.orders.ExpirePending(ctx, cutoff, u.batch)
		total += n
		if err != nil {
			return total, err
		}
		if n < u.batch {
			return total, nil
		}
	}
}
