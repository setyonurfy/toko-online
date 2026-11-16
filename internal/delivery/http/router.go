package handler

import (
	"net/http"
	"toko-online/internal/domain"
)

type Dependencies struct {
	Auth        *AuthHandler
	Products    *ProductHandler
	Tokens      TokenParser
	Users       UserFinder
	AuthLimiter *RateLimiter
}

func NewRouter(d Dependencies) http.Handler {
	mux := http.NewServeMux()

	adminOnly := func(h http.HandlerFunc) http.Handler {
		return Authenticate(d.Tokens, RequireRole(d.Users, domain.RoleAdmin, h))
	}
	createProduct := adminOnly(d.Products.Create)
	updateProduct := adminOnly(d.Products.Update)
	deleteProduct := adminOnly(d.Products.Delete)

	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/auth/register", d.AuthLimiter.Middleware(http.HandlerFunc(d.Auth.Register)))
	mux.Handle("/auth/login", d.AuthLimiter.Middleware(http.HandlerFunc(d.Auth.Login)))
	mux.Handle("/me", Authenticate(d.Tokens, http.HandlerFunc(d.Auth.Me)))

	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			d.Products.List(w, r)
		case http.MethodPost:
			createProduct.ServeHTTP(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	})

	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			d.Products.Get(w, r)
		case http.MethodPut:
			updateProduct.ServeHTTP(w, r)
		case http.MethodDelete:
			deleteProduct.ServeHTTP(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
		}
	})
	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
