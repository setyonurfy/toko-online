package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"toko-online/internal/domain"
)

func TestOrderCreate_Sukses(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	user := seedUser(t, db, "siti@mail.com")
	kopi := seedProduct(t, db, "Kopi", 50000, 10)

	o, err := repo.Create(ctx, user, []domain.OrderLine{{ProductID: kopi, Jumlah: 3}})
	if err != nil {
		t.Fatalf("create gagal: %v", err)
	}

	if o.ID == 0 || o.Status != domain.OrderPending || o.UserID != user {
		t.Errorf("pesanan tidak sesuai: %+v", o)
	}
	if o.Total != 150000 {
		t.Errorf("total = %d, ingin 150000", o.Total)
	}
	if len(o.Items) != 1 || o.Items[0].NamaProduk != "Kopi" || o.Items[0].Harga != 50000 || o.Items[0].Subtotal != 150000 {
		t.Errorf("barang pesanan tidak sesuai: %+v", o.Items)
	}
	if stok := hitung(t, db, `SELECT stok FROM products WHERE id = $1`, kopi); stok != 7 {
		t.Errorf("stok = %d, ingin 7", stok)
	}
}

func TestOrderCreate_StokKurang_SemuaDibatalkan(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	user := seedUser(t, db, "siti@mail.com")
	a := seedProduct(t, db, "Barang A", 10000, 5) // id lebih kecil, diproses lebih dulu
	b := seedProduct(t, db, "Barang B", 20000, 1) // stok tidak cukup untuk 2

	_, err := repo.Create(ctx, user, []domain.OrderLine{
		{ProductID: a, Jumlah: 2},
		{ProductID: b, Jumlah: 2},
	})

	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("error = %v, ingin ErrInsufficientStock", err)
	}
	// Inti atomisitas: pengurangan stok A (yang sudah berhasil) harus ikut dibatalkan.
	if stok := hitung(t, db, `SELECT stok FROM products WHERE id = $1`, a); stok != 5 {
		t.Errorf("stok A = %d, ingin 5 (rollback gagal)", stok)
	}
	if stok := hitung(t, db, `SELECT stok FROM products WHERE id = $1`, b); stok != 1 {
		t.Errorf("stok B = %d, ingin 1", stok)
	}
	if n := hitung(t, db, `SELECT count(*) FROM orders`); n != 0 {
		t.Errorf("jumlah pesanan = %d, ingin 0", n)
	}
	if n := hitung(t, db, `SELECT count(*) FROM order_items`); n != 0 {
		t.Errorf("jumlah barang pesanan = %d, ingin 0", n)
	}
}

func TestOrderCreate_ProdukTidakAda(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)

	user := seedUser(t, db, "siti@mail.com")

	_, err := repo.Create(context.Background(), user, []domain.OrderLine{{ProductID: 999, Jumlah: 1}})

	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("error = %v, ingin ErrProductNotFound", err)
	}
	if n := hitung(t, db, `SELECT count(*) FROM orders`); n != 0 {
		t.Errorf("jumlah pesanan = %d, ingin 0", n)
	}
}

func TestOrderCreate_Konkuren_TidakAdaOverselling(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	const stokAwal = 5
	const pembeli = 20

	user := seedUser(t, db, "pembeli@mail.com")
	langka := seedProduct(t, db, "Barang Langka", 10000, stokAwal)

	var (
		wg        sync.WaitGroup
		mulai     = make(chan struct{})
		berhasil  atomic.Int64
		kehabisan atomic.Int64
	)

	for i := 0; i < pembeli; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-mulai // semua goroutine menunggu aba-aba agar benar-benar berebut bersamaan

			_, err := repo.Create(ctx, user, []domain.OrderLine{{ProductID: langka, Jumlah: 1}})
			switch {
			case err == nil:
				berhasil.Add(1)
			case errors.Is(err, domain.ErrInsufficientStock):
				kehabisan.Add(1)
			default:
				t.Errorf("error tak terduga: %v", err)
			}
		}()
	}
	close(mulai)
	wg.Wait()

	if berhasil.Load() != stokAwal {
		t.Errorf("pesanan berhasil = %d, ingin tepat %d", berhasil.Load(), stokAwal)
	}
	if kehabisan.Load() != pembeli-stokAwal {
		t.Errorf("pesanan ditolak karena stok = %d, ingin %d", kehabisan.Load(), pembeli-stokAwal)
	}
	if stok := hitung(t, db, `SELECT stok FROM products WHERE id = $1`, langka); stok != 0 {
		t.Errorf("stok akhir = %d, ingin 0 (tidak boleh negatif maupun tersisa)", stok)
	}
	if n := hitung(t, db, `SELECT count(*) FROM orders`); n != stokAwal {
		t.Errorf("jumlah pesanan tersimpan = %d, ingin %d", n, stokAwal)
	}
	if n := hitung(t, db, `SELECT COALESCE(SUM(jumlah), 0) FROM order_items`); n != stokAwal {
		t.Errorf("total barang terjual = %d, ingin %d", n, stokAwal)
	}
}

func TestOrderGetDanList(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	siti := seedUser(t, db, "siti@mail.com")
	andi := seedUser(t, db, "andi@mail.com")
	kopi := seedProduct(t, db, "Kopi", 50000, 100)

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
