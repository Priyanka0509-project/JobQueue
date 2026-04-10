package ratelimiter

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens     int
	lastRefill time.Time
}

type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	maxTokens  int
	windowSize time.Duration
}

func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets:    make(map[string]*bucket),
		maxTokens:  maxRequests,
		windowSize: window,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[ip]
	if !exists {
		rl.buckets[ip] = &bucket{
			tokens:     rl.maxTokens,
			lastRefill: time.Now(),
		}
		b = rl.buckets[ip]
	}

	if time.Since(b.lastRefill) >= rl.windowSize {
		b.tokens = rl.maxTokens
		b.lastRefill = time.Now()
	}

	if b.tokens <= 0 {
		return false
	}

	b.tokens--
	return true
}

func (rl *RateLimiter) Remaining(ip string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[ip]
	if !ok {
		return rl.maxTokens
	}
	return b.tokens
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {

		rl.mu.Lock()
		for ip, b := range rl.buckets {
			if time.Since(b.lastRefill) > rl.windowSize*2 {
				delete(rl.buckets, ip)

			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Ratelimiter(next http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)

		if !rl.Allow(ip) {

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.maxTokens))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests) // 429
			w.Write([]byte(`{"error":"rate limit exceeded — max 2 uploads per minute"}`))
			return
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.maxTokens))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rl.Remaining(ip)))

		next(w, r)
	}
}

func getClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {

		return r.RemoteAddr
	}
	return ip
}
