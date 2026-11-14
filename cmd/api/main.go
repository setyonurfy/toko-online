package main

import (
	"context"
	"log"
	"net/http"
	"toko-online/internal/config"
	"toko-online/internal/database"
	"toko-online/internal/domain"
	"toko-online/internal/repository"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("gagal konek ke database: %v", err)
	}
	defer db.Close()
	log.Println("database terhubung")

	repo := repository.NewUserPostgres(db)
	u := &domain.User{Nama: "Jhondoe", Email: "jhondoe@mail.com", PasswordHash: "belum-di-hash"}
	if err := repo.Create(context.Background(), u); err != nil {
		log.Fatalf("gagal: %v", err)
	}
	log.Printf("user dibuat: id=%d", u.ID)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	addr := ":" + cfg.Port
	log.Printf("server berjalan di %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
