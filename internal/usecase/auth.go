package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"toko-online/internal/domain"

	"golang.org/x/crypto/bcrypt"
)

type AuthUsecase struct {
	users domain.UserRepository
	cost  int
}

func NewAuthUsecase(users domain.UserRepository) *AuthUsecase {
	return &AuthUsecase{users: users, cost: bcrypt.DefaultCost}
}

func (a *AuthUsecase) Register(ctx context.Context, nama, email, password string) (*domain.User, error) {
	nama = strings.TrimSpace(nama)
	email = strings.ToLower(strings.TrimSpace(email))

	if nama == "" || len(password) < 8 || len(password) > 70 {
		return nil, domain.ErrInvalidInput
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return nil, domain.ErrInvalidInput
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), a.cost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &domain.User{Nama: nama, Email: email, PasswordHash: string(hash)}
	if err := a.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (a *AuthUsecase) Login(ctx context.Context, email, password string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	u, err := a.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	return u, nil
}
