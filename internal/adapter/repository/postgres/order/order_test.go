package order_test

import (
	"context"
	"errors"
	"testing"

	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/testdb"
	"toko-online/internal/domain"
)

func TestOrderGetDanList(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	repo := order.NewOrderPostgres(db)
	ctx := context.Background()

	siti := testdb.SeedUser(t, db, "siti@mail.com")
	andi := testdb.SeedUser(t, db, "andi@mail.com")
	kopi := testdb.SeedProduct(t, db, "Kopi", 50000, 100)

	o1, err := repo.Create(ctx, siti, []domain.OrderLine{{ProductID: kopi, Jumlah: 1}})
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	o2, err := repo.Create(ctx, siti, []domain.OrderLine{{ProductID: kopi, Jumlah: 2}})
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	if _, err := repo.Create(ctx, andi, []domain.OrderLine{{ProductID: kopi, Jumlah: 1}}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	t.Run("detail memuat barang", func(t *testing.T) {
		got, err := repo.GetByID(ctx, o1.ID)
		if err != nil {
			t.Fatalf("get gagal: %v", err)
		}
		if got.UserID != siti || len(got.Items) != 1 || got.Items[0].NamaProduk != "Kopi" {
			t.Errorf("detail tidak sesuai: %+v", got)
		}
	})

	t.Run("harga di pesanan tidak berubah saat harga produk diubah", func(t *testing.T) {
		if _, err := db.Exec(`UPDATE products SET harga = 99999 WHERE id = $1`, kopi); err != nil {
			t.Fatalf("ubah harga: %v", err)
		}
		got, err := repo.GetByID(ctx, o1.ID)
		if err != nil {
			t.Fatalf("get gagal: %v", err)
		}
		if got.Items[0].Harga != 50000 {
			t.Errorf("harga barang = %d, ingin 50000 (snapshot)", got.Items[0].Harga)
		}
	})

	t.Run("pesanan tidak ada", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, 9999); !errors.Is(err, domain.ErrOrderNotFound) {
			t.Errorf("error = %v, ingin ErrOrderNotFound", err)
		}
	})

	t.Run("daftar hanya milik sendiri, terbaru dulu, tanpa barang", func(t *testing.T) {
		daftar, err := repo.ListByUser(ctx, siti, 10, 0)
		if err != nil {
			t.Fatalf("list gagal: %v", err)
		}
		if len(daftar) != 2 {
			t.Fatalf("jumlah = %d, ingin 2", len(daftar))
		}
		if daftar[0].ID != o2.ID || daftar[1].ID != o1.ID {
			t.Errorf("urutan = [%d, %d], ingin [%d, %d] (terbaru dulu)", daftar[0].ID, daftar[1].ID, o2.ID, o1.ID)
		}
		if daftar[0].Items != nil {
			t.Error("daftar adalah ringkasan, tidak boleh memuat barang")
		}
	})

	t.Run("paginasi", func(t *testing.T) {
		daftar, err := repo.ListByUser(ctx, siti, 1, 1)
		if err != nil {
			t.Fatalf("list gagal: %v", err)
		}
		if len(daftar) != 1 || daftar[0].ID != o1.ID {
			t.Errorf("halaman ke-2 = %+v, ingin hanya pesanan %d", daftar, o1.ID)
		}
	})
}
