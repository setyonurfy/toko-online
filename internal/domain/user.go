package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUserNotFound       = errors.New("user tidak ditemukan")
	ErrEmailAlreadyUsed   = errors.New("email sudah terdaftar")
	ErrInvalidInput       = errors.New("input tidak valid")
	ErrInvalidCredentials = errors.New("email atau password salah")
)

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

type User struct {
	ID           int64     `json:"id"`
	Nama         string    `json:"nama"`
	Email        string    `json:"email"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
type UserRepository interface {
	Create(ctx context.Context, u *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetById(ctx context.Context, id int64) (*User, error)
}

type LoginResult struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}
