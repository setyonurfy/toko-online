package usecase

import (
	"context"
	"errors"
	"testing"

	"toko-online/internal/domain"
)

func buatPesanan(t *testing.T, uc *OrderUsecase, userID int64) *domain.Order {
	t.Helper()
	o, err := uc.Place(context.Background(), userID, []domain.OrderLine{{ProductID: 1, Jumlah: 1}})
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	return o
}

func TestOrderCancel(t *testing.T) {
	ctx := context.Background()

	t.Run("pemilik membatalkan pesanannya", func(t *testing.T) {
		uc, repo := newTestOrderUsecase()
		o := buatPesanan(t, uc, 1)

		hasil, err := uc.Cancel(ctx, 1, o.ID)
		if err != nil {
			t.Fatalf("cancel gagal: %v", err)
		}
		if hasil.Status != domain.OrderCancelled {
			t.Errorf("status = %q, ingin %q", hasil.Status, domain.OrderCancelled)
		}
		if !repo.cancelCalled || repo.gotCancelID != o.ID {
			t.Errorf("repository dipanggil = %v dengan ID %d, ingin dipanggil dengan ID %d",
				repo.cancelCalled, repo.gotCancelID, o.ID)
		}
	})

	t.Run("bukan pemilik: sama dengan pesanan tidak ada, repository tidak disentuh", func(t *testing.T) {
		uc, repo := newTestOrderUsecase()
		o := buatPesanan(t, uc, 1)

		_, errBukanMilik := uc.Cancel(ctx, 2, o.ID)
		_, errTidakAda := uc.Cancel(ctx, 2, 9999)

		if !errors.Is(errBukanMilik, domain.ErrOrderNotFound) {
			t.Fatalf("error = %v, ingin ErrOrderNotFound", errBukanMilik)
		}
		if errBukanMilik != errTidakAda {
			t.Errorf("jawabannya harus identik agar keberadaan pesanan tidak bocor: %v vs %v", errBukanMilik, errTidakAda)
		}
		if repo.cancelCalled {
			t.Error("pesanan orang lain tidak boleh sampai dibatalkan")
		}
	})

	t.Run("id tidak sah", func(t *testing.T) {
		uc, repo := newTestOrderUsecase()

		_, err := uc.Cancel(ctx, 1, 0)

		if !errors.Is(err, domain.ErrOrderNotFound) {
			t.Fatalf("error = %v, ingin ErrOrderNotFound", err)
		}
		if repo.cancelCalled {
			t.Error("repository tidak boleh dipanggil")
		}
	})

	t.Run("error dari repository diteruskan", func(t *testing.T) {
		uc, repo := newTestOrderUsecase()
		o := buatPesanan(t, uc, 1)
		repo.cancelErr = domain.ErrOrderNotCancellable

		_, err := uc.Cancel(ctx, 1, o.ID)

		if !errors.Is(err, domain.ErrOrderNotCancellable) {
			t.Fatalf("error = %v, ingin ErrOrderNotCancellable", err)
		}
	})
}
