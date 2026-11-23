package testdb

import (
	"context"
	"database/sql"
	"testing"

	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/payment"
	"toko-online/internal/domain"
)

func teks(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var s string
	if err := db.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("query teks %q: %v", query, err)
	}
	return s
}

func SeedPesanan(t *testing.T, db *sql.DB, userID, productID int64, jumlah int) *domain.Order {
	t.Helper()
	o, err := order.NewOrderPostgres(db).Create(context.Background(), userID,
		[]domain.OrderLine{{ProductID: productID, Jumlah: jumlah}})
	if err != nil {
		t.Fatalf("seed pesanan: %v", err)
	}
	return o
}

type FixturBayar struct {
	User    int64
	Produk  int64
	Pesanan *domain.Order
	Tagihan *domain.Payment
}

func SiapkanTagihan(t *testing.T, db *sql.DB) FixturBayar {
	t.Helper()
	Reset(t, db)
	user := SeedUser(t, db, "siti@mail.com")
	produk := SeedProduct(t, db, "Kopi", 50000, 10)
	o := SeedPesanan(t, db, user, produk, 3)

	p, err := payment.NewPaymentPostgres(db).CreatePending(context.Background(), o.ID, "SIM-uji-1")
	if err != nil {
		t.Fatalf("setup tagihan: %v", err)
	}
	return FixturBayar{User: user, Produk: produk, Pesanan: o, Tagihan: p}
}

func StatusPesanan(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	return teks(t, db, `SELECT status FROM orders WHERE id = $1`, id)
}

func StatusPembayaran(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	return teks(t, db, `SELECT status FROM payments WHERE id = $1`, id)
}

func StokProduk(t *testing.T, db *sql.DB, id int64) int64 {
	t.Helper()
	return Count(t, db, `SELECT stok FROM products WHERE id = $1`, id)
}
