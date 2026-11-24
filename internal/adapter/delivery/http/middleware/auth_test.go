package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"toko-online/internal/domain"
)

type fakeUsers struct {
	user *domain.User
	err  error
}

func (f *fakeUsers) Profile(ctx context.Context, id int64) (*domain.User, error) {
	return f.user, f.err
}

func TestRequireRole(t *testing.T) {
	admin := &domain.User{ID: 1, Role: domain.RoleAdmin}
	pembeli := &domain.User{ID: 2, Role: domain.RoleCustomer}

	tests := []struct {
		name          string
		adaIDDiCtx    bool
		svc           *fakeUsers
		wantStatus    int
		wantDipanggil bool
	}{
		{"admin diizinkan", true, &fakeUsers{user: admin}, http.StatusOK, true},
		{"pembeli ditolak", true, &fakeUsers{user: pembeli}, http.StatusForbidden, false},
		{"tanpa identitas", false, &fakeUsers{user: admin}, http.StatusUnauthorized, false},
		{"akun sudah tidak ada", true, &fakeUsers{err: domain.ErrUserNotFound}, http.StatusUnauthorized, false},
		{"database error", true, &fakeUsers{err: errors.New("db mati")}, http.StatusInternalServerError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dipanggil := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				dipanggil = true
				w.WriteHeader(http.StatusOK)
			})
			h := RequireRole(tt.svc, domain.RoleAdmin, next)

			req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
			if tt.adaIDDiCtx {
				req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, int64(7)))
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, ingin %d", rec.Code, tt.wantStatus)
			}
			if dipanggil != tt.wantDipanggil {
				t.Errorf("handler dipanggil = %v, ingin %v", dipanggil, tt.wantDipanggil)
			}
		})
	}
}
