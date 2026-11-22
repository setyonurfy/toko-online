package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	handler "toko-online/internal/adapter/delivery/http"
	"toko-online/internal/adapter/repository/postgres"
	"toko-online/internal/infrastructure/auth"
	"toko-online/internal/infrastructure/config"
	"toko-online/internal/infrastructure/database"
	"toko-online/internal/infrastructure/logging"
	"toko-online/internal/infrastructure/payment"
	"toko-online/internal/infrastructure/worker"
	"toko-online/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("aplikasi berhenti karena galat", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("konfigurasi tidak valid: %w", err)
	}

	logger, err := logging.New(os.Stderr, cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return fmt.Errorf("konfigurasi log tidak valid: %w", err)
	}
	slog.SetDefault(logger)

	if len(cfg.JWTSecret) < 32 {
		return errors.New("JWT_SECRET wajib diisi, minimal 32 karakter")
	}
	if len(cfg.WebhookSecret) < 32 {
		return errors.New("PAYMENT_WEBHOOK_SECRET wajib diisi, minimal 32 karakter")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("gagal konek database: %w", err)
	}
	defer db.Close()
	slog.Info("database terhubung")

	authLimiter := handler.NewRateLimiter(10, 5) // 10 per menit, burst 5

	authLimiter.StartCleanup(ctx, 5*time.Minute, 15*time.Minute)

	userRepo := postgres.NewUserPostgres(db)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	authUsecase := usecase.NewAuthUsecase(userRepo, tokens)
	authHandler := handler.NewAuthHandler(authUsecase)

	productRepo := postgres.NewProductPostgres(db)
	productUsecase := usecase.NewProductUsecase(productRepo)
	productHandler := handler.NewProductHandler(productUsecase)

	orderRepo := postgres.NewOrderPostgres(db)
	orderUsecase := usecase.NewOrderUsecase(orderRepo)
	orderHandler := handler.NewOrderHandler(orderUsecase)

	paymentRepo := postgres.NewPaymentPostgres(db)
	gateway := payment.NewFakeGateway(cfg.WebhookSecret)
	paymentUsecase := usecase.NewPaymentUsecase(orderUsecase, paymentRepo, gateway)
	paymentHandler := handler.NewPaymentHandler(paymentUsecase)

	expiryUsecase := usecase.NewOrderExpiryUsecase(orderRepo, cfg.PaymentTTL, cfg.ExpiryBatch)
	expiryWorker, err := worker.NewExpiryWorker(expiryUsecase, cfg.ExpiryInterval)
	if err != nil {
		return fmt.Errorf("gagal membuat pekerja kedaluwarsa: %w", err)
	}

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

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("gagal membuka port %s: %w", cfg.Port, err)
	}

	var background sync.WaitGroup
	background.Add(1)
	go func() {
		defer background.Done()
		expiryWorker.Run(ctx)
	}()

	slog.Info("server berjalan", "addr", srv.Addr)
	err = serve(ctx, srv, ln, cfg.ShutdownTimeout)

	stop()
	background.Wait()
	slog.Info("server berhenti")
	return err
}

func serve(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server berhenti tak terduga: %w", err)
	case <-ctx.Done():
	}

	slog.Info("sinyal berhenti diterima, menunggu permintaan yang sedang berjalan", "timeout", shutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		srv.Close()
		return fmt.Errorf("shutdown server: %w", err)
	}

	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server berhenti tak terduga: %w", err)
	}
	return nil
}
