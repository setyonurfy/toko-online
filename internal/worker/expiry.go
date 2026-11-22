package worker

import (
	"context"
	"log/slog"
	"time"
	"toko-online/internal/domain"
)

type Expirer interface {
	ExpireDue(ctx context.Context) (int, error)
}

type ExpiryWorker struct {
	expirer  Expirer
	interval time.Duration
	logger   *slog.Logger
}

func NewExpiryWorker(expirer Expirer, interval time.Duration) (*ExpiryWorker, error) {
	if expirer == nil || interval <= 0 {
		return nil, domain.ErrInvalidInput
	}
	return &ExpiryWorker{expirer: expirer, interval: interval, logger: slog.Default()}, nil
}

func (w *ExpiryWorker) Run(ctx context.Context) {
	w.tick(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *ExpiryWorker) tick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("worker kedaluwarsa: panic dipulihkan", "panic", r)
		}
	}()

	n, err := w.expirer.ExpireDue(ctx)
	if n > 0 {
		w.logger.Info("pesanan kedaluwarsa, stok dikembalikan", "count", n)
	}
	if err != nil && ctx.Err() == nil {
		w.logger.Error("worker kedaluwarsa gagal", "error", err)
	}
}
