package repository

import (
	"context"
	"errors"
	"testing"

	"toko-online/internal/domain"
)

func TestProductDelete_SudahPernahDipesan(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	ctx := context.Background()
	produk := NewProductPostgres(db)
	pesanan := NewOrderPostgres(db)

	user := seedUser(t, db, "siti@mail.com")
	dipesan := seedProduct(t, db, "Sudah Dipesan", 10000, 5)
	bebas := seedProduct(t, db, "Belum Pernah Dipesan", 10000, 5)

	if _, err := pesanan.Create(ctx, user, []domain.OrderLine{{ProductID: dipesan, Jumlah: 1}}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	if err := produk.Delete(ctx, dipesan); !errors.Is(err, domain.ErrProductInUse) {
		t.Errorf("hapus produk yang sudah dipesan: error = %v, ingin ErrProductInUse", err)
	}
	if err := produk.Delete(ctx, bebas); err != nil {
		t.Errorf("hapus produk yang belum pernah dipesan: error = %v, ingin nil", err)
	}
}
