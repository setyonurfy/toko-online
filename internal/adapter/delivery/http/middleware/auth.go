package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"toko-online/internal/adapter/delivery/http/httpx"
	"toko-online/internal/domain"
)

type TokenParser interface {
	Parse(token string) (int64, error)
}

type ctxKey struct {
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(ctxKey{}).(int64)
	return id, ok
}

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func Authenticate(tokens TokenParser, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			httpx.WriteError(w, http.StatusUnauthorized, "token tidak ditemukan")
			return
		}

		id, err := tokens.Parse(strings.TrimSpace(token))
		if err != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			httpx.WriteError(w, http.StatusUnauthorized, "token tidak valid atau kadaluwarsa")
			return
		}

		ctx := WithUserID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type UserFinder interface {
	Profile(ctx context.Context, id int64) (*domain.User, error)
}

func RequireRole(users UserFinder, role domain.Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserIDFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
			return
		}
		user, err := users.Profile(r.Context(), id)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
				return
			}
			slog.Error("cek role gagal", "error", err)
			httpx.WriteError(w, http.StatusInternalServerError, "terjadi kesalahan")
			return
		}
		if user.Role != role {
			httpx.WriteError(w, http.StatusForbidden, "akses ditolak")
			return
		}
		next.ServeHTTP(w, r)
	})
}
