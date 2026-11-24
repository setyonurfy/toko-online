package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"toko-online/internal/domain"

	"golang.org/x/crypto/bcrypt"
)

type fakeTokens struct{}

func (fakeTokens) Generate(userID int64) (string, error) {
	return "token-palsu", nil
}

func newTestUsecase() *AuthUsecase {
	uc := NewAuthUsecase(newMockRepo(), fakeTokens{})
	uc.setCost(bcrypt.MinCost)
	return uc
}

type mockUserRepo struct {
	byEmail map[string]*domain.User
	nextID  int64
}

var _ domain.UserRepository = (*mockUserRepo)(nil)

func newMockRepo() *mockUserRepo {
	return &mockUserRepo{byEmail: make(map[string]*domain.User)}
}

func (m *mockUserRepo) Create(ctx context.Context, u *domain.User) error {
	if _, ada := m.byEmail[u.Email]; ada {
		return domain.ErrEmailAlreadyUsed
	}
	m.nextID++
	u.ID = m.nextID
	u.Role = domain.RoleCustomer
	salinan := *u
	m.byEmail[u.Email] = &salinan
	return nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, ada := m.byEmail[email]
	if !ada {
		return nil, domain.ErrUserNotFound
	}
	salinan := *u
	return &salinan, nil
}

func (m *mockUserRepo) GetById(ctx context.Context, id int64) (*domain.User, error) {
	for _, u := range m.byEmail {
		if u.ID == id {
			salinan := *u
			return &salinan, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

// test

func TestRegister(t *testing.T) {
	tests := []struct {
		alert    string
		nama     string
		email    string
		password string
		wantErr  error
	}{
		{"sukses", "john doe", "jhondoe@mail.com", "pass12345", nil},
		{"nama kosong", "", "jhondoe@mail.com", "pass12345", domain.ErrInvalidInput},
		{"email salah format", "john doe", "bukan-email", "pass12345", domain.ErrInvalidInput},
		{"password terlalu pendek", "john doe", "jhondoe@mail.com", "pass", domain.ErrInvalidInput},
		{"nama tepat 100 karakter", strings.Repeat("a", 100), "jhondoe@mail.com", "pass12345", nil},
		{"nama 100 karakter multibyte", strings.Repeat("é", 100), "jhondoe@mail.com", "pass12345", nil},
		{"nama 101 karakter", strings.Repeat("a", 101), "jhondoe@mail.com", "pass12345", domain.ErrInvalidInput},
		{"email lebih dari 255 karakter", "john doe", strings.Repeat("a", 250) + "@mail.com", "pass12345", domain.ErrInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.alert, func(t *testing.T) {
			uc := newTestUsecase()

			u, err := uc.Register(context.Background(), tt.nama, tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, ingin %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if u.ID == 0 {
				t.Error("ID seharusnya terisi")
			}
			if u.PasswordHash == tt.password {
				t.Error("password tidak boleh disimpan")
			}
			if u.Role != domain.RoleCustomer {
				t.Errorf("role = %q, must be %q", u.Role, domain.RoleCustomer)
			}
		})
	}
}

func TestRegister_EmailSudahTerdaftar(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	if _, err := uc.Register(ctx, "jhon doe", "jhondoe@mail.com", "pass12345"); err != nil {
		t.Fatalf("register gagal: %v", err)
	}
	_, err := uc.Register(ctx, "jhon da", "jhondoe@mail.com", "pass5487543")

	if !errors.Is(err, domain.ErrEmailAlreadyUsed) {
		t.Fatalf("error = %v, ingin %v", err, domain.ErrEmailAlreadyUsed)
	}
}

func TestLogin(t *testing.T) {
	uc := newTestUsecase()
	ctx := context.Background()

	if _, err := uc.Register(ctx, "jhon doe", "jhondoe@mail.com", "pass12345"); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	tests := []struct {
		alert    string
		email    string
		password string
		wantErr  error
	}{
		{"sukses", "jhondoe@mail.com", "pass12345", nil},
		{"email huruf besar", "JHONDOE@mail.com", "pass12345", nil},
		{"password salah", "jhondoe@mail.com", "pw-salah", domain.ErrInvalidCredentials},
		{"email tidak terdaftar", "bukan@mail.com", "pass12345", domain.ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.alert, func(t *testing.T) {
			res, err := uc.Login(ctx, tt.email, tt.password)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if res.User.Email != "jhondoe@mail.com" {
					t.Errorf("email = %q, want jhondoe@mail.com", res.User.Email)
				}
				if res.Token != "token-palsu" {
					t.Errorf("token = %q, must be token-palsu", res.Token)
				}

			}
		})
	}
}

func TestLogin_EmailTidakAda_TetapMenjalankanBcrypt(t *testing.T) {
	uc := newTestUsecase()

	panggilan := 0
	uc.compare = func(hashed, password []byte) error {
		panggilan++
		return bcrypt.CompareHashAndPassword(hashed, password)
	}

	_, err := uc.Login(context.Background(), "tidak-ada@mail.com", "apa-saja")

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, ingin %v", err, domain.ErrInvalidCredentials)
	}
	if panggilan != 1 {
		t.Errorf("bcrypt dijalankan %d kali, ingin 1 (agar waktu respons tidak membocorkan email)", panggilan)
	}
}
