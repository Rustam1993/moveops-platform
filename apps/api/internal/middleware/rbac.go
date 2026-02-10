package middleware

import (
	"net/http"

	"github.com/google/uuid"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
)

func RequirePermission(queries *gen.Queries, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ActorFromContext(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
				return
			}

			userID, err := uuid.Parse(actor.UserID)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid actor", nil)
				return
			}
			tenantID, err := uuid.Parse(actor.TenantID)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid actor", nil)
				return
			}

			has, err := queries.UserHasPermission(r.Context(), gen.UserHasPermissionParams{
				UserID:     userID,
				TenantID:   tenantID,
				Permission: permission,
			})
			if err != nil {
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Permission check failed", nil)
				return
			}
			if !has {
				writeError(w, r, http.StatusForbidden, "forbidden", "Permission denied", map[string]string{"permission": permission})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
