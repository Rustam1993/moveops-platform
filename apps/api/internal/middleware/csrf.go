package middleware

import (
	"net/http"
	"strings"
)

func EnforceCSRF(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}
			actor, ok := ActorFromContext(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
				return
			}
			token := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
			if token == "" || token != actor.CSRFToken {
				writeError(w, r, http.StatusForbidden, "CSRF_INVALID", "Invalid CSRF token", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
