package payment_test

import (
	"context"
	"errors"
	"testing"

	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/payment"
	"toko-online/internal/adapter/repository/postgres/testdb"
	"toko-online/internal/domain"
)

func TestPaymentApplyPaid_Sukses(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)
	repo := payment.NewPaymentPostgres(db)

	hasil, err := repo.ApplyPaid(context.Background(), f.Tagihan.Reference, 150000)

	if err != nil {
		t.Fatalf("apply gagal: %v", err)
	}
	if hasil != domain.OutcomePaid {
		t.Errorf("hasil = %q, ingin %q", hasil, domain.OutcomePaid)
	}
	if s := testdb.StatusPesanan(t, db, f.Pesanan.ID); s != "paid" {
		t.Errorf("status pesanan = %q, ingin paid", s)
	}
	if s := testdb.StatusPembayaran(t, db, f.Tagihan.ID); s != "paid" {
		t.Errorf("status pembayaran = %q, ingin paid", s)
	}
	if n := testdb.Count(t, db, `SELECT count(*) FROM payments WHERE id = $1 AND paid_at IS NOT NULL`, f.Tagihan.ID); n != 1 {
		t.Error("paid_at harus terisi")
	}
	if stok := testdb.StokProduk(t, db, f.Produk); stok != 7 {
		t.Errorf("stok = %d, ingin 7 (pesanan lunas tidak mengubah stok)", stok)
	}
}

func TestPaymentApplyPaid_Idempoten(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)
	repo := payment.NewPaymentPostgres(db)
	ctx := context.Background()

	pertama, err := repo.ApplyPaid(ctx, f.Tagihan.Reference, 150000)
	if err != nil {
		t.Fatalf("apply pertama gagal: %v", err)
	}
	kedua, err := repo.ApplyPaid(ctx, f.Tagihan.Reference, 150000)
	if err != nil {
		t.Fatalf("apply kedua gagal: %v", err)
	}

	if pertama != domain.OutcomePaid || kedua != domain.OutcomeAlreadyPaid {
		t.Errorf("hasil = (%q, %q), ingin (%q, %q)", pertama, kedua, domain.OutcomePaid, domain.OutcomeAlreadyPaid)
	}
	if s := testdb.StatusPesanan(t, db, f.Pesanan.ID); s != "paid" {
		t.Errorf("status pesanan = %q, ingin paid", s)
	}
}

func TestPaymentApplyPaid_JumlahTidakSesuai(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)
	repo := payment.NewPaymentPostgres(db)

	_, err := repo.ApplyPaid(context.Background(), f.Tagihan.Reference, 149999)

	if !errors.Is(err, domain.ErrAmountMismatch) {
		t.Fatalf("error = %v, ingin ErrAmountMismatch", err)
	}
	if s := testdb.StatusPesanan(t, db, f.Pesanan.ID); s != "pending" {
		t.Errorf("status pesanan = %q, ingin tetap pending", s)
	}
	if s := testdb.StatusPembayaran(t, db, f.Tagihan.ID); s != "pending" {
		t.Errorf("status pembayaran = %q, ingin tetap pending", s)
	}
}

func TestPaymentApplyPaid_ReferensiTidakAda(t *testing.T) {
	db := testdb.Open(t)
	testdb.Reset(t, db)
	repo := payment.NewPaymentPostgres(db)

	_, err := repo.ApplyPaid(context.Background(), "SIM-tidak-ada", 1000)

	if !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("error = %v, ingin ErrPaymentNotFound", err)
	}
}

func TestPaymentApplyPaid_Terlambat(t *testing.T) {
	db := testdb.Open(t)
	f := testdb.SiapkanTagihan(t, db)
	repo := payment.NewPaymentPostgres(db)
	ctx := context.Background()

	if _, err := order.NewOrderPostgres(db).Cancel(ctx, f.Pesanan.ID); err != nil {
		t.Fatalf("setup: batalkan pesanan gagal: %v", err)
	}

	hasil, err := repo.ApplyPaid(ctx, f.Tagihan.Reference, 150000)

	if err != nil {
		t.Fatalf("apply gagal: %v", err)
	}
	if hasil != domain.OutcomeRefundNeeded {
		t.Errorf("hasil = %q, ingin %q", hasil, domain.OutcomeRefundNeeded)
	}
	if s := testdb.StatusPesanan(t, db, f.Pesanan.ID); s != "cancelled" {
		t.Errorf("status pesanan = %q, ingin tetap cancelled", s)
	}
	if s := testdb.StatusPembayaran(t, db, f.Tagihan.ID); s != "refund_needed" {
		t.Errorf("status pembayaran = %q, ingin refund_needed", s)
	}
	if stok := testdb.StokProduk(t, db, f.Produk); stok != 10 {
		t.Errorf("stok = %d, ingin 10 (sudah dikembalikan oleh pembatalan, dan pembayaran tidak mengubahnya)", stok)
	}

	// Webhook ulang untuk kasus ini tetap dijawab sama dan tidak mengubah apa pun.
	ulang, err := repo.ApplyPaid(ctx, f.Tagihan.Reference, 150000)
	if err != nil || ulang != domain.OutcomeRefundNeeded {
		t.Errorf("webhook ulang: hasil=%q error=%v, ingin %q", ulang, err, domain.OutcomeRefundNeeded)
	}
}
