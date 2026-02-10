package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	openapimiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/moveops-platform/apps/api/internal/audit"
	"github.com/moveops-platform/apps/api/internal/config"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/handlers"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
)

func NewRouter(cfg config.Config, q *gen.Queries, logger *slog.Logger) (http.Handler, error) {
	specPath := filepath.Join("openapi.yaml")
	if _, err := os.Stat(specPath); err != nil {
		return nil, fmt.Errorf("openapi spec not found at %s: %w", specPath, err)
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("load openapi spec: %w", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("validate openapi spec: %w", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.Logging(logger))

	api := chi.NewRouter()
	api.Use(openapimiddleware.OapiRequestValidatorWithOptions(doc, &openapimiddleware.Options{
		SilenceServersWarning: true,
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			requestID := w.Header().Get("X-Request-Id")
			httpx.WriteJSON(w, statusCode, httpx.ErrorEnvelope{
				Error:     httpx.ErrorBody{Code: "validation_error", Message: message},
				RequestID: requestID,
			})
		},
	}))

	auditLogger := audit.NewLogger(q)
	h := handlers.NewServer(cfg, q, auditLogger, logger)

	authMW := middleware.AuthMiddleware{Queries: q, CookieName: cfg.SessionCookieName}
	loginLimiter := middleware.NewLoginRateLimiter(10, time.Minute)

	api.Group(func(public chi.Router) {
		public.With(loginLimiter.Middleware).Post("/auth/login", h.PostAuthLogin)
		public.Get("/health", h.GetHealth)
	})

	api.Group(func(protected chi.Router) {
		protected.Use(authMW.RequireAuth)
		protected.Get("/auth/me", h.GetAuthMe)
		protected.Get("/auth/csrf", h.GetAuthCsrf)
		protected.With(middleware.EnforceCSRF(cfg.CSRFEnforce)).Post("/auth/logout", h.PostAuthLogout)

		protected.With(
			middleware.RequirePermission(q, "customers.read"),
		).Get("/customers/{customerId}", func(w http.ResponseWriter, r *http.Request) {
			h.GetCustomersCustomerId(w, r, chi.URLParam(r, "customerId"))
		})

		protected.With(
			middleware.RequirePermission(q, "customers.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Post("/customers", h.PostCustomers)
	})

	r.Mount("/api", api)
	return r, nil
}
