package middleware

import (
	"net/http"

	"github.com/google/uuid"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
)

func RequirePermission(queries *gen.Queries, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, tenantID, ok := actorIDsFromRequest(w, r)
			if !ok {
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

func RequireAnyPermission(queries *gen.Queries, permissions ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, tenantID, ok := actorIDsFromRequest(w, r)
			if !ok {
				return
			}
			if len(permissions) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			for _, permission := range permissions {
				has, err := queries.UserHasPermission(r.Context(), gen.UserHasPermissionParams{
					UserID:     userID,
					TenantID:   tenantID,
					Permission: permission,
				})
				if err != nil {
					writeError(w, r, http.StatusInternalServerError, "internal_error", "Permission check failed", nil)
					return
				}
				if has {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeError(w, r, http.StatusForbidden, "forbidden", "Permission denied", map[string]any{"permissions": permissions})
		})
	}
}

func actorIDsFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actor, ok := ActorFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return uuid.Nil, uuid.Nil, false
	}

	userID, err := uuid.Parse(actor.UserID)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid actor", nil)
		return uuid.Nil, uuid.Nil, false
	}
	tenantID, err := uuid.Parse(actor.TenantID)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid actor", nil)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, tenantID, true
}
