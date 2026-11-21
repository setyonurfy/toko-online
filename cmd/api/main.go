package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"toko-online/internal/auth"
	"toko-online/internal/config"
	"toko-online/internal/database"
	handler "toko-online/internal/delivery/http"
	"toko-online/internal/payment"
	"toko-online/internal/repository"
	"toko-online/internal/usecase"
	"toko-online/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("konfigurasi tidak valid: %v", err)
	}
	if len(cfg.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET wajib diisi, minimal 32 karakter")
	}
	if len(cfg.WebhookSecret) < 32 {
		log.Fatal("PAYMENT_WEBHOOK_SECRET wajib diisi, minimal 32 karakter")
	}

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("gagal konek database: %v", err)
	}
	defer db.Close()
	log.Println("database terhubung")

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
		log.Fatalf("pekerja kedaluwarsa: %v", err)
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

	log.Printf("server berjalan di %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
