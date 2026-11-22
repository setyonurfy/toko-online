package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"toko-online/internal/domain"
)

type fakeExpirer struct {
	mu     sync.Mutex
	jumlah int
	calls  chan int
	hasil  func(panggilan int) (int, error)
}

func newFakeExpirer(hasil func(panggilan int) (int, error)) *fakeExpirer {
	return &fakeExpirer{calls: make(chan int, 1000), hasil: hasil}
}

func (f *fakeExpirer) ExpireDue(ctx context.Context) (int, error) {
	f.mu.Lock()
	f.jumlah++
	panggilan := f.jumlah
	f.mu.Unlock()

	select {
	case f.calls <- panggilan:
	default:
	}
	return f.hasil(panggilan)
}

func buatWorker(t *testing.T, f *fakeExpirer, interval time.Duration) *ExpiryWorker {
	t.Helper()
	w, err := NewExpiryWorker(f, interval)
	if err != nil {
		t.Fatalf("NewExpiryWorker gagal: %v", err)
	}
	w.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return w
}

func mulai(w *ExpiryWorker) (context.CancelFunc, <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	selesai := make(chan struct{})
	go func() {
		defer close(selesai)
		w.Run(ctx)
	}()
	return cancel, selesai
}

func tungguPanggilan(t *testing.T, f *fakeExpirer, ingin int) {
	t.Helper()
	batas := time.After(2 * time.Second)
	for {
		select {
		case n := <-f.calls:
			if n >= ingin {
				return
			}
		case <-batas:
			t.Fatalf("panggilan ke-%d tidak terjadi dalam 2 detik", ingin)
		}
	}
}

func tungguSelesai(t *testing.T, selesai <-chan struct{}) {
	t.Helper()
	select {
	case <-selesai:
	case <-time.After(2 * time.Second):
		t.Fatal("Run tidak berhenti setelah konteks dibatalkan")
	}
}

func TestExpiryWorker_LangsungBerjalanSaatMulai(t *testing.T) {
	f := newFakeExpirer(func(int) (int, error) { return 0, nil })
	w := buatWorker(t, f, time.Hour)

	cancel, selesai := mulai(w)
	tungguPanggilan(t, f, 1)
	cancel()
	tungguSelesai(t, selesai)
}

func TestExpiryWorker_BerjalanBerkala(t *testing.T) {
	f := newFakeExpirer(func(int) (int, error) { return 0, nil })
	w := buatWorker(t, f, 5*time.Millisecond)

	cancel, selesai := mulai(w)
	tungguPanggilan(t, f, 3)
	cancel()
	tungguSelesai(t, selesai)
}

func TestExpiryWorker_GalatTidakMenghentikanPekerja(t *testing.T) {
	f := newFakeExpirer(func(panggilan int) (int, error) {
		if panggilan == 1 {
			return 0, errors.New("database mati")
		}
		return 0, nil
	})
	w := buatWorker(t, f, 5*time.Millisecond)

	cancel, selesai := mulai(w)
	tungguPanggilan(t, f, 3)
	cancel()
	tungguSelesai(t, selesai)
}

func TestExpiryWorker_PanicTidakMenghentikanPekerja(t *testing.T) {
	f := newFakeExpirer(func(panggilan int) (int, error) {
		if panggilan == 1 {
			panic("boom")
		}
		return 0, nil
	})
	w := buatWorker(t, f, 5*time.Millisecond)

	cancel, selesai := mulai(w)
	tungguPanggilan(t, f, 3)
	cancel()
	tungguSelesai(t, selesai)
}

func TestNewExpiryWorker_TidakValid(t *testing.T) {
	f := newFakeExpirer(func(int) (int, error) { return 0, nil })

	tests := []struct {
		name     string
		expirer  Expirer
		interval time.Duration
	}{
		{"interval nol", f, 0},
		{"interval negatif", f, -time.Second},
		{"expirer nil", nil, time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewExpiryWorker(tt.expirer, tt.interval)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("error = %v, ingin %v", err, domain.ErrInvalidInput)
			}
			if w != nil {
				t.Error("worker harus nil saat konfigurasi tidak valid")
			}
		})
	}
}
