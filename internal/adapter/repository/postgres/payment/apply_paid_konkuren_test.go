package payment_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/payment"
	"toko-online/internal/adapter/repository/postgres/testdb"
	"toko-online/internal/domain"
)

func TestPaymentApplyPaid_Konkuren_WebhookDuplikat(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)
	repo := payment.NewPaymentPostgres(db)
	ctx := context.Background()

	const pengiriman = 10
	var (
		wg      sync.WaitGroup
		mulai   = make(chan struct{})
		dibayar atomic.Int64
		sudah   atomic.Int64
	)

	for i := 0; i < pengiriman; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-mulai

			hasil, err := repo.ApplyPaid(ctx, f.Tagihan.Reference, 150000)
			switch {
			case err != nil:
				t.Errorf("error tak terduga: %v", err)
			case hasil == domain.OutcomePaid:
				dibayar.Add(1)
			case hasil == domain.OutcomeAlreadyPaid:
				sudah.Add(1)
			default:
				t.Errorf("hasil tak terduga: %q", hasil)
			}
		}()
	}
	close(mulai)
	wg.Wait()

	if dibayar.Load() != 1 {
		t.Errorf("diproses sebagai lunas = %d kali, ingin tepat 1", dibayar.Load())
	}
	if sudah.Load() != pengiriman-1 {
		t.Errorf("dijawab sudah lunas = %d kali, ingin %d", sudah.Load(), pengiriman-1)
	}
	if s := testdb.StatusPesanan(t, db, f.Pesanan.ID); s != "paid" {
		t.Errorf("status pesanan = %q, ingin paid", s)
	}
}

func TestPaymentApplyPaid_Balapan_BayarLawanBatal(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	orders := order.NewOrderPostgres(db)
	payments := payment.NewPaymentPostgres(db)
	ctx := context.Background()

	const putaran = 30
	const stokAwal = 100

	user := testdb.SeedUser(t, db, "siti@mail.com")
	produk := testdb.SeedProduct(t, db, "Barang Balapan", 10000, stokAwal)

	lunas := 0
	for i := 0; i < putaran; i++ {
		o := testdb.SeedPesanan(t, db, user, produk, 1)
		ref := fmt.Sprintf("SIM-balapan-%d", i)
		p, err := payments.CreatePending(ctx, o.ID, ref)
		if err != nil {
			t.Fatalf("putaran %d: setup tagihan gagal: %v", i, err)
		}

		var (
			wg         sync.WaitGroup
			mulai      = make(chan struct{})
			hasilBayar domain.PaymentOutcome
			errBayar   error
			errBatal   error
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-mulai
			hasilBayar, errBayar = payments.ApplyPaid(ctx, ref, o.Total)
		}()
		go func() {
			defer wg.Done()
			<-mulai
			_, errBatal = orders.Cancel(ctx, o.ID)
		}()
		close(mulai)
		wg.Wait()

		if errBayar != nil {
			t.Fatalf("putaran %d: ApplyPaid gagal: %v", i, errBayar)
		}

		switch hasilBayar {
		case domain.OutcomePaid:
			lunas++
			if !errors.Is(errBatal, domain.ErrOrderNotCancellable) {
				t.Errorf("putaran %d: pembayaran menang, pembatalan seharusnya ErrOrderNotCancellable, dapat %v", i, errBatal)
			}
			if s := testdb.StatusPesanan(t, db, o.ID); s != "paid" {
				t.Errorf("putaran %d: status pesanan = %q, ingin paid", i, s)
			}
			if s := testdb.StatusPembayaran(t, db, p.ID); s != "paid" {
				t.Errorf("putaran %d: status pembayaran = %q, ingin paid", i, s)
			}
		case domain.OutcomeRefundNeeded:
			if errBatal != nil {
				t.Errorf("putaran %d: pembatalan menang, seharusnya berhasil, dapat %v", i, errBatal)
			}
			if s := testdb.StatusPesanan(t, db, o.ID); s != "cancelled" {
				t.Errorf("putaran %d: status pesanan = %q, ingin cancelled", i, s)
			}
			if s := testdb.StatusPembayaran(t, db, p.ID); s != "refund_needed" {
				t.Errorf("putaran %d: status pembayaran = %q, ingin refund_needed", i, s)
			}
		default:
			t.Fatalf("putaran %d: hasil tak terduga %q", i, hasilBayar)
		}
	}

	if stok := testdb.StokProduk(t, db, produk); stok != int64(stokAwal-lunas) {
		t.Errorf("stok akhir = %d, ingin %d (stok awal %d dikurangi %d pesanan lunas)", stok, stokAwal-lunas, stokAwal, lunas)
	}
	t.Logf("dari %d putaran: %d pembayaran menang, %d pembatalan menang", putaran, lunas, putaran-lunas)
}
