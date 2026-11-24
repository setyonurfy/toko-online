package handler

import (
	"net/http"
	"strings"

	"toko-online/internal/adapter/delivery/http/auth"
	"toko-online/internal/adapter/delivery/http/httpx"
	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/adapter/delivery/http/order"
	"toko-online/internal/adapter/delivery/http/payment"
	"toko-online/internal/adapter/delivery/http/product"
	"toko-online/internal/domain"
)

type Dependencies struct {
	Auth          *auth.AuthHandler
	Products      *product.ProductHandler
	Orders        *order.OrderHandler
	Tokens        middleware.TokenParser
	Users         middleware.UserFinder
	AuthLimiter   *middleware.RateLimiter
	ReadLimiter   *middleware.RateLimiter
	ActionLimiter *middleware.RateLimiter
	AdminLimiter  *middleware.RateLimiter
	Payments      *payment.PaymentHandler
}

func NewRouter(d Dependencies) http.Handler {
	mux := http.NewServeMux()

	adminOnly := func(h http.HandlerFunc) http.Handler {
		return middleware.Authenticate(d.Tokens, d.AdminLimiter.MiddlewareByUser(middleware.RequireRole(d.Users, domain.RoleAdmin, h)))
	}
	userRead := func(h http.HandlerFunc) http.Handler {
		return middleware.Authenticate(d.Tokens, d.ReadLimiter.MiddlewareByUser(h))
	}
	userAction := func(h http.HandlerFunc) http.Handler {
		return middleware.Authenticate(d.Tokens, d.ActionLimiter.MiddlewareByUser(h))
	}

	createProduct := adminOnly(d.Products.Create)
	updateProduct := adminOnly(d.Products.Update)
	deleteProduct := adminOnly(d.Products.Delete)

	createOrder := userAction(d.Orders.Create)
	listOrders := userRead(d.Orders.List)
	getOrder := userRead(d.Orders.Get)
	cancelOrder := userAction(d.Orders.Cancel)
	payOrder := userAction(d.Payments.Pay)

	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/auth/register", d.AuthLimiter.Middleware(http.HandlerFunc(d.Auth.Register)))
	mux.Handle("/auth/login", d.AuthLimiter.Middleware(http.HandlerFunc(d.Auth.Login)))
	mux.Handle("/me", userRead(d.Auth.Me))

	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			d.Products.List(w, r)
		case http.MethodPost:
			createProduct.ServeHTTP(w, r)
		default:
			httpx.MethodNotAllowed(w, http.MethodGet, http.MethodPost)
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
			httpx.MethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
		}
	})

	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listOrders.ServeHTTP(w, r)
		case http.MethodPost:
			createOrder.ServeHTTP(w, r)
		default:
			httpx.MethodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	})

	mux.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cancel"):
			if r.Method != http.MethodPost {
				httpx.MethodNotAllowed(w, http.MethodPost)
				return
			}
			cancelOrder.ServeHTTP(w, r)
		case strings.HasSuffix(r.URL.Path, "/pay"):
			if r.Method != http.MethodPost {
				httpx.MethodNotAllowed(w, http.MethodPost)
				return
			}
			payOrder.ServeHTTP(w, r)
		default:
			if r.Method != http.MethodGet {
				httpx.MethodNotAllowed(w, http.MethodGet)
				return
			}
			getOrder.ServeHTTP(w, r)
		}
	})

	mux.HandleFunc("/webhooks/payments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.MethodNotAllowed(w, http.MethodPost)
			return
		}
		d.Payments.Webhook(w, r)
	})

	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.MethodNotAllowed(w, http.MethodGet)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
