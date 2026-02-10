package middleware

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/moveops-platform/apps/api/internal/auth"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
)

type AuthMiddleware struct {
	Queries    *gen.Queries
	CookieName string
}

func (m AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(m.CookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
			return
		}

		principal, err := m.Queries.GetSessionPrincipalByTokenHash(r.Context(), auth.HashToken(cookie.Value))
		if err != nil {
			if err == pgx.ErrNoRows {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "Session is invalid", nil)
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load session", nil)
			return
		}

		_ = m.Queries.TouchSession(r.Context(), principal.SessionID)

		ctx := WithActor(r.Context(), Actor{
			SessionID:  principal.SessionID.String(),
			UserID:     principal.UserID.String(),
			TenantID:   principal.TenantID.String(),
			Email:      principal.Email,
			FullName:   principal.FullName,
			TenantSlug: principal.TenantSlug,
			TenantName: principal.TenantName,
			CSRFToken:  principal.CsrfToken,
			ExpiresAt:  principal.ExpiresAt,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
