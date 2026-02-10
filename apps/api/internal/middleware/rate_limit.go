package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type loginAttempt struct {
	count      int
	windowEnds time.Time
}

type LoginRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	attempt map[string]loginAttempt
}

func NewLoginRateLimiter(limit int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		limit:   limit,
		window:  window,
		attempt: map[string]loginAttempt{},
	}
}

func (rl *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r.RemoteAddr)
		if ip == "" {
			ip = "unknown"
		}

		now := time.Now()
		rl.mu.Lock()
		entry := rl.attempt[ip]
		if entry.windowEnds.Before(now) {
			entry = loginAttempt{count: 0, windowEnds: now.Add(rl.window)}
		}
		entry.count++
		rl.attempt[ip] = entry
		rl.mu.Unlock()

		if entry.count > rl.limit {
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", "Too many login attempts", nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
