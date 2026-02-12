package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	importRateLimiter := middleware.NewIPRateLimiter(8, time.Minute)
	exportRateLimiter := middleware.NewIPRateLimiter(30, time.Minute)

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

		protected.With(
			middleware.RequirePermission(q, "storage.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Post("/jobs/{jobId}/storage", func(w http.ResponseWriter, r *http.Request) {
			jobID, ok := parseUUIDParam(w, r, chi.URLParam(r, "jobId"), "invalid_job_id", "Job id must be a valid UUID")
			if !ok {
				return
			}
			h.PostJobsJobIdStorage(w, r, openapi_types.UUID(jobID))
		})

		protected.With(
			middleware.RequirePermission(q, "storage.read"),
		).Get("/storage", func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			facility := strings.TrimSpace(query.Get("facility"))
			if facility == "" {
				httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "facility query parameter is required", nil)
				return
			}

			params := oapi.GetStorageParams{Facility: facility}

			if qRaw := strings.TrimSpace(query.Get("q")); qRaw != "" {
				params.Q = &qRaw
			}

			if statusRaw := strings.TrimSpace(query.Get("status")); statusRaw != "" {
				status := oapi.StorageStatus(statusRaw)
				params.Status = &status
			}

			if boolRaw := strings.TrimSpace(query.Get("hasDateOut")); boolRaw != "" {
				parsed, err := strconv.ParseBool(boolRaw)
				if err != nil {
					httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "hasDateOut must be true or false", nil)
					return
				}
				params.HasDateOut = &parsed
			}
			if boolRaw := strings.TrimSpace(query.Get("balanceDue")); boolRaw != "" {
				parsed, err := strconv.ParseBool(boolRaw)
				if err != nil {
					httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "balanceDue must be true or false", nil)
					return
				}
				params.BalanceDue = &parsed
			}
			if boolRaw := strings.TrimSpace(query.Get("hasContainers")); boolRaw != "" {
				parsed, err := strconv.ParseBool(boolRaw)
				if err != nil {
					httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "hasContainers must be true or false", nil)
					return
				}
				params.HasContainers = &parsed
			}

			if pastDueRaw := strings.TrimSpace(query.Get("pastDueDays")); pastDueRaw != "" {
				parsed, err := strconv.Atoi(pastDueRaw)
				if err != nil || parsed < 0 {
					httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "pastDueDays must be a non-negative integer", nil)
					return
				}
				params.PastDueDays = &parsed
			}

			if limitRaw := strings.TrimSpace(query.Get("limit")); limitRaw != "" {
				parsed, err := strconv.Atoi(limitRaw)
				if err != nil || parsed < 1 {
					httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "limit must be a positive integer", nil)
					return
				}
				params.Limit = &parsed
			}

			if cursorRaw := strings.TrimSpace(query.Get("cursor")); cursorRaw != "" {
				params.Cursor = &cursorRaw
			}

			h.GetStorage(w, r, params)
		})

		protected.With(
			middleware.RequirePermission(q, "storage.read"),
		).Get("/storage/{storageRecordId}", func(w http.ResponseWriter, r *http.Request) {
			storageRecordID, ok := parseUUIDParam(w, r, chi.URLParam(r, "storageRecordId"), "invalid_storage_record_id", "Storage record id must be a valid UUID")
			if !ok {
				return
			}
			h.GetStorageStorageRecordId(w, r, openapi_types.UUID(storageRecordID))
		})

		protected.With(
			middleware.RequirePermission(q, "storage.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
		).Put("/storage/{storageRecordId}", func(w http.ResponseWriter, r *http.Request) {
			storageRecordID, ok := parseUUIDParam(w, r, chi.URLParam(r, "storageRecordId"), "invalid_storage_record_id", "Storage record id must be a valid UUID")
			if !ok {
				return
			}
			h.PutStorageStorageRecordId(w, r, openapi_types.UUID(storageRecordID))
		})

		protected.With(
			importRateLimiter.Middleware("Too many import requests"),
			middleware.RequirePermission(q, "imports.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
			middleware.LimitBodyBytes(cfg.ImportMaxFileBytes),
		).Post("/imports/dry-run", h.PostImportsDryRun)

		protected.With(
			importRateLimiter.Middleware("Too many import requests"),
			middleware.RequirePermission(q, "imports.write"),
			middleware.EnforceCSRF(cfg.CSRFEnforce),
			middleware.LimitBodyBytes(cfg.ImportMaxFileBytes),
		).Post("/imports/apply", h.PostImportsApply)

		protected.With(
			importRateLimiter.Middleware("Too many import requests"),
			middleware.RequirePermission(q, "imports.read"),
		).Get("/imports/{importRunId}", func(w http.ResponseWriter, r *http.Request) {
			importRunID, ok := parseUUIDParam(w, r, chi.URLParam(r, "importRunId"), "invalid_import_run_id", "Import run id must be a valid UUID")
			if !ok {
				return
			}
			h.GetImportsImportRunId(w, r, openapi_types.UUID(importRunID))
		})

		protected.With(
			importRateLimiter.Middleware("Too many import requests"),
			middleware.RequirePermission(q, "imports.read"),
		).Get("/imports/{importRunId}/errors.csv", func(w http.ResponseWriter, r *http.Request) {
			importRunID, ok := parseUUIDParam(w, r, chi.URLParam(r, "importRunId"), "invalid_import_run_id", "Import run id must be a valid UUID")
			if !ok {
				return
			}
			h.GetImportsImportRunIdErrorsCsv(w, r, openapi_types.UUID(importRunID))
		})

		protected.With(
			importRateLimiter.Middleware("Too many import requests"),
			middleware.RequirePermission(q, "imports.read"),
		).Get("/imports/{importRunId}/report.json", func(w http.ResponseWriter, r *http.Request) {
			importRunID, ok := parseUUIDParam(w, r, chi.URLParam(r, "importRunId"), "invalid_import_run_id", "Import run id must be a valid UUID")
			if !ok {
				return
			}
			h.GetImportsImportRunIdReportJson(w, r, openapi_types.UUID(importRunID))
		})

		protected.With(
			importRateLimiter.Middleware("Too many import requests"),
			middleware.RequireAnyPermission(q, "imports.read", "exports.read"),
		).Get("/imports/templates/{template}.csv", func(w http.ResponseWriter, r *http.Request) {
			template := oapi.ImportTemplate(strings.TrimSpace(chi.URLParam(r, "template")))
			h.GetImportsTemplatesTemplateCsv(w, r, template)
		})

		protected.With(
			exportRateLimiter.Middleware("Too many export requests"),
			middleware.RequirePermission(q, "exports.read"),
		).Get("/exports/customers.csv", h.GetExportsCustomersCsv)

		protected.With(
			exportRateLimiter.Middleware("Too many export requests"),
			middleware.RequirePermission(q, "exports.read"),
		).Get("/exports/estimates.csv", h.GetExportsEstimatesCsv)

		protected.With(
			exportRateLimiter.Middleware("Too many export requests"),
			middleware.RequirePermission(q, "exports.read"),
		).Get("/exports/jobs.csv", h.GetExportsJobsCsv)

		protected.With(
			exportRateLimiter.Middleware("Too many export requests"),
			middleware.RequirePermission(q, "exports.read"),
		).Get("/exports/storage.csv", h.GetExportsStorageCsv)
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
