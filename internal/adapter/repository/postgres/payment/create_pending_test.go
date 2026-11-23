package payment_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/payment"
	"toko-online/internal/adapter/repository/postgres/testdb"
	"toko-online/internal/domain"
)

func TestPaymentCreatePending_JumlahDariDatabase(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)

	if f.Tagihan.Amount != 150000 {
		t.Errorf("jumlah = %d, ingin 150000 (total pesanan di database)", f.Tagihan.Amount)
	}
	if f.Tagihan.Status != domain.PaymentPending || f.Tagihan.OrderID != f.Pesanan.ID || f.Tagihan.Reference != "SIM-uji-1" {
		t.Errorf("tagihan tidak sesuai: %+v", f.Tagihan)
	}
	if f.Tagihan.PaidAt != nil {
		t.Error("PaidAt harus kosong untuk tagihan pending")
	}
}

func TestPaymentCreatePending_Idempoten(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)
	repo := payment.NewPaymentPostgres(db)
	ctx := context.Background()

	kedua, err := repo.CreatePending(ctx, f.Pesanan.ID, "SIM-uji-2")
	if err != nil {
		t.Fatalf("create kedua gagal: %v", err)
	}

	if kedua.ID != f.Tagihan.ID || kedua.Reference != "SIM-uji-1" {
		t.Errorf("harus mengembalikan tagihan pertama, dapat %+v", kedua)
	}
	if n := testdb.Count(t, db, `SELECT count(*) FROM payments WHERE order_id = $1`, f.Pesanan.ID); n != 1 {
		t.Errorf("jumlah tagihan = %d, ingin 1", n)
	}

	ada, err := repo.GetPendingByOrder(ctx, f.Pesanan.ID)
	if err != nil || ada.ID != f.Tagihan.ID {
		t.Errorf("GetPendingByOrder: tagihan=%v error=%v", ada, err)
	}
}

func TestPaymentCreatePending_TidakBisa(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)
	repo := payment.NewPaymentPostgres(db)
	ctx := context.Background()

	t.Run("pesanan tidak ada", func(t *testing.T) {
		_, err := repo.CreatePending(ctx, 999999, "SIM-x")
		if !errors.Is(err, domain.ErrOrderNotFound) {
			t.Errorf("error = %v, ingin ErrOrderNotFound", err)
		}
	})

	t.Run("pesanan sudah dibatalkan", func(t *testing.T) {
		dibatalkan := testdb.SeedPesanan(t, db, f.User, f.Produk, 1)
		if _, err := order.NewOrderPostgres(db).Cancel(ctx, dibatalkan.ID); err != nil {
			t.Fatalf("setup gagal: %v", err)
		}

		_, err := repo.CreatePending(ctx, dibatalkan.ID, "SIM-y")

		if !errors.Is(err, domain.ErrOrderNotPayable) {
			t.Errorf("error = %v, ingin ErrOrderNotPayable", err)
		}
	})

	t.Run("pesanan tanpa tagihan pending", func(t *testing.T) {
		lain := testdb.SeedPesanan(t, db, f.User, f.Produk, 1)
		_, err := repo.GetPendingByOrder(ctx, lain.ID)
		if !errors.Is(err, domain.ErrPaymentNotFound) {
			t.Errorf("error = %v, ingin ErrPaymentNotFound", err)
		}
	})
}

func TestPaymentCreatePending_Konkuren(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	repo := payment.NewPaymentPostgres(db)
	ctx := context.Background()

	user := testdb.SeedUser(t, db, "siti@mail.com")
	produk := testdb.SeedProduct(t, db, "Kopi", 50000, 10)
	o := testdb.SeedPesanan(t, db, user, produk, 1)

	const pemanggil = 10
	var (
		wg        sync.WaitGroup
		mulai     = make(chan struct{})
		idTagihan = make([]int64, pemanggil)
	)

	for i := 0; i < pemanggil; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-mulai

			p, err := repo.CreatePending(ctx, o.ID, fmt.Sprintf("SIM-konkuren-%d", i))
			if err != nil {
				t.Errorf("pemanggil %d: %v", i, err)
				return
			}
			idTagihan[i] = p.ID
		}(i)
	}
	close(mulai)
	wg.Wait()

	if n := testdb.Count(t, db, `SELECT count(*) FROM payments WHERE order_id = $1`, o.ID); n != 1 {
		t.Errorf("jumlah tagihan = %d, ingin tepat 1", n)
	}
	for i, id := range idTagihan {
		if id == 0 || id != idTagihan[0] {
			t.Errorf("pemanggil %d mendapat tagihan %d, ingin semuanya sama dengan %d", i, id, idTagihan[0])
		}
	}
}
