package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"toko-online/internal/domain"
)

// ---- fungsi bantu ----

func teks(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var s string
	if err := db.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("query teks %q: %v", query, err)
	}
	return s
}

func seedPesanan(t *testing.T, db *sql.DB, userID, productID int64, jumlah int) *domain.Order {
	t.Helper()
	o, err := NewOrderPostgres(db).Create(context.Background(), userID,
		[]domain.OrderLine{{ProductID: productID, Jumlah: jumlah}})
	if err != nil {
		t.Fatalf("seed pesanan: %v", err)
	}
	return o
}

type fixturBayar struct {
	user    int64
	produk  int64
	pesanan *domain.Order
	tagihan *domain.Payment
}

func siapkanTagihan(t *testing.T, db *sql.DB) fixturBayar {
	t.Helper()
	resetDB(t, db)
	user := seedUser(t, db, "siti@mail.com")
	produk := seedProduct(t, db, "Kopi", 50000, 10)
	o := seedPesanan(t, db, user, produk, 3)

	p, err := NewPaymentPostgres(db).CreatePending(context.Background(), o.ID, "SIM-uji-1")
	if err != nil {
		t.Fatalf("setup tagihan: %v", err)
	}
	return fixturBayar{user: user, produk: produk, pesanan: o, tagihan: p}
}

func statusPesanan(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	return teks(t, db, `SELECT status FROM orders WHERE id = $1`, id)
}

func statusPembayaran(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	return teks(t, db, `SELECT status FROM payments WHERE id = $1`, id)
}

func stokProduk(t *testing.T, db *sql.DB, id int64) int64 {
	t.Helper()
	return hitung(t, db, `SELECT stok FROM products WHERE id = $1`, id)
}

// ---- CreatePending dan GetPendingByOrder ----

func TestPaymentCreatePending_JumlahDariDatabase(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)

	if f.tagihan.Amount != 150000 {
		t.Errorf("jumlah = %d, ingin 150000 (total pesanan di database)", f.tagihan.Amount)
	}
	if f.tagihan.Status != domain.PaymentPending || f.tagihan.OrderID != f.pesanan.ID || f.tagihan.Reference != "SIM-uji-1" {
		t.Errorf("tagihan tidak sesuai: %+v", f.tagihan)
	}
	if f.tagihan.PaidAt != nil {
		t.Error("PaidAt harus kosong untuk tagihan pending")
	}
}

func TestPaymentCreatePending_Idempoten(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	repo := NewPaymentPostgres(db)
	ctx := context.Background()

	kedua, err := repo.CreatePending(ctx, f.pesanan.ID, "SIM-uji-2")
	if err != nil {
		t.Fatalf("create kedua gagal: %v", err)
	}

	if kedua.ID != f.tagihan.ID || kedua.Reference != "SIM-uji-1" {
		t.Errorf("harus mengembalikan tagihan pertama, dapat %+v", kedua)
	}
	if n := hitung(t, db, `SELECT count(*) FROM payments WHERE order_id = $1`, f.pesanan.ID); n != 1 {
		t.Errorf("jumlah tagihan = %d, ingin 1", n)
	}

	ada, err := repo.GetPendingByOrder(ctx, f.pesanan.ID)
	if err != nil || ada.ID != f.tagihan.ID {
		t.Errorf("GetPendingByOrder: tagihan=%v error=%v", ada, err)
	}
}

func TestPaymentCreatePending_TidakBisa(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	repo := NewPaymentPostgres(db)
	ctx := context.Background()

	t.Run("pesanan tidak ada", func(t *testing.T) {
		_, err := repo.CreatePending(ctx, 999999, "SIM-x")
		if !errors.Is(err, domain.ErrOrderNotFound) {
			t.Errorf("error = %v, ingin ErrOrderNotFound", err)
		}
	})

	t.Run("pesanan sudah dibatalkan", func(t *testing.T) {
		dibatalkan := seedPesanan(t, db, f.user, f.produk, 1)
		if _, err := NewOrderPostgres(db).Cancel(ctx, dibatalkan.ID); err != nil {
			t.Fatalf("setup gagal: %v", err)
		}

		_, err := repo.CreatePending(ctx, dibatalkan.ID, "SIM-y")

		if !errors.Is(err, domain.ErrOrderNotPayable) {
			t.Errorf("error = %v, ingin ErrOrderNotPayable", err)
		}
	})

	t.Run("pesanan tanpa tagihan pending", func(t *testing.T) {
		lain := seedPesanan(t, db, f.user, f.produk, 1)
		_, err := repo.GetPendingByOrder(ctx, lain.ID)
		if !errors.Is(err, domain.ErrPaymentNotFound) {
			t.Errorf("error = %v, ingin ErrPaymentNotFound", err)
		}
	})
}

func TestPaymentCreatePending_Konkuren(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewPaymentPostgres(db)
	ctx := context.Background()

	user := seedUser(t, db, "siti@mail.com")
	produk := seedProduct(t, db, "Kopi", 50000, 10)
	o := seedPesanan(t, db, user, produk, 1)

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

	if n := hitung(t, db, `SELECT count(*) FROM payments WHERE order_id = $1`, o.ID); n != 1 {
		t.Errorf("jumlah tagihan = %d, ingin tepat 1", n)
	}
	for i, id := range idTagihan {
		if id == 0 || id != idTagihan[0] {
			t.Errorf("pemanggil %d mendapat tagihan %d, ingin semuanya sama dengan %d", i, id, idTagihan[0])
		}
	}
}

// ---- ApplyPaid ----

func TestPaymentApplyPaid_Sukses(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	repo := NewPaymentPostgres(db)

	hasil, err := repo.ApplyPaid(context.Background(), f.tagihan.Reference, 150000)

	if err != nil {
		t.Fatalf("apply gagal: %v", err)
	}
	if hasil != domain.OutcomePaid {
		t.Errorf("hasil = %q, ingin %q", hasil, domain.OutcomePaid)
	}
	if s := statusPesanan(t, db, f.pesanan.ID); s != "paid" {
		t.Errorf("status pesanan = %q, ingin paid", s)
	}
	if s := statusPembayaran(t, db, f.tagihan.ID); s != "paid" {
		t.Errorf("status pembayaran = %q, ingin paid", s)
	}
	if n := hitung(t, db, `SELECT count(*) FROM payments WHERE id = $1 AND paid_at IS NOT NULL`, f.tagihan.ID); n != 1 {
		t.Error("paid_at harus terisi")
	}
	if stok := stokProduk(t, db, f.produk); stok != 7 {
		t.Errorf("stok = %d, ingin 7 (pesanan lunas tidak mengubah stok)", stok)
	}
}

func TestPaymentApplyPaid_Idempoten(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	repo := NewPaymentPostgres(db)
	ctx := context.Background()

	pertama, err := repo.ApplyPaid(ctx, f.tagihan.Reference, 150000)
	if err != nil {
		t.Fatalf("apply pertama gagal: %v", err)
	}
	kedua, err := repo.ApplyPaid(ctx, f.tagihan.Reference, 150000)
	if err != nil {
		t.Fatalf("apply kedua gagal: %v", err)
	}

	if pertama != domain.OutcomePaid || kedua != domain.OutcomeAlreadyPaid {
		t.Errorf("hasil = (%q, %q), ingin (%q, %q)", pertama, kedua, domain.OutcomePaid, domain.OutcomeAlreadyPaid)
	}
	if s := statusPesanan(t, db, f.pesanan.ID); s != "paid" {
		t.Errorf("status pesanan = %q, ingin paid", s)
	}
}

func TestPaymentApplyPaid_JumlahTidakSesuai(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	repo := NewPaymentPostgres(db)

	_, err := repo.ApplyPaid(context.Background(), f.tagihan.Reference, 149999)

	if !errors.Is(err, domain.ErrAmountMismatch) {
		t.Fatalf("error = %v, ingin ErrAmountMismatch", err)
	}
	if s := statusPesanan(t, db, f.pesanan.ID); s != "pending" {
		t.Errorf("status pesanan = %q, ingin tetap pending", s)
	}
	if s := statusPembayaran(t, db, f.tagihan.ID); s != "pending" {
		t.Errorf("status pembayaran = %q, ingin tetap pending", s)
	}
}

func TestPaymentApplyPaid_ReferensiTidakAda(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewPaymentPostgres(db)

	_, err := repo.ApplyPaid(context.Background(), "SIM-tidak-ada", 1000)

	if !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("error = %v, ingin ErrPaymentNotFound", err)
	}
}

func TestPaymentApplyPaid_Terlambat(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	repo := NewPaymentPostgres(db)
	ctx := context.Background()

	if _, err := NewOrderPostgres(db).Cancel(ctx, f.pesanan.ID); err != nil {
		t.Fatalf("setup: batalkan pesanan gagal: %v", err)
	}

	hasil, err := repo.ApplyPaid(ctx, f.tagihan.Reference, 150000)

	if err != nil {
		t.Fatalf("apply gagal: %v", err)
	}
	if hasil != domain.OutcomeRefundNeeded {
		t.Errorf("hasil = %q, ingin %q", hasil, domain.OutcomeRefundNeeded)
	}
	if s := statusPesanan(t, db, f.pesanan.ID); s != "cancelled" {
		t.Errorf("status pesanan = %q, ingin tetap cancelled", s)
	}
	if s := statusPembayaran(t, db, f.tagihan.ID); s != "refund_needed" {
		t.Errorf("status pembayaran = %q, ingin refund_needed", s)
	}
	if stok := stokProduk(t, db, f.produk); stok != 10 {
		t.Errorf("stok = %d, ingin 10 (sudah dikembalikan oleh pembatalan, dan pembayaran tidak mengubahnya)", stok)
	}

	// Webhook ulang untuk kasus ini tetap dijawab sama dan tidak mengubah apa pun.
	ulang, err := repo.ApplyPaid(ctx, f.tagihan.Reference, 150000)
	if err != nil || ulang != domain.OutcomeRefundNeeded {
		t.Errorf("webhook ulang: hasil=%q error=%v, ingin %q", ulang, err, domain.OutcomeRefundNeeded)
	}
}

func TestPaymentApplyPaid_Konkuren_WebhookDuplikat(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	repo := NewPaymentPostgres(db)
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

			hasil, err := repo.ApplyPaid(ctx, f.tagihan.Reference, 150000)
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
	if s := statusPesanan(t, db, f.pesanan.ID); s != "paid" {
		t.Errorf("status pesanan = %q, ingin paid", s)
	}
}

func TestPaymentApplyPaid_Balapan_BayarLawanBatal(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	orders := NewOrderPostgres(db)
	payments := NewPaymentPostgres(db)
	ctx := context.Background()

	const putaran = 30
	const stokAwal = 100

	user := seedUser(t, db, "siti@mail.com")
	produk := seedProduct(t, db, "Barang Balapan", 10000, stokAwal)

	lunas := 0
	for i := 0; i < putaran; i++ {
		o := seedPesanan(t, db, user, produk, 1)
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
			if s := statusPesanan(t, db, o.ID); s != "paid" {
				t.Errorf("putaran %d: status pesanan = %q, ingin paid", i, s)
			}
			if s := statusPembayaran(t, db, p.ID); s != "paid" {
				t.Errorf("putaran %d: status pembayaran = %q, ingin paid", i, s)
			}
		case domain.OutcomeRefundNeeded:
			if errBatal != nil {
				t.Errorf("putaran %d: pembatalan menang, seharusnya berhasil, dapat %v", i, errBatal)
			}
			if s := statusPesanan(t, db, o.ID); s != "cancelled" {
				t.Errorf("putaran %d: status pesanan = %q, ingin cancelled", i, s)
			}
			if s := statusPembayaran(t, db, p.ID); s != "refund_needed" {
				t.Errorf("putaran %d: status pembayaran = %q, ingin refund_needed", i, s)
			}
		default:
			t.Fatalf("putaran %d: hasil tak terduga %q", i, hasilBayar)
		}
	}

	if stok := stokProduk(t, db, produk); stok != int64(stokAwal-lunas) {
		t.Errorf("stok akhir = %d, ingin %d (stok awal %d dikurangi %d pesanan lunas)", stok, stokAwal-lunas, stokAwal, lunas)
	}
	t.Logf("dari %d putaran: %d pembayaran menang, %d pembatalan menang", putaran, lunas, putaran-lunas)
}
