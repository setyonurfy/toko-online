package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"toko-online/internal/domain"
)

// ---- mock repository ----

type mockOrderRepo struct {
	orders map[int64]*domain.Order
	nextID int64

	createErr    error
	createCalled bool
	gotUserID    int64
	gotLines     []domain.OrderLine

	lastLimit, lastOffset int
}

var _ domain.OrderRepository = (*mockOrderRepo)(nil)

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{orders: make(map[int64]*domain.Order)}
}

func (m *mockOrderRepo) Create(ctx context.Context, userID int64, lines []domain.OrderLine) (*domain.Order, error) {
	m.createCalled = true
	m.gotUserID = userID
	m.gotLines = slices.Clone(lines)
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.nextID++
	o := &domain.Order{ID: m.nextID, UserID: userID, Status: domain.OrderPending, Total: 1000}
	m.orders[o.ID] = o
	salinan := *o
	return &salinan, nil
}

func (m *mockOrderRepo) GetByID(ctx context.Context, id int64) (*domain.Order, error) {
	o, ada := m.orders[id]
	if !ada {
		return nil, domain.ErrOrderNotFound
	}
	salinan := *o
	return &salinan, nil
}

func (m *mockOrderRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Order, error) {
	m.lastLimit, m.lastOffset = limit, offset
	hasil := make([]domain.Order, 0)
	for _, o := range m.orders {
		if o.UserID == userID {
			hasil = append(hasil, *o)
		}
	}
	return hasil, nil
}

func newTestOrderUsecase() (*OrderUsecase, *mockOrderRepo) {
	repo := newMockOrderRepo()
	return NewOrderUsecase(repo), repo
}

// ---- test ----

func TestOrderPlace_Validasi(t *testing.T) {
	banyak := make([]domain.OrderLine, 51)
	for i := range banyak {
		banyak[i] = domain.OrderLine{ProductID: int64(i + 1), Jumlah: 1}
	}

	tests := []struct {
		name    string
		userID  int64
		lines   []domain.OrderLine
		wantErr error
	}{
		{"sukses satu baris", 7, []domain.OrderLine{{ProductID: 1, Jumlah: 2}}, nil},
		{"sukses beberapa baris", 7, []domain.OrderLine{{ProductID: 1, Jumlah: 2}, {ProductID: 2, Jumlah: 1}}, nil},
		{"keranjang kosong", 7, nil, domain.ErrInvalidInput},
		{"terlalu banyak baris", 7, banyak, domain.ErrInvalidInput},
		{"id produk nol", 7, []domain.OrderLine{{ProductID: 0, Jumlah: 1}}, domain.ErrInvalidInput},
		{"id produk negatif", 7, []domain.OrderLine{{ProductID: -1, Jumlah: 1}}, domain.ErrInvalidInput},
		{"jumlah nol", 7, []domain.OrderLine{{ProductID: 1, Jumlah: 0}}, domain.ErrInvalidInput},
		{"jumlah negatif", 7, []domain.OrderLine{{ProductID: 1, Jumlah: -3}}, domain.ErrInvalidInput},
		{"jumlah terlalu besar", 7, []domain.OrderLine{{ProductID: 1, Jumlah: 1001}}, domain.ErrInvalidInput},
		{"user tidak sah", 0, []domain.OrderLine{{ProductID: 1, Jumlah: 1}}, domain.ErrInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, repo := newTestOrderUsecase()

			o, err := uc.Place(context.Background(), tt.userID, tt.lines)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, ingin %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if repo.createCalled {
					t.Error("input tidak valid tidak boleh sampai ke repository")
				}
				return
			}
			if o.ID == 0 || o.UserID != tt.userID {
				t.Errorf("pesanan tidak sesuai: %+v", o)
			}
		})
	}
}

func TestOrderPlace_GabungDanUrutkan(t *testing.T) {
	uc, repo := newTestOrderUsecase()

	_, err := uc.Place(context.Background(), 7, []domain.OrderLine{
		{ProductID: 3, Jumlah: 2},
		{ProductID: 1, Jumlah: 1},
		{ProductID: 3, Jumlah: 4},
	})
	if err != nil {
		t.Fatalf("place gagal: %v", err)
	}

	want := []domain.OrderLine{{ProductID: 1, Jumlah: 1}, {ProductID: 3, Jumlah: 6}}
	if !slices.Equal(repo.gotLines, want) {
		t.Errorf("repository menerima %v, ingin %v (digabung dan terurut)", repo.gotLines, want)
	}
	if repo.gotUserID != 7 {
		t.Errorf("userID = %d, ingin 7", repo.gotUserID)
	}
}

func TestOrderPlace_PemecahanTidakMelewatiBatas(t *testing.T) {
	uc, repo := newTestOrderUsecase()

	// 600 + 600 per baris wajar, tetapi gabungannya 1200 > 1000.
	_, err := uc.Place(context.Background(), 7, []domain.OrderLine{
		{ProductID: 1, Jumlah: 600},
		{ProductID: 1, Jumlah: 600},
	})

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, ingin ErrInvalidInput", err)
	}
	if repo.createCalled {
		t.Error("repository tidak boleh dipanggil")
	}
}

func TestOrderPlace_ErrorRepositoryDiteruskan(t *testing.T) {
	for _, wantErr := range []error{domain.ErrInsufficientStock, domain.ErrProductNotFound} {
		t.Run(wantErr.Error(), func(t *testing.T) {
			uc, repo := newTestOrderUsecase()
			repo.createErr = wantErr

			_, err := uc.Place(context.Background(), 7, []domain.OrderLine{{ProductID: 1, Jumlah: 1}})

			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, ingin %v", err, wantErr)
			}
		})
	}
}

func TestOrderGet_Kepemilikan(t *testing.T) {
	uc, _ := newTestOrderUsecase()
	ctx := context.Background()

	milikSiti, err := uc.Place(ctx, 1, []domain.OrderLine{{ProductID: 1, Jumlah: 1}})
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	t.Run("pemilik boleh melihat", func(t *testing.T) {
		o, err := uc.Get(ctx, 1, milikSiti.ID)
		if err != nil || o.ID != milikSiti.ID {
			t.Fatalf("get oleh pemilik: o=%v err=%v", o, err)
		}
	})

	t.Run("orang lain mendapat jawaban yang sama dengan pesanan tidak ada", func(t *testing.T) {
		_, errBukanMilik := uc.Get(ctx, 2, milikSiti.ID)
		_, errTidakAda := uc.Get(ctx, 2, 9999)

		if !errors.Is(errBukanMilik, domain.ErrOrderNotFound) {
			t.Fatalf("pesanan orang lain: error = %v, ingin ErrOrderNotFound", errBukanMilik)
		}
		if errBukanMilik != errTidakAda {
			t.Errorf("jawabannya harus identik agar keberadaan pesanan tidak bocor: %v vs %v", errBukanMilik, errTidakAda)
		}
	})

	t.Run("id tidak sah", func(t *testing.T) {
		if _, err := uc.Get(ctx, 1, 0); !errors.Is(err, domain.ErrOrderNotFound) {
			t.Errorf("error = %v, ingin ErrOrderNotFound", err)
		}
	})
}

func TestOrderListMine(t *testing.T) {
	uc, repo := newTestOrderUsecase()
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := uc.Place(ctx, 1, []domain.OrderLine{{ProductID: 1, Jumlah: 1}}); err != nil {
			t.Fatalf("setup gagal: %v", err)
		}
	}
	if _, err := uc.Place(ctx, 2, []domain.OrderLine{{ProductID: 1, Jumlah: 1}}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	hasil, err := uc.ListMine(ctx, 1, 1000, -5)
	if err != nil {
		t.Fatalf("list gagal: %v", err)
	}
	if len(hasil) != 2 {
		t.Errorf("jumlah pesanan = %d, ingin 2 (hanya milik user 1)", len(hasil))
	}
	if repo.lastLimit != 100 || repo.lastOffset != 0 {
		t.Errorf("repository menerima (limit=%d, offset=%d), ingin (100, 0)", repo.lastLimit, repo.lastOffset)
	}
}
