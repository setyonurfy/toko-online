package main

import (
	"context"
	"log"
	"net/http"
	"time"
	"toko-online/internal/config"
	"toko-online/internal/database"
	handler "toko-online/internal/delivery/http"
	"toko-online/internal/repository"
	"toko-online/internal/usecase"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("gagal konek ke database: %v", err)
	}
	defer db.Close()
	log.Println("database terhubung")

	userRepo := repository.NewUserPostgres(db)
	authUsecase := usecase.NewAuthUsecase(userRepo)
	authHandler := handler.NewAuthHandler(authUsecase)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.NewRouter(authHandler),
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
