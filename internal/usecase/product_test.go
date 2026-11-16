package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"toko-online/internal/domain"
)

// ---- mock repository ----

type mockProductRepo struct {
	data       map[int64]*domain.Product
	nextID     int64
	lastLimit  int
	lastOffset int
}

var _ domain.ProductRepository = (*mockProductRepo)(nil)

func newMockProductRepo() *mockProductRepo {
	return &mockProductRepo{data: make(map[int64]*domain.Product)}
}

func (m *mockProductRepo) Create(ctx context.Context, p *domain.Product) error {
	m.nextID++
	p.ID = m.nextID
	salinan := *p
	m.data[p.ID] = &salinan
	return nil
}

func (m *mockProductRepo) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	p, ada := m.data[id]
	if !ada {
		return nil, domain.ErrProductNotFound
	}
	salinan := *p
	return &salinan, nil
}

func (m *mockProductRepo) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	m.lastLimit, m.lastOffset = limit, offset
	hasil := make([]domain.Product, 0, len(m.data))
	for _, p := range m.data {
		hasil = append(hasil, *p)
	}
	return hasil, nil
}

func (m *mockProductRepo) Update(ctx context.Context, p *domain.Product) error {
	if _, ada := m.data[p.ID]; !ada {
		return domain.ErrProductNotFound
	}
	salinan := *p
	m.data[p.ID] = &salinan
	return nil
}

func (m *mockProductRepo) Delete(ctx context.Context, id int64) error {
	if _, ada := m.data[id]; !ada {
		return domain.ErrProductNotFound
	}
	delete(m.data, id)
	return nil
}

// ---- helper ----

func newTestProductUsecase() (*ProductUsecase, *mockProductRepo) {
	repo := newMockProductRepo()
	return NewProductUsecase(repo), repo
}

func siapkanProduk(t *testing.T) (*ProductUsecase, *mockProductRepo, *domain.Product) {
	t.Helper()
	uc, repo := newTestProductUsecase()
	p, err := uc.Create(context.Background(), "Kopi", "Enak", 50000, 5)
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	return uc, repo, p
}

// ---- test ----

func TestProductCreate(t *testing.T) {
	tests := []struct {
		name      string
		nama      string
		deskripsi string
		harga     int64
		stok      int
		wantErr   error
	}{
		{"sukses", "Kopi Arabika", "Biji kopi 250 gram", 55000, 10, nil},
		{"nama dirapikan", "  Kopi  ", "", 55000, 10, nil},
		{"stok nol boleh", "Teh Hijau", "", 20000, 0, nil},
		{"nama kosong", "", "x", 55000, 10, domain.ErrInvalidInput},
		{"nama hanya spasi", "   ", "x", 55000, 10, domain.ErrInvalidInput},
		{"nama terlalu panjang", strings.Repeat("a", 151), "", 55000, 10, domain.ErrInvalidInput},
		{"deskripsi terlalu panjang", "Kopi", strings.Repeat("a", 2001), 55000, 10, domain.ErrInvalidInput},
		{"harga nol", "Kopi", "", 0, 10, domain.ErrInvalidInput},
		{"harga negatif", "Kopi", "", -1000, 10, domain.ErrInvalidInput},
		{"harga terlalu besar", "Kopi", "", 1_000_000_001, 10, domain.ErrInvalidInput},
		{"stok negatif", "Kopi", "", 55000, -1, domain.ErrInvalidInput},
		{"stok terlalu besar", "Kopi", "", 55000, 1_000_001, domain.ErrInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, repo := newTestProductUsecase()

			p, err := uc.Create(context.Background(), tt.nama, tt.deskripsi, tt.harga, tt.stok)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, ingin %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if len(repo.data) != 0 {
					t.Error("produk tidak valid tidak boleh sampai ke repository")
				}
				return
			}
			if p.ID == 0 {
				t.Error("ID seharusnya terisi")
			}
			if p.Nama != strings.TrimSpace(tt.nama) {
				t.Errorf("nama = %q, ingin %q", p.Nama, strings.TrimSpace(tt.nama))
			}
		})
	}
}

func TestProductList_NormalisasiPaginasi(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{"nol memakai bawaan", 0, 0, 20, 0},
		{"negatif dibetulkan", -5, -1, 20, 0},
		{"terlalu besar dipotong", 1000, 10, 100, 10},
		{"nilai wajar dipakai apa adanya", 10, 5, 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, repo := newTestProductUsecase()

			if _, err := uc.List(context.Background(), tt.limit, tt.offset); err != nil {
				t.Fatalf("list gagal: %v", err)
			}
			if repo.lastLimit != tt.wantLimit || repo.lastOffset != tt.wantOffset {
				t.Errorf("repository menerima (limit=%d, offset=%d), ingin (%d, %d)",
					repo.lastLimit, repo.lastOffset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

func TestProductGet(t *testing.T) {
	uc, _, p := siapkanProduk(t)
	ctx := context.Background()

	if got, err := uc.Get(ctx, p.ID); err != nil || got.Nama != "Kopi" {
		t.Errorf("get produk ada: got=%v err=%v", got, err)
	}
	if _, err := uc.Get(ctx, 999); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("id tidak ada: error = %v, ingin ErrProductNotFound", err)
	}
	if _, err := uc.Get(ctx, 0); !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("id nol: error = %v, ingin ErrProductNotFound", err)
	}
}

func TestProductUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("sukses", func(t *testing.T) {
		uc, repo, p := siapkanProduk(t)

		if _, err := uc.Update(ctx, p.ID, "Kopi Gayo", "Lebih enak", 60000, 8); err != nil {
			t.Fatalf("update gagal: %v", err)
		}
		if tersimpan := repo.data[p.ID]; tersimpan.Harga != 60000 || tersimpan.Nama != "Kopi Gayo" {
			t.Errorf("data tersimpan tidak sesuai: %+v", tersimpan)
		}
	})

	t.Run("tidak valid tidak mengubah data", func(t *testing.T) {
		uc, repo, p := siapkanProduk(t)

		_, err := uc.Update(ctx, p.ID, "", "x", 1, 1)

		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("error = %v, ingin ErrInvalidInput", err)
		}
		if repo.data[p.ID].Nama != "Kopi" {
			t.Error("data lama tidak boleh berubah saat input tidak valid")
		}
	})

	t.Run("id tidak ada", func(t *testing.T) {
		uc, _, _ := siapkanProduk(t)

		_, err := uc.Update(ctx, 999, "Kopi", "", 50000, 1)

		if !errors.Is(err, domain.ErrProductNotFound) {
			t.Fatalf("error = %v, ingin ErrProductNotFound", err)
		}
	})
}

func TestProductDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("sukses", func(t *testing.T) {
		uc, _, p := siapkanProduk(t)

		if err := uc.Delete(ctx, p.ID); err != nil {
			t.Fatalf("delete gagal: %v", err)
		}
		if _, err := uc.Get(ctx, p.ID); !errors.Is(err, domain.ErrProductNotFound) {
			t.Errorf("setelah dihapus, get seharusnya ErrProductNotFound, dapat %v", err)
		}
	})

	t.Run("id tidak ada", func(t *testing.T) {
		uc, _, _ := siapkanProduk(t)
		if err := uc.Delete(ctx, 999); !errors.Is(err, domain.ErrProductNotFound) {
			t.Errorf("error = %v, ingin ErrProductNotFound", err)
		}
	})
}
