package order_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/testdb"
	"toko-online/internal/domain"
)

func TestOrderCancel_MengembalikanStok(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	repo := order.NewOrderPostgres(db)
	ctx := context.Background()

	user := testdb.SeedUser(t, db, "siti@mail.com")
	a := testdb.SeedProduct(t, db, "Barang A", 10000, 10)
	b := testdb.SeedProduct(t, db, "Barang B", 20000, 5)

	o, err := repo.Create(ctx, user, []domain.OrderLine{
		{ProductID: a, Jumlah: 3},
		{ProductID: b, Jumlah: 2},
	})
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	if stokA, stokB := testdb.Count(t, db, `SELECT stok FROM products WHERE id = $1`, a), testdb.Count(t, db, `SELECT stok FROM products WHERE id = $1`, b); stokA != 7 || stokB != 3 {
		t.Fatalf("stok setelah memesan = (%d, %d), ingin (7, 3)", stokA, stokB)
	}

	dibatalkan, err := repo.Cancel(ctx, o.ID)
	if err != nil {
		t.Fatalf("cancel gagal: %v", err)
	}

	if dibatalkan.Status != domain.OrderCancelled {
		t.Errorf("status hasil = %q, ingin %q", dibatalkan.Status, domain.OrderCancelled)
	}
	if len(dibatalkan.Items) != 2 {
		t.Errorf("jumlah barang pada hasil = %d, ingin 2", len(dibatalkan.Items))
	}
	if stok := testdb.Count(t, db, `SELECT stok FROM products WHERE id = $1`, a); stok != 10 {
		t.Errorf("stok A = %d, ingin 10 (kembali ke awal)", stok)
	}
	if stok := testdb.Count(t, db, `SELECT stok FROM products WHERE id = $1`, b); stok != 5 {
		t.Errorf("stok B = %d, ingin 5 (kembali ke awal)", stok)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM orders WHERE id = $1`, o.ID).Scan(&status); err != nil {
		t.Fatalf("baca status: %v", err)
	}
	if status != "cancelled" {
		t.Errorf("status di database = %q, ingin cancelled", status)
	}
}

func TestOrderCancel_HanyaPesananPending(t *testing.T) {
	db := testdb.Open(t)
	repo := order.NewOrderPostgres(db)
	ctx := context.Background()

	t.Run("sudah dibatalkan: stok tidak boleh dikembalikan dua kali", func(t *testing.T) {
		testdb.Reset(t, db)
		user := testdb.SeedUser(t, db, "siti@mail.com")
		produk := testdb.SeedProduct(t, db, "Barang A", 10000, 10)

		o, err := repo.Create(ctx, user, []domain.OrderLine{{ProductID: produk, Jumlah: 3}})
		if err != nil {
			t.Fatalf("setup gagal: %v", err)
		}
		if _, err := repo.Cancel(ctx, o.ID); err != nil {
			t.Fatalf("pembatalan pertama gagal: %v", err)
		}

		_, err = repo.Cancel(ctx, o.ID)

		if !errors.Is(err, domain.ErrOrderNotCancellable) {
			t.Fatalf("pembatalan kedua: error = %v, ingin ErrOrderNotCancellable", err)
		}
		if stok := testdb.Count(t, db, `SELECT stok FROM products WHERE id = $1`, produk); stok != 10 {
			t.Errorf("stok = %d, ingin 10 (tidak boleh bertambah lagi)", stok)
		}
	})

	t.Run("sudah dibayar: tidak boleh dibatalkan", func(t *testing.T) {
		testdb.Reset(t, db)
		user := testdb.SeedUser(t, db, "siti@mail.com")
		produk := testdb.SeedProduct(t, db, "Barang A", 10000, 10)

		o, err := repo.Create(ctx, user, []domain.OrderLine{{ProductID: produk, Jumlah: 3}})
		if err != nil {
			t.Fatalf("setup gagal: %v", err)
		}
		if _, err := db.Exec(`UPDATE orders SET status = 'paid' WHERE id = $1`, o.ID); err != nil {
			t.Fatalf("ubah status: %v", err)
		}

		_, err = repo.Cancel(ctx, o.ID)

		if !errors.Is(err, domain.ErrOrderNotCancellable) {
			t.Fatalf("error = %v, ingin ErrOrderNotCancellable", err)
		}
		if stok := testdb.Count(t, db, `SELECT stok FROM products WHERE id = $1`, produk); stok != 7 {
			t.Errorf("stok = %d, ingin 7 (pesanan dibayar tidak mengembalikan stok)", stok)
		}
		var status string
		if err := db.QueryRow(`SELECT status FROM orders WHERE id = $1`, o.ID).Scan(&status); err != nil {
			t.Fatalf("baca status: %v", err)
		}
		if status != "paid" {
			t.Errorf("status = %q, ingin tetap paid", status)
		}
	})
}

func TestOrderCancel_PesananTidakAda(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	repo := order.NewOrderPostgres(db)

	_, err := repo.Cancel(context.Background(), 9999)

	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("error = %v, ingin ErrOrderNotFound", err)
	}
}

func TestOrderCancel_Konkuren_StokDikembalikanSekali(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	repo := order.NewOrderPostgres(db)
	ctx := context.Background()

	const stokAwal = 10
	const jumlahDipesan = 3
	const pembatal = 10

	user := testdb.SeedUser(t, db, "siti@mail.com")
	produk := testdb.SeedProduct(t, db, "Barang Uji", 10000, stokAwal)

	o, err := repo.Create(ctx, user, []domain.OrderLine{{ProductID: produk, Jumlah: jumlahDipesan}})
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	var (
		wg       sync.WaitGroup
		mulai    = make(chan struct{})
		berhasil atomic.Int64
		ditolak  atomic.Int64
	)

	for i := 0; i < pembatal; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-mulai // semua pembatal menunggu aba-aba agar benar-benar bersamaan

			_, err := repo.Cancel(ctx, o.ID)
			switch {
			case err == nil:
				berhasil.Add(1)
			case errors.Is(err, domain.ErrOrderNotCancellable):
				ditolak.Add(1)
			default:
				t.Errorf("error tak terduga: %v", err)
			}
		}()
	}
	close(mulai)
	wg.Wait()

	if berhasil.Load() != 1 {
		t.Errorf("pembatalan berhasil = %d, ingin tepat 1", berhasil.Load())
	}
	if ditolak.Load() != pembatal-1 {
		t.Errorf("pembatalan ditolak = %d, ingin %d", ditolak.Load(), pembatal-1)
	}
	if stok := testdb.Count(t, db, `SELECT stok FROM products WHERE id = $1`, produk); stok != stokAwal {
		t.Errorf("stok akhir = %d, ingin %d (stok hanya boleh dikembalikan sekali)", stok, stokAwal)
	}
}
