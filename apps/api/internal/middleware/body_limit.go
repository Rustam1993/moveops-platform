package middleware

import (
	"net/http"
	"strings"
)

type BodyLimitOverride struct {
	PathPrefix string
	MaxBytes   int64
}

func LimitBodyBytes(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func LimitBodyBytesWithOverrides(defaultMax int64, overrides []BodyLimitOverride) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			maxBytes := defaultMax
			path := r.URL.Path
			apiPath := strings.TrimPrefix(path, "/api")
			for _, override := range overrides {
				if override.PathPrefix == "" || override.MaxBytes <= 0 {
					continue
				}
				if strings.HasPrefix(path, override.PathPrefix) || strings.HasPrefix(apiPath, override.PathPrefix) {
					maxBytes = override.MaxBytes
					break
				}
			}
			if maxBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
