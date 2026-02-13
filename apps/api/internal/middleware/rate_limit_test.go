package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIPRateLimiterReturnsRateLimitedEnvelope(t *testing.T) {
	limiter := NewIPRateLimiterWithMaxEntries(1, time.Minute, 32)
	handler := limiter.Middleware("Too many requests")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/storage?facility=Main", nil)
	req1.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(first, req1)
	if first.Code != http.StatusOK {
		t.Fatalf("expected first request status 200, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/storage?facility=Main", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(second, req2)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d", second.Code)
	}
	body := second.Body.String()
	if !strings.Contains(body, `"code":"RATE_LIMITED"`) {
		t.Fatalf("expected RATE_LIMITED error code in response body, got %s", body)
	}
}
