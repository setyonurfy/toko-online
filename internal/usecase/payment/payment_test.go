package payment

import (
	"context"
	"errors"
	"testing"

	"toko-online/internal/domain"
)

func TestPay(t *testing.T) {
	ctx := context.Background()
	pesananPending := &domain.Order{ID: 5, UserID: 7, Status: domain.OrderPending, Total: 150000}

	t.Run("sukses membuat tagihan baru", func(t *testing.T) {
		uc, repo, gw := newTestPaymentUsecase(pesananPending, nil)

		p, err := uc.Pay(ctx, 7, 5)
		if err != nil {
			t.Fatalf("pay gagal: %v", err)
		}

		if p.Reference != "SIM-uji" {
			t.Errorf("referensi = %q, ingin SIM-uji", p.Reference)
		}
		if gw.chargeCalls != 1 || gw.gotAmount != 150000 {
			t.Errorf("gateway dipanggil %d kali dengan jumlah %d, ingin 1 kali dengan 150000", gw.chargeCalls, gw.gotAmount)
		}
		if repo.createCalls != 1 || repo.gotOrderID != 5 || repo.gotRef != "SIM-uji" {
			t.Errorf("repository menerima (order=%d, ref=%q, panggilan=%d), ingin (5, SIM-uji, 1)",
				repo.gotOrderID, repo.gotRef, repo.createCalls)
		}
	})

	t.Run("pesanan orang lain atau tidak ada", func(t *testing.T) {
		uc, repo, gw := newTestPaymentUsecase(nil, domain.ErrOrderNotFound)

		_, err := uc.Pay(ctx, 7, 5)

		if !errors.Is(err, domain.ErrOrderNotFound) {
			t.Fatalf("error = %v, ingin ErrOrderNotFound", err)
		}
		if gw.chargeCalls != 0 || repo.createCalls != 0 {
			t.Error("gateway dan repository tidak boleh disentuh")
		}
	})

	for _, status := range []domain.OrderStatus{domain.OrderPaid, domain.OrderCancelled} {
		t.Run("pesanan berstatus "+string(status), func(t *testing.T) {
			order := &domain.Order{ID: 5, UserID: 7, Status: status, Total: 1000}
			uc, repo, gw := newTestPaymentUsecase(order, nil)

			_, err := uc.Pay(ctx, 7, 5)

			if !errors.Is(err, domain.ErrOrderNotPayable) {
				t.Fatalf("error = %v, ingin ErrOrderNotPayable", err)
			}
			if gw.chargeCalls != 0 || repo.createCalls != 0 {
				t.Error("gateway dan repository tidak boleh disentuh")
			}
		})
	}

	t.Run("tagihan pending yang ada dipakai ulang tanpa menghubungi gateway", func(t *testing.T) {
		uc, repo, gw := newTestPaymentUsecase(pesananPending, nil)
		repo.pending = &domain.Payment{ID: 9, OrderID: 5, Reference: "SIM-lama", Amount: 150000, Status: domain.PaymentPending}

		p, err := uc.Pay(ctx, 7, 5)

		if err != nil {
			t.Fatalf("pay gagal: %v", err)
		}
		if p.Reference != "SIM-lama" {
			t.Errorf("referensi = %q, ingin SIM-lama (tagihan yang sudah ada)", p.Reference)
		}
		if gw.chargeCalls != 0 || repo.createCalls != 0 {
			t.Errorf("gateway dipanggil %d kali dan create %d kali, ingin 0 dan 0", gw.chargeCalls, repo.createCalls)
		}
	})

	t.Run("gateway gagal", func(t *testing.T) {
		uc, repo, gw := newTestPaymentUsecase(pesananPending, nil)
		gw.chargeErr = errors.New("gateway mati")

		_, err := uc.Pay(ctx, 7, 5)

		if !errors.Is(err, gw.chargeErr) {
			t.Fatalf("error = %v, ingin membungkus %v", err, gw.chargeErr)
		}
		if repo.createCalls != 0 {
			t.Error("tagihan tidak boleh disimpan kalau gateway gagal")
		}
	})

	t.Run("error saat memeriksa tagihan yang ada diteruskan", func(t *testing.T) {
		uc, _, gw := newTestPaymentUsecase(pesananPending, nil)
		uc.payments.(*mockPaymentRepo).getErr = errors.New("db mati")

		_, err := uc.Pay(ctx, 7, 5)

		if err == nil {
			t.Fatal("error seharusnya diteruskan")
		}
		if gw.chargeCalls != 0 {
			t.Error("gateway tidak boleh dihubungi kalau pemeriksaan tagihan gagal")
		}
	})

	t.Run("error dari repository saat menyimpan diteruskan", func(t *testing.T) {
		uc, repo, _ := newTestPaymentUsecase(pesananPending, nil)
		repo.createErr = domain.ErrOrderNotPayable

		_, err := uc.Pay(ctx, 7, 5)

		if !errors.Is(err, domain.ErrOrderNotPayable) {
			t.Fatalf("error = %v, ingin ErrOrderNotPayable", err)
		}
	})
}

func TestHandleWebhook(t *testing.T) {
	ctx := context.Background()
	body, ts, sig := []byte(`{}`), "123", "abc"
	eventBayar := domain.GatewayEvent{Type: domain.EventPaymentPaid, Reference: "SIM-1", Amount: 150000}

	t.Run("verifikasi gagal tidak menyentuh penyimpanan", func(t *testing.T) {
		for _, gagal := range []error{domain.ErrInvalidSignature, domain.ErrStaleWebhook, domain.ErrMalformedWebhook} {
			uc, repo, gw := newTestPaymentUsecase(nil, nil)
			gw.parseErr = gagal

			_, err := uc.HandleWebhook(ctx, body, ts, sig)

			if !errors.Is(err, gagal) {
				t.Errorf("error = %v, ingin %v", err, gagal)
			}
			if repo.applyCalls != 0 {
				t.Errorf("%v: penyimpanan tidak boleh disentuh", gagal)
			}
		}
	})

	t.Run("jenis peristiwa lain diabaikan", func(t *testing.T) {
		uc, repo, gw := newTestPaymentUsecase(nil, nil)
		gw.event = domain.GatewayEvent{Type: "payment.failed", Reference: "SIM-1", Amount: 150000}

		outcome, err := uc.HandleWebhook(ctx, body, ts, sig)

		if err != nil {
			t.Fatalf("error = %v, ingin nil", err)
		}
		if outcome != domain.OutcomeIgnored {
			t.Errorf("hasil = %q, ingin %q", outcome, domain.OutcomeIgnored)
		}
		if repo.applyCalls != 0 {
			t.Error("penyimpanan tidak boleh disentuh untuk peristiwa yang diabaikan")
		}
	})

	for _, want := range []domain.PaymentOutcome{domain.OutcomePaid, domain.OutcomeAlreadyPaid, domain.OutcomeRefundNeeded} {
		t.Run("hasil "+string(want)+" diteruskan", func(t *testing.T) {
			uc, repo, gw := newTestPaymentUsecase(nil, nil)
			gw.event = eventBayar
			repo.applyOutcome = want

			got, err := uc.HandleWebhook(ctx, body, ts, sig)

			if err != nil {
				t.Fatalf("error = %v, ingin nil", err)
			}
			if got != want {
				t.Errorf("hasil = %q, ingin %q", got, want)
			}
			if repo.gotApplyRef != "SIM-1" || repo.gotApplyAmt != 150000 {
				t.Errorf("repository menerima (ref=%q, jumlah=%d), ingin (SIM-1, 150000)", repo.gotApplyRef, repo.gotApplyAmt)
			}
		})
	}

	for _, wantErr := range []error{domain.ErrPaymentNotFound, domain.ErrAmountMismatch} {
		t.Run("error "+wantErr.Error()+" diteruskan", func(t *testing.T) {
			uc, repo, gw := newTestPaymentUsecase(nil, nil)
			gw.event = eventBayar
			repo.applyErr = wantErr

			_, err := uc.HandleWebhook(ctx, body, ts, sig)

			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, ingin %v", err, wantErr)
			}
		})
	}
}
