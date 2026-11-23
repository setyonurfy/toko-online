package product_test

import (
	"context"
	"errors"
	"testing"

	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/product"
	"toko-online/internal/adapter/repository/postgres/testdb"
	"toko-online/internal/domain"
)

func TestProductDelete_SudahPernahDipesan(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	ctx := context.Background()
	produk := product.NewProductPostgres(db)
	pesanan := order.NewOrderPostgres(db)

	user := testdb.SeedUser(t, db, "siti@mail.com")
	dipesan := testdb.SeedProduct(t, db, "Sudah Dipesan", 10000, 5)
	bebas := testdb.SeedProduct(t, db, "Belum Pernah Dipesan", 10000, 5)

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
