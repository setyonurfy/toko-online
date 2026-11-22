package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"toko-online/internal/auth"
	"toko-online/internal/config"
	"toko-online/internal/database"
	handler "toko-online/internal/delivery/http"
	"toko-online/internal/logging"
	"toko-online/internal/payment"
	"toko-online/internal/repository"
	"toko-online/internal/usecase"
	"toko-online/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal("konfigurasi tidak valid", "error", err)
	}

	logger, err := logging.New(os.Stderr, cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		fatal("konfigurasi log tidak valid", "error", err)
	}
	slog.SetDefault(logger)

	if len(cfg.JWTSecret) < 32 {
		fatal("JWT_SECRET wajib diisi, minimal 32 karakter")
	}
	if len(cfg.WebhookSecret) < 32 {
		fatal("PAYMENT_WEBHOOK_SECRET wajib diisi, minimal 32 karakter")
	}

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fatal("gagal konek database", "error", err)
	}
	defer db.Close()
	slog.Info("database terhubung")

	authLimiter := handler.NewRateLimiter(10, 5) // 10 per menit, burst 5

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	authLimiter.StartCleanup(ctx, 5*time.Minute, 15*time.Minute)

	userRepo := repository.NewUserPostgres(db)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	authUsecase := usecase.NewAuthUsecase(userRepo, tokens)
	authHandler := handler.NewAuthHandler(authUsecase)

	productRepo := repository.NewProductPostgres(db)
	productUsecase := usecase.NewProductUsecase(productRepo)
	productHandler := handler.NewProductHandler(productUsecase)

	orderRepo := repository.NewOrderPostgres(db)
	orderUsecase := usecase.NewOrderUsecase(orderRepo)
	orderHandler := handler.NewOrderHandler(orderUsecase)

	paymentRepo := repository.NewPaymentPostgres(db)
	gateway := payment.NewFakeGateway(cfg.WebhookSecret)
	paymentUsecase := usecase.NewPaymentUsecase(orderUsecase, paymentRepo, gateway)
	paymentHandler := handler.NewPaymentHandler(paymentUsecase)

	expiryUsecase := usecase.NewOrderExpiryUsecase(orderRepo, cfg.PaymentTTL, cfg.ExpiryBatch)
	expiryWorker, err := worker.NewExpiryWorker(expiryUsecase, cfg.ExpiryInterval)
	if err != nil {
		fatal("gagal membuat pekerja kedaluwarsa", "error", err)
	}
	go expiryWorker.Run(ctx)

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: handler.NewRouter(handler.Dependencies{
			Auth:        authHandler,
			Products:    productHandler,
			Orders:      orderHandler,
			Payments:    paymentHandler,
			Tokens:      tokens,
			Users:       authUsecase,
			AuthLimiter: authLimiter,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	slog.Info("server berjalan", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		fatal("server berhenti", "error", err)
	}
}

func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
