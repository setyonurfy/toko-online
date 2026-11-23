package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"toko-online/internal/domain"

	"github.com/lib/pq"
)

type userPostgres struct {
	db *sql.DB
}

func NewUserPostgres(db *sql.DB) domain.UserRepository {
	return &userPostgres{db: db}
}

func (r *userPostgres) Create(ctx context.Context, u *domain.User) error {
	const q = `
	INSERT INTO users (nama, email, password_hash)
	VALUES ($1, $2, $3)
	RETURNING id, role, created_at`

	err := r.db.QueryRowContext(ctx, q, u.Nama, u.Email, u.PasswordHash).Scan(&u.ID, &u.Role, &u.CreatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return domain.ErrEmailAlreadyUsed
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *userPostgres) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
	SELECT id, nama, email, password_hash, role, created_at
	FROM users WHERE email = $1`

	var u domain.User
	err := r.db.QueryRowContext(ctx, q, email).Scan(&u.ID, &u.Nama, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil

}

func (r *userPostgres) GetById(ctx context.Context, id int64) (*domain.User, error) {
	const q = `
		SELECT id, nama, email, password_hash, role, created_at
		FROM users WHERE id = $1`

	var u domain.User
	err := r.db.QueryRowContext(ctx, q, id).Scan(&u.ID, &u.Nama, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}
