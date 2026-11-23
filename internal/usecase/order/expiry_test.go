package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"toko-online/internal/domain"
)

func newTestExpiryUsecase(ttl time.Duration, batch int, hasil ...int) (*OrderExpiryUsecase, *mockOrderRepo, time.Time) {
	repo := newMockOrderRepo()
	repo.expireResults = hasil
	sekarang := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	uc := NewOrderExpiryUsecase(repo, ttl, batch)
	uc.now = func() time.Time { return sekarang }
	return uc, repo, sekarang
}

func TestExpireDue_MenghitungBatasWaktu(t *testing.T) {
	uc, repo, sekarang := newTestExpiryUsecase(time.Hour, 5, 0)

	if _, err := uc.ExpireDue(context.Background()); err != nil {
		t.Fatalf("ExpireDue gagal: %v", err)
	}
	if want := sekarang.Add(-time.Hour); !repo.gotCutoff.Equal(want) {
		t.Errorf("cutoff = %v, ingin %v", repo.gotCutoff, want)
	}
	if repo.gotBatch != 5 {
		t.Errorf("batch = %d, ingin 5", repo.gotBatch)
	}
}

func TestExpireDue_Menguras(t *testing.T) {
	uc, repo, _ := newTestExpiryUsecase(time.Hour, 3, 3, 3, 1)

	total, err := uc.ExpireDue(context.Background())
	if err != nil {
		t.Fatalf("ExpireDue gagal: %v", err)
	}
	if total != 7 {
		t.Errorf("total = %d, ingin 7", total)
	}
	if repo.expireCalls != 3 {
		t.Errorf("jumlah panggilan repository = %d, ingin 3", repo.expireCalls)
	}
}

func TestExpireDue_BerhentiSaatBatchTidakPenuh(t *testing.T) {
	uc, repo, _ := newTestExpiryUsecase(time.Hour, 3, 2)

	total, err := uc.ExpireDue(context.Background())
	if err != nil {
		t.Fatalf("ExpireDue gagal: %v", err)
	}
	if total != 2 || repo.expireCalls != 1 {
		t.Errorf("total = %d, panggilan = %d, ingin 2 dan 1", total, repo.expireCalls)
	}
}

func TestExpireDue_GalatDariRepository(t *testing.T) {
	uc, repo, _ := newTestExpiryUsecase(time.Hour, 3)
	repo.expireErr = errors.New("database mati")

	total, err := uc.ExpireDue(context.Background())
	if !errors.Is(err, repo.expireErr) {
		t.Errorf("error = %v, ingin %v", err, repo.expireErr)
	}
	if total != 0 || repo.expireCalls != 1 {
		t.Errorf("total = %d, panggilan = %d, ingin 0 dan 1", total, repo.expireCalls)
	}
}

func TestExpireDue_KonfigurasiTidakValid(t *testing.T) {
	tests := []struct {
		name  string
		ttl   time.Duration
		batch int
	}{
		{"ttl nol", 0, 5},
		{"ttl negatif", -time.Hour, 5},
		{"batch nol", time.Hour, 0},
		{"batch negatif", time.Hour, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, repo, _ := newTestExpiryUsecase(tt.ttl, tt.batch)

			_, err := uc.ExpireDue(context.Background())
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("error = %v, ingin %v", err, domain.ErrInvalidInput)
			}
			if repo.expireCalls != 0 {
				t.Errorf("repository tidak boleh dipanggil, dipanggil %d kali", repo.expireCalls)
			}
		})
	}
}

func TestExpireDue_KonteksBatal(t *testing.T) {
	uc, repo, _ := newTestExpiryUsecase(time.Hour, 3, 3)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := uc.ExpireDue(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, ingin %v", err, context.Canceled)
	}
	if repo.expireCalls != 0 {
		t.Errorf("repository tidak boleh dipanggil, dipanggil %d kali", repo.expireCalls)
	}
}
