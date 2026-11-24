package middleware

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"toko-online/internal/adapter/delivery/http/httpx"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   float64
	now     func() time.Time
}

func NewRateLimiter(perMinute, burst int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
		now:     time.Now,
	}
}

func (rl *RateLimiter) Allow(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	b, ada := rl.buckets[key]
	if !ada {
		b = &bucket{tokens: rl.burst, lastSeen: now}
		rl.buckets[key] = b
	} else {
		b.tokens += now.Sub(b.lastSeen).Seconds() * rl.rate
		if b.tokens > rl.burst {
			b.tokens = rl.burst
		}
		b.lastSeen = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / rl.rate * float64(time.Second))
	return false, wait
}

func (rl *RateLimiter) Cleanup(maxIdle time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	batas := rl.now().Add(-maxIdle)
	for key, b := range rl.buckets {
		if b.lastSeen.Before(batas) {
			delete(rl.buckets, key)
		}
	}
}

func (rl *RateLimiter) StartCleanup(ctx context.Context, setiap, maxIdle time.Duration) {
	go func() {
		ticker := time.NewTicker(setiap)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rl.Cleanup(maxIdle)
			}
		}
	}()
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, tunggu := rl.Allow(httpx.ClientIP(r))
		if !ok {
			detik := int(math.Ceil(tunggu.Seconds()))
			slog.Warn("rate limit menolak permintaan", "ip", httpx.ClientIP(r), "path", r.URL.Path)
			w.Header().Set("Retry-After", strconv.Itoa(detik))
			httpx.WriteError(w, http.StatusTooManyRequests, "terlalu banyak percobaan, coba lagi nanti")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) MiddlewareByUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserIDFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
			return
		}
		diizinkan, tunggu := rl.Allow(strconv.FormatInt(id, 10))
		if !diizinkan {
			detik := int(math.Ceil(tunggu.Seconds()))
			slog.Warn("rate limit menolak permintaan pengguna", "user_id", id, "path", r.URL.Path)
			w.Header().Set("Retry-After", strconv.Itoa(detik))
			httpx.WriteError(w, http.StatusTooManyRequests, "terlalu banyak permintaan, coba lagi nanti")
			return
		}
		next.ServeHTTP(w, r)
	})
}
