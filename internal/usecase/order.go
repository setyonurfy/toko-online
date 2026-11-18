package usecase

import (
	"context"
	"sort"
	"toko-online/internal/domain"
)

const (
	maxBarisPesanan  = 50
	maxJumlahPerItem = 100
)

type OrderUsecase struct {
	orders domain.OrderRepository
}

func NewOrderUsecase(orders domain.OrderRepository) *OrderUsecase {
	return &OrderUsecase{orders: orders}
}

func (u *OrderUsecase) Place(ctx context.Context, userID int64, lines []domain.OrderLine) (*domain.Order, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidInput
	}
	rapi, err := normalizeLines(lines)
	if err != nil {
		return nil, err
	}
	return u.orders.Create(ctx, userID, rapi)
}

func (u *OrderUsecase) Get(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	if orderID <= 0 {
		return nil, domain.ErrOrderNotFound
	}
	o, err := u.orders.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if o.UserID != userID {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

func (u *OrderUsecase) ListMine(ctx context.Context, userID int64, limit, offset int) ([]domain.Order, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidInput
	}
	limit, offset = normalizePage(limit, offset)
	return u.orders.ListByUser(ctx, userID, limit, offset)
}

func normalizeLines(lines []domain.OrderLine) ([]domain.OrderLine, error) {
	if len(lines) == 0 || len(lines) > maxBarisPesanan {
		return nil, domain.ErrInvalidInput
	}

	jumlah := make(map[int64]int, len(lines))
	for _, l := range lines {
		if l.ProductID <= 0 || l.Jumlah <= 0 || l.Jumlah > maxJumlahPerItem {
			return nil, domain.ErrInvalidInput

		}
		jumlah[l.ProductID] += l.Jumlah
		if jumlah[l.ProductID] > maxJumlahPerItem {
			return nil, domain.ErrInvalidInput
		}
	}
	hasil := make([]domain.OrderLine, 0, len(jumlah))
	for id, n := range jumlah {
		hasil = append(hasil, domain.OrderLine{ProductID: id, Jumlah: n})
	}
	sort.Slice(hasil, func(i, j int) bool {
		return hasil[i].ProductID < hasil[j].ProductID
	})
	return hasil, nil
}
func (u *OrderUsecase) Cancel(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	// Get memeriksa kepemilikan: pesanan orang lain dijawab sama dengan "tidak ada".
	if _, err := u.Get(ctx, userID, orderID); err != nil {
		return nil, err
	}
	return u.orders.Cancel(ctx, orderID)
}
