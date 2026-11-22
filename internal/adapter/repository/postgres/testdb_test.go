package postgres

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL tidak diisi; integration test dilewati")
	}

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL tidak valid: %v", err)
	}
	if !strings.HasSuffix(strings.TrimPrefix(u.Path, "/"), "_test") {
		t.Fatalf("menolak berjalan: nama database harus berakhiran _test, karena test ini menghapus seluruh data")
	}

	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatalf("buka database uji: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("ping database uji: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func resetDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`TRUNCATE payments, order_items, orders, products, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset database uji: %v", err)
	}
}

func seedUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(
		`INSERT INTO users (nama, email, password_hash) VALUES ('Pembeli Uji', $1, 'hash-uji') RETURNING id`,
		email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedProduct(t *testing.T, db *sql.DB, nama string, harga int64, stok int) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(
		`INSERT INTO products (nama, harga, stok) VALUES ($1, $2, $3) RETURNING id`,
		nama, harga, stok,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed produk: %v", err)
	}
	return id
}

// hitung menjalankan query yang menghasilkan satu angka.
func hitung(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("query hitung %q: %v", query, err)
	}
	return n
}
