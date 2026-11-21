package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"toko-online/internal/domain"
)

func umurkanPesanan(t *testing.T, db *sql.DB, id int64, umur time.Duration) {
	t.Helper()
	_, err := db.Exec(
		`UPDATE orders SET created_at = NOW() - ($1::bigint * interval '1 second') WHERE id = $2`,
		int64(umur.Seconds()), id,
	)
	if err != nil {
		t.Fatalf("umurkan pesanan %d: %v", id, err)
	}
}

func batasKedaluwarsa() time.Time {
	return time.Now().Add(-time.Hour)
}

func TestOrderExpirePending_MengembalikanStok(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	user := seedUser(t, db, "siti@mail.com")
	a := seedProduct(t, db, "Barang A", 10000, 10)
	b := seedProduct(t, db, "Barang B", 20000, 5)

	lama, err := repo.Create(ctx, user, []domain.OrderLine{
		{ProductID: a, Jumlah: 3},
		{ProductID: b, Jumlah: 2},
	})
	if err != nil {
		t.Fatalf("setup pesanan lama gagal: %v", err)
	}
	baru := seedPesanan(t, db, user, a, 1)
	umurkanPesanan(t, db, lama.ID, 2*time.Hour)
	umurkanPesanan(t, db, baru.ID, 10*time.Minute)

	n, err := repo.ExpirePending(ctx, batasKedaluwarsa(), 10)
	if err != nil {
		t.Fatalf("ExpirePending gagal: %v", err)
	}
	if n != 1 {
		t.Errorf("jumlah kedaluwarsa = %d, ingin 1", n)
	}
	if s := statusPesanan(t, db, lama.ID); s != string(domain.OrderExpired) {
		t.Errorf("status pesanan lama = %q, ingin %q", s, domain.OrderExpired)
	}
	if s := statusPesanan(t, db, baru.ID); s != string(domain.OrderPending) {
		t.Errorf("status pesanan baru = %q, ingin %q", s, domain.OrderPending)
	}
	if stok := stokProduk(t, db, a); stok != 9 {
		t.Errorf("stok A = %d, ingin 9", stok)
	}
	if stok := stokProduk(t, db, b); stok != 5 {
		t.Errorf("stok B = %d, ingin 5", stok)
	}

	n, err = repo.ExpirePending(ctx, batasKedaluwarsa(), 10)
	if err != nil || n != 0 {
		t.Errorf("panggilan kedua: n = %d, err = %v, ingin 0 tanpa error", n, err)
	}
	if stok := stokProduk(t, db, a); stok != 9 {
		t.Errorf("stok A setelah panggilan kedua = %d, ingin 9 (tidak boleh dikembalikan dua kali)", stok)
	}
}

func TestOrderExpirePending_HanyaPesananPending(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	user := seedUser(t, db, "siti@mail.com")
	a := seedProduct(t, db, "Barang A", 10000, 10)

	lunas := seedPesanan(t, db, user, a, 1)
	batal := seedPesanan(t, db, user, a, 1)
	if _, err := repo.Cancel(ctx, batal.ID); err != nil {
		t.Fatalf("setup pembatalan gagal: %v", err)
	}
	if _, err := db.Exec(`UPDATE orders SET status = $1 WHERE id = $2`, string(domain.OrderPaid), lunas.ID); err != nil {
		t.Fatalf("setup status lunas gagal: %v", err)
	}
	umurkanPesanan(t, db, lunas.ID, 2*time.Hour)
	umurkanPesanan(t, db, batal.ID, 2*time.Hour)

	n, err := repo.ExpirePending(ctx, batasKedaluwarsa(), 10)
	if err != nil {
		t.Fatalf("ExpirePending gagal: %v", err)
	}
	if n != 0 {
		t.Errorf("jumlah kedaluwarsa = %d, ingin 0", n)
	}
	if s := statusPesanan(t, db, lunas.ID); s != string(domain.OrderPaid) {
		t.Errorf("status pesanan lunas = %q, ingin %q", s, domain.OrderPaid)
	}
	if s := statusPesanan(t, db, batal.ID); s != string(domain.OrderCancelled) {
		t.Errorf("status pesanan batal = %q, ingin %q", s, domain.OrderCancelled)
	}
	if stok := stokProduk(t, db, a); stok != 9 {
		t.Errorf("stok A = %d, ingin 9", stok)
	}
}

func TestOrderExpirePending_DibatasiUkuranBatch(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	user := seedUser(t, db, "siti@mail.com")
	a := seedProduct(t, db, "Barang A", 10000, 10)
	for i := 0; i < 5; i++ {
		o := seedPesanan(t, db, user, a, 1)
		umurkanPesanan(t, db, o.ID, 2*time.Hour)
	}

	ingin := []int{2, 2, 1, 0}
	for i, want := range ingin {
		n, err := repo.ExpirePending(ctx, batasKedaluwarsa(), 2)
		if err != nil {
			t.Fatalf("panggilan ke-%d gagal: %v", i+1, err)
		}
		if n != want {
			t.Errorf("panggilan ke-%d: n = %d, ingin %d", i+1, n, want)
		}
	}
	if stok := stokProduk(t, db, a); stok != 10 {
		t.Errorf("stok A = %d, ingin 10", stok)
	}
	if n := hitung(t, db, `SELECT count(*) FROM orders WHERE status = $1`, string(domain.OrderExpired)); n != 5 {
		t.Errorf("jumlah pesanan expired = %d, ingin 5", n)
	}
}

func TestOrderExpirePending_BatchTidakValid(t *testing.T) {
	db := openTestDB(t)

	_, err := NewOrderPostgres(db).ExpirePending(context.Background(), batasKedaluwarsa(), 0)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("error = %v, ingin %v", err, domain.ErrInvalidInput)
	}
}

func TestOrderExpirePending_WebhookTerlambat(t *testing.T) {
	db := openTestDB(t)
	f := siapkanTagihan(t, db)
	ctx := context.Background()
	umurkanPesanan(t, db, f.pesanan.ID, 2*time.Hour)

	n, err := NewOrderPostgres(db).ExpirePending(ctx, batasKedaluwarsa(), 10)
	if err != nil || n != 1 {
		t.Fatalf("ExpirePending: n = %d, err = %v, ingin 1 tanpa error", n, err)
	}
	if stok := stokProduk(t, db, f.produk); stok != 10 {
		t.Fatalf("stok setelah kedaluwarsa = %d, ingin 10", stok)
	}

	hasil, err := NewPaymentPostgres(db).ApplyPaid(ctx, f.tagihan.Reference, f.tagihan.Amount)
	if err != nil {
		t.Fatalf("ApplyPaid gagal: %v", err)
	}
	if hasil != domain.OutcomeRefundNeeded {
		t.Errorf("hasil = %q, ingin %q", hasil, domain.OutcomeRefundNeeded)
	}
	if s := statusPembayaran(t, db, f.tagihan.ID); s != string(domain.PaymentRefundNeeded) {
		t.Errorf("status pembayaran = %q, ingin %q", s, domain.PaymentRefundNeeded)
	}
	if s := statusPesanan(t, db, f.pesanan.ID); s != string(domain.OrderExpired) {
		t.Errorf("status pesanan = %q, ingin %q", s, domain.OrderExpired)
	}
	if stok := stokProduk(t, db, f.produk); stok != 10 {
		t.Errorf("stok akhir = %d, ingin 10", stok)
	}
}

func TestOrderExpirePending_Konkuren_LawanCancel(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	repo := NewOrderPostgres(db)
	ctx := context.Background()

	const jumlahPesanan = 20
	user := seedUser(t, db, "siti@mail.com")
	produk := seedProduct(t, db, "Barang A", 10000, jumlahPesanan)
	ids := make([]int64, 0, jumlahPesanan)
	for i := 0; i < jumlahPesanan; i++ {
		o := seedPesanan(t, db, user, produk, 1)
		umurkanPesanan(t, db, o.ID, 2*time.Hour)
		ids = append(ids, o.ID)
	}
	if stok := stokProduk(t, db, produk); stok != 0 {
		t.Fatalf("stok setelah memesan = %d, ingin 0", stok)
	}

	var wg sync.WaitGroup
	wg.Add(jumlahPesanan + 1)
	go func() {
		defer wg.Done()
		if _, err := repo.ExpirePending(ctx, batasKedaluwarsa(), jumlahPesanan); err != nil {
			t.Errorf("ExpirePending gagal: %v", err)
		}
	}()
	for _, id := range ids {
		go func(id int64) {
			defer wg.Done()
			if _, err := repo.Cancel(ctx, id); err != nil && !errors.Is(err, domain.ErrOrderNotCancellable) {
				t.Errorf("cancel pesanan %d gagal: %v", id, err)
			}
		}(id)
	}
	wg.Wait()

	if stok := stokProduk(t, db, produk); stok != jumlahPesanan {
		t.Errorf("stok akhir = %d, ingin %d (stok tiap pesanan kembali tepat sekali)", stok, jumlahPesanan)
	}
	selesai := hitung(t, db, `SELECT count(*) FROM orders WHERE status IN ($1, $2)`,
		string(domain.OrderCancelled), string(domain.OrderExpired))
	if selesai != jumlahPesanan {
		t.Errorf("pesanan cancelled atau expired = %d, ingin %d", selesai, jumlahPesanan)
	}
	if n := hitung(t, db, `SELECT count(*) FROM orders WHERE status = $1`, string(domain.OrderPending)); n != 0 {
		t.Errorf("pesanan masih pending = %d, ingin 0", n)
	}
}
