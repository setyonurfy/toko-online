package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "secret-untuk-test-minimal-32-karakter!!"

func mustGenerate(t *testing.T, m *TokenManager, id int64) string {
	t.Helper()
	token, err := m.Generate(id)
	if err != nil {
		t.Fatalf("generate gagal: %v", err)
	}
	return token
}

func TestTokenManager(t *testing.T) {
	m := NewTokenManager(testSecret, time.Hour)

	t.Run("generate lalu parse", func(t *testing.T) {
		token := mustGenerate(t, m, 42)

		id, err := m.Parse(token)
		if err != nil {
			t.Fatalf("parse gagal: %v", err)
		}
		if id != 42 {
			t.Errorf("id = %d, ingin 42", id)
		}
	})

	t.Run("token kedaluwarsa", func(t *testing.T) {
		lama := NewTokenManager(testSecret, time.Hour)
		lama.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
		token := mustGenerate(t, lama, 42) // terbit 2 jam lalu, kedaluwarsa 1 jam lalu

		if _, err := m.Parse(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, ingin ErrInvalidToken", err)
		}
	})

	t.Run("secret berbeda", func(t *testing.T) {
		lain := NewTokenManager("secret-lain-yang-juga-cukup-panjang!!", time.Hour)
		token := mustGenerate(t, lain, 42)

		if _, err := m.Parse(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, ingin ErrInvalidToken", err)
		}
	})

	t.Run("signature diubah", func(t *testing.T) {
		token := mustGenerate(t, m, 42)

		bagian := strings.Split(token, ".")
		awal := "A"
		if strings.HasPrefix(bagian[2], "A") {
			awal = "B"
		}
		bagian[2] = awal + bagian[2][1:]
		rusak := strings.Join(bagian, ".")

		if _, err := m.Parse(rusak); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, ingin ErrInvalidToken", err)
		}
	})

	t.Run("bukan token", func(t *testing.T) {
		if _, err := m.Parse("bukan-token"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, ingin ErrInvalidToken", err)
		}
	})
}
