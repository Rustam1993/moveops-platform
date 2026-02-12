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
	inner *ipRateLimiter
}

type IPRateLimiter struct {
	inner *ipRateLimiter
}

type ipRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	attempt map[string]loginAttempt
}

func NewLoginRateLimiter(limit int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{inner: newIPRateLimiter(limit, window)}
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{inner: newIPRateLimiter(limit, window)}
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &ipRateLimiter{
		limit:   limit,
		window:  window,
		attempt: map[string]loginAttempt{},
	}
}

func (rl *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return rl.inner.middleware("Too many login attempts", next)
}

func (rl *IPRateLimiter) Middleware(message string) func(http.Handler) http.Handler {
	if message == "" {
		message = "Rate limit exceeded"
	}
	return func(next http.Handler) http.Handler {
		return rl.inner.middleware(message, next)
	}
}

func (rl *ipRateLimiter) middleware(message string, next http.Handler) http.Handler {
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
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", message, nil)
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
