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
	"github.com/google/uuid"
	openapimiddleware "github.com/oapi-codegen/nethttp-middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moveops-platform/apps/api/internal/audit"
	"github.com/moveops-platform/apps/api/internal/config"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/gen/oapi"
	"github.com/moveops-platform/apps/api/internal/handlers"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
)

func NewRouter(cfg config.Config, q *gen.Queries, pool *pgxpool.Pool, logger *slog.Logger) (http.Handler, error) {
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
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))
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
	h := handlers.NewServer(cfg, q, auditLogger, logger, pool)

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
			customerID, ok := parseUUIDParam(w, r, chi.URLParam(r, "customerId"), "invalid_customer_id", "Customer id must be a valid UUID")
			if !ok {
				return
			}
			h.GetCustomersCustomerId(w, r, openapi_types.UUID(customerID))
		})

		protected.With(
			middleware.RequirePermission(q, "customers.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Post("/customers", h.PostCustomers)

		protected.With(
			middleware.RequirePermission(q, "estimates.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Post("/estimates", func(w http.ResponseWriter, r *http.Request) {
			h.PostEstimates(w, r, oapi.PostEstimatesParams{IdempotencyKey: r.Header.Get("Idempotency-Key")})
		})

		protected.With(
			middleware.RequirePermission(q, "estimates.read"),
		).Get("/estimates/{estimateId}", func(w http.ResponseWriter, r *http.Request) {
			estimateID, ok := parseUUIDParam(w, r, chi.URLParam(r, "estimateId"), "invalid_estimate_id", "Estimate id must be a valid UUID")
			if !ok {
				return
			}
			h.GetEstimatesEstimateId(w, r, openapi_types.UUID(estimateID))
		})

		protected.With(
			middleware.RequirePermission(q, "estimates.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Patch("/estimates/{estimateId}", func(w http.ResponseWriter, r *http.Request) {
			estimateID, ok := parseUUIDParam(w, r, chi.URLParam(r, "estimateId"), "invalid_estimate_id", "Estimate id must be a valid UUID")
			if !ok {
				return
			}
			h.PatchEstimatesEstimateId(w, r, openapi_types.UUID(estimateID))
		})

		protected.With(
			middleware.RequirePermission(q, "estimates.convert"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Post("/estimates/{estimateId}/convert", func(w http.ResponseWriter, r *http.Request) {
			estimateID, ok := parseUUIDParam(w, r, chi.URLParam(r, "estimateId"), "invalid_estimate_id", "Estimate id must be a valid UUID")
			if !ok {
				return
			}
			h.PostEstimatesEstimateIdConvert(w, r, openapi_types.UUID(estimateID), oapi.PostEstimatesEstimateIdConvertParams{IdempotencyKey: r.Header.Get("Idempotency-Key")})
		})

		protected.With(
			middleware.RequirePermission(q, "calendar.read"),
		).Get("/calendar", func(w http.ResponseWriter, r *http.Request) {
			fromDate, ok := parseDateQueryParam(w, r, "from")
			if !ok {
				return
			}
			toDate, ok := parseDateQueryParam(w, r, "to")
			if !ok {
				return
			}
			params := oapi.GetCalendarParams{
				From: fromDate,
				To:   toDate,
			}

			if phaseRaw := r.URL.Query().Get("phase"); phaseRaw != "" {
				phase := oapi.GetCalendarParamsPhase(phaseRaw)
				params.Phase = &phase
			}
			if jobTypeRaw := r.URL.Query().Get("jobType"); jobTypeRaw != "" {
				jobType := oapi.GetCalendarParamsJobType(jobTypeRaw)
				params.JobType = &jobType
			}
			if userIDRaw := r.URL.Query().Get("userId"); userIDRaw != "" {
				userID, ok := parseUUIDParam(w, r, userIDRaw, "invalid_user_id", "userId must be a valid UUID")
				if !ok {
					return
				}
				typed := openapi_types.UUID(userID)
				params.UserId = &typed
			}
			if departmentIDRaw := r.URL.Query().Get("departmentId"); departmentIDRaw != "" {
				departmentID, ok := parseUUIDParam(w, r, departmentIDRaw, "invalid_department_id", "departmentId must be a valid UUID")
				if !ok {
					return
				}
				typed := openapi_types.UUID(departmentID)
				params.DepartmentId = &typed
			}

			h.GetCalendar(w, r, params)
		})

		protected.With(
			middleware.RequirePermission(q, "jobs.read"),
		).Get("/jobs/{jobId}", func(w http.ResponseWriter, r *http.Request) {
			jobID, ok := parseUUIDParam(w, r, chi.URLParam(r, "jobId"), "invalid_job_id", "Job id must be a valid UUID")
			if !ok {
				return
			}
			h.GetJobsJobId(w, r, openapi_types.UUID(jobID))
		})

		protected.With(
			middleware.RequirePermission(q, "calendar.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Patch("/jobs/{jobId}", func(w http.ResponseWriter, r *http.Request) {
			jobID, ok := parseUUIDParam(w, r, chi.URLParam(r, "jobId"), "invalid_job_id", "Job id must be a valid UUID")
			if !ok {
				return
			}
			h.PatchJobsJobId(w, r, openapi_types.UUID(jobID))
		})
	})

	r.Mount("/api", api)
	return r, nil
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, raw, code, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, code, message, nil)
		return uuid.Nil, false
	}
	return id, true
}

func parseDateQueryParam(w http.ResponseWriter, r *http.Request, key string) (openapi_types.Date, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", key+" query parameter is required", nil)
		return openapi_types.Date{}, false
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", key+" must be in YYYY-MM-DD format", nil)
		return openapi_types.Date{}, false
	}
	return openapi_types.Date{Time: time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)}, true
}
