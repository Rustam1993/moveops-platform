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
	mu          sync.Mutex
	limit       int
	window      time.Duration
	maxEntries  int
	attempt     map[string]loginAttempt
	lastCleanup time.Time
}

func NewLoginRateLimiter(limit int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{inner: newIPRateLimiter(limit, window, 10000)}
}

func NewLoginRateLimiterWithMaxEntries(limit int, window time.Duration, maxEntries int) *LoginRateLimiter {
	return &LoginRateLimiter{inner: newIPRateLimiter(limit, window, maxEntries)}
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{inner: newIPRateLimiter(limit, window, 10000)}
}

func NewIPRateLimiterWithMaxEntries(limit int, window time.Duration, maxEntries int) *IPRateLimiter {
	return &IPRateLimiter{inner: newIPRateLimiter(limit, window, maxEntries)}
}

func newIPRateLimiter(limit int, window time.Duration, maxEntries int) *ipRateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &ipRateLimiter{
		limit:      limit,
		window:     window,
		maxEntries: maxEntries,
		attempt:    map[string]loginAttempt{},
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
		rl.cleanupExpired(now)
		entry := rl.attempt[ip]
		if entry.windowEnds.Before(now) {
			entry = loginAttempt{count: 0, windowEnds: now.Add(rl.window)}
		}
		entry.count++
		rl.attempt[ip] = entry
		rl.enforceBound()
		rl.mu.Unlock()

		if entry.count > rl.limit {
			writeError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", message, nil)
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

func (rl *ipRateLimiter) cleanupExpired(now time.Time) {
	if rl.lastCleanup.Add(time.Second).After(now) {
		return
	}
	for ip, entry := range rl.attempt {
		if entry.windowEnds.Before(now) {
			delete(rl.attempt, ip)
		}
	}
	rl.lastCleanup = now
}

func (rl *ipRateLimiter) enforceBound() {
	for len(rl.attempt) > rl.maxEntries {
		var (
			oldestIP   string
			oldestTime time.Time
			set        bool
		)
		for ip, entry := range rl.attempt {
			if !set || entry.windowEnds.Before(oldestTime) {
				oldestIP = ip
				oldestTime = entry.windowEnds
				set = true
			}
		}
		if !set {
			return
		}
		delete(rl.attempt, oldestIP)
	}
}
