package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		ctx := WithRequestID(r.Context(), requestID)
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
