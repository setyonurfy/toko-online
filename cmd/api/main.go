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
	"toko-online/internal/repository"
	"toko-online/internal/usecase"
)

func main() {
	cfg := config.Load()
	if len(cfg.JWTSecret) < 32 {
		log.Fatalf("JWT_SECRET wajib diisi, minimal 32 karakter")
	}

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("gagal konek ke database: %v", err)
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

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.NewRouter(authHandler, tokens, authUsecase, authLimiter),
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
