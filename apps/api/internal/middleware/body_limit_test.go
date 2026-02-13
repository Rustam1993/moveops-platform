package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitBodyBytesWithOverridesMatchesAPIPrefix(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := LimitBodyBytesWithOverrides(2, []BodyLimitOverride{
		{PathPrefix: "/imports/dry-run", MaxBytes: 10},
	})
	router := mw(handler)

	t.Run("override applies on /api path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/imports/dry-run", strings.NewReader("12345"))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("default limit applies elsewhere", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/customers", strings.NewReader("12345"))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
		}
	})
}
