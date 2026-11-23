package order

import (
	"context"

	"toko-online/internal/domain"
)

func (u *OrderUsecase) Cancel(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	// Get memeriksa kepemilikan: pesanan orang lain dijawab sama dengan "tidak ada".
	if _, err := u.Get(ctx, userID, orderID); err != nil {
		return nil, err
	}
	return u.orders.Cancel(ctx, orderID)
}
