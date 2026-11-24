package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	sekarang := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(60, 3) // 1 tiket/detik, kapasitas 3
	rl.now = func() time.Time { return sekarang }

	for i := 1; i <= 3; i++ {
		if ok, _ := rl.Allow("1.2.3.4"); !ok {
			t.Fatalf("permintaan ke-%d seharusnya diizinkan", i)
		}
	}

	ok, tunggu := rl.Allow("1.2.3.4")
	if ok {
		t.Fatal("permintaan ke-4 seharusnya ditolak")
	}
	if tunggu != time.Second {
		t.Errorf("tunggu = %v, ingin 1s", tunggu)
	}

	if ok, _ := rl.Allow("5.6.7.8"); !ok {
		t.Error("IP lain tidak boleh terpengaruh")
	}

	sekarang = sekarang.Add(2 * time.Second) // 2 tiket terisi
	for i := 1; i <= 2; i++ {
		if ok, _ := rl.Allow("1.2.3.4"); !ok {
			t.Fatalf("setelah 2 detik, permintaan ke-%d seharusnya diizinkan", i)
		}
	}
	if ok, _ := rl.Allow("1.2.3.4"); ok {
		t.Error("tiket seharusnya habis lagi")
	}
}

func TestRateLimiter_TidakMelebihiKapasitas(t *testing.T) {
	sekarang := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(60, 3)
	rl.now = func() time.Time { return sekarang }

	rl.Allow("a")
	sekarang = sekarang.Add(24 * time.Hour) // menunggu lama tidak boleh menimbun tiket

	diizinkan := 0
	for i := 0; i < 10; i++ {
		if ok, _ := rl.Allow("a"); ok {
			diizinkan++
		}
	}
	if diizinkan != 3 {
		t.Errorf("diizinkan = %d, ingin 3 (sesuai kapasitas)", diizinkan)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	sekarang := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(60, 3)
	rl.now = func() time.Time { return sekarang }

	rl.Allow("lama")
	sekarang = sekarang.Add(2 * time.Hour)
	rl.Allow("baru")

	rl.Cleanup(time.Hour)

	if _, ada := rl.buckets["lama"]; ada {
		t.Error("ember 'lama' seharusnya sudah dibuang")
	}
	if _, ada := rl.buckets["baru"]; !ada {
		t.Error("ember 'baru' seharusnya masih ada")
	}
}

func TestRateLimiter_Konkuren(t *testing.T) {
	tetap := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(1, 50) // waktu dibekukan, jadi tidak ada pengisian ulang
	rl.now = func() time.Time { return tetap }

	var wg sync.WaitGroup
	var diizinkan atomic.Int64

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := rl.Allow("x"); ok {
				diizinkan.Add(1)
			}
		}()
	}
	wg.Wait()

	if diizinkan.Load() != 50 {
		t.Errorf("diizinkan = %d, ingin tepat 50", diizinkan.Load())
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	sekarang := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(60, 2)
	rl.now = func() time.Time { return sekarang }

	dipanggil := 0
	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dipanggil++
		w.WriteHeader(http.StatusOK)
	}))

	kirim := func(remoteAddr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// port berbeda, IP sama: harus berbagi ember
	if rec := kirim("1.2.3.4:5555"); rec.Code != http.StatusOK {
		t.Fatalf("ke-1: status = %d, ingin 200", rec.Code)
	}
	if rec := kirim("1.2.3.4:6666"); rec.Code != http.StatusOK {
		t.Fatalf("ke-2: status = %d, ingin 200", rec.Code)
	}

	rec := kirim("1.2.3.4:7777")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ke-3: status = %d, ingin 429", rec.Code)
	}
	if got := rec.Result().Header.Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, ingin 1", got)
	}
	if dipanggil != 2 {
		t.Errorf("handler dipanggil %d kali, ingin 2 (yang ditolak tidak boleh sampai ke handler)", dipanggil)
	}

	if rec := kirim("5.6.7.8:1111"); rec.Code != http.StatusOK {
		t.Errorf("IP lain: status = %d, ingin 200", rec.Code)
	}
}

func TestRateLimiter_MiddlewareByUser(t *testing.T) {
	sekarang := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(60, 2)
	rl.now = func() time.Time { return sekarang }

	dipanggil := 0
	h := rl.MiddlewareByUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dipanggil++
		w.WriteHeader(http.StatusOK)
	}))

	kirim := func(userID int64, remoteAddr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/orders", nil)
		req.RemoteAddr = remoteAddr
		req = req.WithContext(WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := kirim(7, "1.1.1.1:1000"); rec.Code != http.StatusOK {
		t.Fatalf("ke-1: status = %d, ingin 200", rec.Code)
	}
	if rec := kirim(7, "2.2.2.2:2000"); rec.Code != http.StatusOK {
		t.Fatalf("ke-2 (IP lain, pengguna sama): status = %d, ingin 200", rec.Code)
	}

	rec := kirim(7, "3.3.3.3:3000")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ke-3: status = %d, ingin 429 (ember dihitung per pengguna, bukan per IP)", rec.Code)
	}
	if got := rec.Result().Header.Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, ingin 1", got)
	}
	if dipanggil != 2 {
		t.Errorf("handler dipanggil %d kali, ingin 2", dipanggil)
	}

	if rec := kirim(8, "1.1.1.1:1000"); rec.Code != http.StatusOK {
		t.Errorf("pengguna lain, IP sama: status = %d, ingin 200", rec.Code)
	}
}

func TestRateLimiter_MiddlewareByUser_TanpaPenggunaDitolak(t *testing.T) {
	rl := NewRateLimiter(60, 2)
	dipanggil := false
	h := rl.MiddlewareByUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dipanggil = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, ingin 401", rec.Code)
	}
	if dipanggil {
		t.Error("handler tidak boleh dipanggil tanpa pengguna di context")
	}
}
