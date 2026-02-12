package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moveops-platform/apps/api/internal/auth"
	"github.com/moveops-platform/apps/api/internal/config"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
)

func TestTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-a", "Tenant A", "a@example.com", "Password123!", []string{"customers.read", "customers.write"})
	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-b", "Tenant B", "b@example.com", "Password123!", []string{"customers.read"})

	cookieA := login(t, env.router, "a@example.com", "Password123!")
	csrfA := csrfToken(t, env.router, cookieA)
	customerID := createCustomer(t, env.router, cookieA, csrfA, "Ada", "Lovelace")

	cookieB := login(t, env.router, "b@example.com", "Password123!")
	status, _ := request(t, env.router, http.MethodGet, "/api/customers/"+customerID, nil, cookieB, "")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant read, got %d", status)
	}
}

func TestRBACDeniesRead(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-rbac", "Tenant RBAC", "admin@example.com", "Password123!", []string{"customers.read", "customers.write"})
	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-rbac-2", "Tenant RBAC2", "other@example.com", "Password123!", []string{"customers.read", "customers.write"})
	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-shared", "Tenant Shared", "writer@example.com", "Password123!", []string{"customers.write"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "readerless@example.com", "Password123!", []string{"customers.write"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "admin2@example.com", "Password123!", []string{"customers.read", "customers.write"})

	adminCookie := login(t, env.router, "admin2@example.com", "Password123!")
	adminCsrf := csrfToken(t, env.router, adminCookie)
	customerID := createCustomer(t, env.router, adminCookie, adminCsrf, "Grace", "Hopper")

	limitedCookie := login(t, env.router, "readerless@example.com", "Password123!")
	status, _ := request(t, env.router, http.MethodGet, "/api/customers/"+customerID, nil, limitedCookie, "")
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing customers.read, got %d", status)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-session", "Tenant Session", "session@example.com", "Password123!", []string{"customers.read", "customers.write"})

	cookie := login(t, env.router, "session@example.com", "Password123!")
	status, _ := request(t, env.router, http.MethodGet, "/api/auth/me", nil, cookie, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 before logout, got %d", status)
	}

	csrf := csrfToken(t, env.router, cookie)
	status, _ = request(t, env.router, http.MethodPost, "/api/auth/logout", nil, cookie, csrf)
	if status != http.StatusNoContent {
		t.Fatalf("expected 204 logout response, got %d", status)
	}

	status, _ = request(t, env.router, http.MethodGet, "/api/auth/me", nil, cookie, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", status)
	}
}

func TestEstimateTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-estimate-a", "Tenant Estimate A", "estimate-a@example.com", "Password123!", []string{"estimates.read", "estimates.write"})
	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-estimate-b", "Tenant Estimate B", "estimate-b@example.com", "Password123!", []string{"estimates.read"})

	cookieA := login(t, env.router, "estimate-a@example.com", "Password123!")
	csrfA := csrfToken(t, env.router, cookieA)
	estimateID := createEstimate(t, env.router, cookieA, csrfA, "idem-estimate-a")

	cookieB := login(t, env.router, "estimate-b@example.com", "Password123!")
	status, _ := request(t, env.router, http.MethodGet, "/api/estimates/"+estimateID, nil, cookieB, "")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant estimate read, got %d", status)
	}
}

func TestEstimateRBACForCreateAndConvert(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-rbac-est", "Tenant RBAC Est", "est-admin@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "est-reader@example.com", "Password123!", []string{"estimates.read"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "est-writer@example.com", "Password123!", []string{"estimates.read", "estimates.write"})

	readerCookie := login(t, env.router, "est-reader@example.com", "Password123!")
	readerCsrf := csrfToken(t, env.router, readerCookie)
	status, _ := request(t, env.router, http.MethodPost, "/api/estimates", estimatePayload("Reader Blocked"), readerCookie, readerCsrf, withIdempotency("idem-rbac-reader"))
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing estimates.write, got %d", status)
	}

	writerCookie := login(t, env.router, "est-writer@example.com", "Password123!")
	writerCsrf := csrfToken(t, env.router, writerCookie)
	estimateID := createEstimate(t, env.router, writerCookie, writerCsrf, "idem-rbac-writer")

	status, _ = request(t, env.router, http.MethodPost, "/api/estimates/"+estimateID+"/convert", nil, writerCookie, writerCsrf, withIdempotency("idem-rbac-convert"))
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing estimates.convert, got %d", status)
	}
}

func TestEstimateCreateIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-idem-create", "Tenant Idem Create", "idem-create@example.com", "Password123!", []string{"estimates.read", "estimates.write"})

	cookie := login(t, env.router, "idem-create@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)

	key := "idem-create-same"
	status1, body1 := request(t, env.router, http.MethodPost, "/api/estimates", estimatePayload("Alice Create"), cookie, csrf, withIdempotency(key))
	if status1 != http.StatusCreated {
		t.Fatalf("expected 201 create, got %d (%s)", status1, string(body1))
	}
	estimateID1 := parseEstimateID(t, body1)

	status2, body2 := request(t, env.router, http.MethodPost, "/api/estimates", estimatePayload("Alice Create"), cookie, csrf, withIdempotency(key))
	if status2 != http.StatusOK {
		t.Fatalf("expected 200 idempotent replay, got %d (%s)", status2, string(body2))
	}
	estimateID2 := parseEstimateID(t, body2)
	if estimateID1 != estimateID2 {
		t.Fatalf("expected same estimate id for idempotent replay, got %s and %s", estimateID1, estimateID2)
	}

	status3, body3 := request(t, env.router, http.MethodPost, "/api/estimates", estimatePayload("Different Payload"), cookie, csrf, withIdempotency(key))
	if status3 != http.StatusConflict {
		t.Fatalf("expected 409 for key reuse with different payload, got %d (%s)", status3, string(body3))
	}
}

func TestEstimateConvertIdempotencyAndSingleJob(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-idem-convert", "Tenant Idem Convert", "idem-convert@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert"})

	cookie := login(t, env.router, "idem-convert@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)
	estimateID := createEstimate(t, env.router, cookie, csrf, "idem-convert-estimate")

	key := "idem-convert-key"
	status1, body1 := request(t, env.router, http.MethodPost, "/api/estimates/"+estimateID+"/convert", nil, cookie, csrf, withIdempotency(key))
	if status1 != http.StatusCreated {
		t.Fatalf("expected 201 convert, got %d (%s)", status1, string(body1))
	}
	jobID1 := parseJobID(t, body1)

	status2, body2 := request(t, env.router, http.MethodPost, "/api/estimates/"+estimateID+"/convert", nil, cookie, csrf, withIdempotency(key))
	if status2 != http.StatusOK {
		t.Fatalf("expected 200 idempotent convert replay, got %d (%s)", status2, string(body2))
	}
	jobID2 := parseJobID(t, body2)
	if jobID1 != jobID2 {
		t.Fatalf("expected same job id for idempotent convert replay, got %s and %s", jobID1, jobID2)
	}

	status3, body3 := request(t, env.router, http.MethodPost, "/api/estimates/"+estimateID+"/convert", nil, cookie, csrf, withIdempotency("idem-convert-key-2"))
	if status3 != http.StatusOK {
		t.Fatalf("expected 200 for repeated convert with different key, got %d (%s)", status3, string(body3))
	}
	jobID3 := parseJobID(t, body3)
	if jobID1 != jobID3 {
		t.Fatalf("expected same job id for repeated convert, got %s and %s", jobID1, jobID3)
	}

	var count int
	estimateUUID, err := uuid.Parse(estimateID)
	if err != nil {
		t.Fatalf("parse estimate id: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE tenant_id = $1 AND estimate_id = $2`, tenantID, estimateUUID).Scan(&count); err != nil {
		t.Fatalf("count jobs by estimate: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 job for estimate, got %d", count)
	}
}

func TestCalendarTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-calendar-a", "Tenant Calendar A", "calendar-a@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "calendar.read", "calendar.write", "jobs.read"})
	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-calendar-b", "Tenant Calendar B", "calendar-b@example.com", "Password123!", []string{"calendar.read"})

	cookieA := login(t, env.router, "calendar-a@example.com", "Password123!")
	csrfA := csrfToken(t, env.router, cookieA)
	estimateID := createEstimate(t, env.router, cookieA, csrfA, "calendar-tenant-isolation")
	_ = convertEstimateToJob(t, env.router, cookieA, csrfA, estimateID, "calendar-tenant-isolation-convert")

	cookieB := login(t, env.router, "calendar-b@example.com", "Password123!")
	status, body := request(t, env.router, http.MethodGet, "/api/calendar?from=2026-03-01&to=2026-04-01", nil, cookieB, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 for calendar read, got %d (%s)", status, string(body))
	}

	jobs := parseCalendarJobs(t, body)
	if len(jobs) != 0 {
		t.Fatalf("expected cross-tenant calendar list to be empty, got %d jobs", len(jobs))
	}
}

func TestCalendarRBACReadDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-calendar-read", "Tenant Calendar Read", "calendar-read-admin@example.com", "Password123!", []string{"calendar.read"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "calendar-read-denied@example.com", "Password123!", []string{"jobs.read"})

	limitedCookie := login(t, env.router, "calendar-read-denied@example.com", "Password123!")
	status, _ := request(t, env.router, http.MethodGet, "/api/calendar?from=2026-03-01&to=2026-04-01", nil, limitedCookie, "")
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing calendar.read, got %d", status)
	}
}

func TestCalendarRBACWriteDeniedOnJobPatch(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-calendar-write", "Tenant Calendar Write", "calendar-write-admin@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "calendar.read", "calendar.write", "jobs.read"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "calendar-write-denied@example.com", "Password123!", []string{"jobs.read", "jobs.write"})

	adminCookie := login(t, env.router, "calendar-write-admin@example.com", "Password123!")
	adminCsrf := csrfToken(t, env.router, adminCookie)
	estimateID := createEstimate(t, env.router, adminCookie, adminCsrf, "calendar-write-denied-estimate")
	jobID := convertEstimateToJob(t, env.router, adminCookie, adminCsrf, estimateID, "calendar-write-denied-convert")

	limitedCookie := login(t, env.router, "calendar-write-denied@example.com", "Password123!")
	limitedCsrf := csrfToken(t, env.router, limitedCookie)
	payload, _ := json.Marshal(map[string]any{
		"scheduledDate": "2026-03-25",
	})
	status, _ := request(t, env.router, http.MethodPatch, "/api/jobs/"+jobID, payload, limitedCookie, limitedCsrf)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing calendar.write, got %d", status)
	}
}

func TestCalendarListIncludesJobInRange(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-calendar-range", "Tenant Calendar Range", "calendar-range@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "calendar.read", "calendar.write", "jobs.read"})

	cookie := login(t, env.router, "calendar-range@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)
	estimateID := createEstimate(t, env.router, cookie, csrf, "calendar-range-estimate")
	jobID := convertEstimateToJob(t, env.router, cookie, csrf, estimateID, "calendar-range-convert")

	status, body := request(t, env.router, http.MethodGet, "/api/calendar?from=2026-03-01&to=2026-04-01", nil, cookie, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 for calendar list, got %d (%s)", status, string(body))
	}
	jobs := parseCalendarJobs(t, body)
	if len(jobs) != 1 {
		t.Fatalf("expected exactly 1 job in range, got %d", len(jobs))
	}
	if jobs[0].JobID != jobID {
		t.Fatalf("expected job %s in calendar, got %s", jobID, jobs[0].JobID)
	}
	if jobs[0].ScheduledDate != "2026-03-20" {
		t.Fatalf("expected scheduledDate 2026-03-20, got %s", jobs[0].ScheduledDate)
	}
}

func TestCalendarScheduleUpdateCreatesAuditLog(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-calendar-audit", "Tenant Calendar Audit", "calendar-audit@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "calendar.read", "calendar.write", "jobs.read"})

	cookie := login(t, env.router, "calendar-audit@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)
	estimateID := createEstimate(t, env.router, cookie, csrf, "calendar-audit-estimate")
	jobID := convertEstimateToJob(t, env.router, cookie, csrf, estimateID, "calendar-audit-convert")

	payload, _ := json.Marshal(map[string]any{
		"scheduledDate": "2026-03-26",
		"pickupTime":    "13:45",
	})
	status, body := request(t, env.router, http.MethodPatch, "/api/jobs/"+jobID, payload, cookie, csrf)
	if status != http.StatusOK {
		t.Fatalf("expected 200 job patch, got %d (%s)", status, string(body))
	}

	jobUUID, err := uuid.Parse(jobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}

	var count int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM audit_log
		WHERE tenant_id = $1
		  AND entity_type = 'job'
		  AND entity_id = $2
		  AND action = 'job.schedule_update'
	`, tenantID, jobUUID).Scan(&count); err != nil {
		t.Fatalf("count job.schedule_update audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 job.schedule_update audit row, got %d", count)
	}
}

func TestStorageTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-storage-a", "Tenant Storage A", "storage-a@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "storage.read", "storage.write"})
	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-storage-b", "Tenant Storage B", "storage-b@example.com", "Password123!", []string{"storage.read"})

	cookieA := login(t, env.router, "storage-a@example.com", "Password123!")
	csrfA := csrfToken(t, env.router, cookieA)
	estimateID := createEstimate(t, env.router, cookieA, csrfA, "storage-tenant-isolation-estimate")
	jobID := convertEstimateToJob(t, env.router, cookieA, csrfA, estimateID, "storage-tenant-isolation-convert")
	storageID := createStorageRecord(t, env.router, cookieA, csrfA, jobID, "Main Facility")

	cookieB := login(t, env.router, "storage-b@example.com", "Password123!")
	status, body := request(t, env.router, http.MethodGet, "/api/storage?facility=Main%20Facility", nil, cookieB, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 for storage list, got %d (%s)", status, string(body))
	}
	rows := parseStorageListItems(t, body)
	if len(rows) != 0 {
		t.Fatalf("expected cross-tenant storage list to be empty, got %d rows", len(rows))
	}

	status, _ = request(t, env.router, http.MethodGet, "/api/storage/"+storageID, nil, cookieB, "")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant storage record read, got %d", status)
	}
}

func TestStorageRBACReadDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-storage-rbac-read", "Tenant Storage RBAC Read", "storage-rbac-read-admin@example.com", "Password123!", []string{"storage.read"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "storage-rbac-read-denied@example.com", "Password123!", []string{"jobs.read"})

	limitedCookie := login(t, env.router, "storage-rbac-read-denied@example.com", "Password123!")
	status, _ := request(t, env.router, http.MethodGet, "/api/storage?facility=Main%20Facility", nil, limitedCookie, "")
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing storage.read, got %d", status)
	}
}

func TestStorageRBACWriteDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-storage-rbac-write", "Tenant Storage RBAC Write", "storage-rbac-write-admin@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "storage.read", "storage.write"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "storage-rbac-write-denied@example.com", "Password123!", []string{"storage.read"})

	adminCookie := login(t, env.router, "storage-rbac-write-admin@example.com", "Password123!")
	adminCsrf := csrfToken(t, env.router, adminCookie)
	estimateID := createEstimate(t, env.router, adminCookie, adminCsrf, "storage-rbac-write-estimate")
	jobID := convertEstimateToJob(t, env.router, adminCookie, adminCsrf, estimateID, "storage-rbac-write-convert")
	storageID := createStorageRecord(t, env.router, adminCookie, adminCsrf, jobID, "Main Facility")

	limitedCookie := login(t, env.router, "storage-rbac-write-denied@example.com", "Password123!")
	limitedCsrf := csrfToken(t, env.router, limitedCookie)

	status, _ := request(t, env.router, http.MethodPost, "/api/jobs/"+jobID+"/storage", storageCreatePayload("Main Facility"), limitedCookie, limitedCsrf)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing storage.write on create, got %d", status)
	}

	status, _ = request(t, env.router, http.MethodPut, "/api/storage/"+storageID, storageUpdatePayload("Main Facility", 4, 2, 22, 3, 140, 45000, 5000, "Updated notes"), limitedCookie, limitedCsrf)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing storage.write on update, got %d", status)
	}
}

func TestStorageCreateAndUpdatePersists(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-storage-persist", "Tenant Storage Persist", "storage-persist@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "storage.read", "storage.write"})

	cookie := login(t, env.router, "storage-persist@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)
	estimateID := createEstimate(t, env.router, cookie, csrf, "storage-persist-estimate")
	jobID := convertEstimateToJob(t, env.router, cookie, csrf, estimateID, "storage-persist-convert")

	status, body := request(t, env.router, http.MethodPost, "/api/jobs/"+jobID+"/storage", storageCreatePayload("Main Facility"), cookie, csrf)
	if status != http.StatusCreated {
		t.Fatalf("expected 201 storage create, got %d (%s)", status, string(body))
	}
	storageID := parseStorageRecordID(t, body)

	status, body = request(t, env.router, http.MethodGet, "/api/storage/"+storageID, nil, cookie, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 storage read, got %d (%s)", status, string(body))
	}
	record := parseStorageRecord(t, body)
	if record.Facility != "Main Facility" {
		t.Fatalf("expected facility Main Facility, got %s", record.Facility)
	}
	if record.Status != "in_storage" {
		t.Fatalf("expected status in_storage, got %s", record.Status)
	}
	if record.Vaults != 2 || record.Pads != 1 || record.Items != 18 || record.OversizeItems != 2 {
		t.Fatalf("unexpected counts after create: %+v", record)
	}

	status, body = request(t, env.router, http.MethodPut, "/api/storage/"+storageID, storageUpdatePayload("Main Facility", 5, 3, 24, 4, 160, 52000, 9000, "Updated storage notes"), cookie, csrf)
	if status != http.StatusOK {
		t.Fatalf("expected 200 storage update, got %d (%s)", status, string(body))
	}

	status, body = request(t, env.router, http.MethodGet, "/api/storage/"+storageID, nil, cookie, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 storage read after update, got %d (%s)", status, string(body))
	}
	record = parseStorageRecord(t, body)
	if record.Vaults != 5 || record.Pads != 3 || record.Items != 24 || record.OversizeItems != 4 {
		t.Fatalf("unexpected counts after update: %+v", record)
	}
	if record.StorageBalanceCents != 52000 || record.MoveBalanceCents != 9000 {
		t.Fatalf("unexpected balances after update: %+v", record)
	}

	status, body = request(t, env.router, http.MethodGet, "/api/storage?facility=Main%20Facility&q="+record.JobNumber, nil, cookie, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 storage list after update, got %d (%s)", status, string(body))
	}
	rows := parseStorageListItems(t, body)
	if len(rows) != 1 {
		t.Fatalf("expected 1 storage list row, got %d", len(rows))
	}
	if rows[0].StorageRecordID != storageID {
		t.Fatalf("expected list row storageRecordId %s, got %s", storageID, rows[0].StorageRecordID)
	}
}

func TestStorageUpdateCreatesAuditLog(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-storage-audit", "Tenant Storage Audit", "storage-audit@example.com", "Password123!", []string{"estimates.read", "estimates.write", "estimates.convert", "storage.read", "storage.write"})

	cookie := login(t, env.router, "storage-audit@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)
	estimateID := createEstimate(t, env.router, cookie, csrf, "storage-audit-estimate")
	jobID := convertEstimateToJob(t, env.router, cookie, csrf, estimateID, "storage-audit-convert")
	storageID := createStorageRecord(t, env.router, cookie, csrf, jobID, "Main Facility")

	status, body := request(t, env.router, http.MethodPut, "/api/storage/"+storageID, storageUpdatePayload("Main Facility", 6, 4, 26, 5, 175, 61000, 12000, "Audit update"), cookie, csrf)
	if status != http.StatusOK {
		t.Fatalf("expected 200 storage update, got %d (%s)", status, string(body))
	}

	storageUUID, err := uuid.Parse(storageID)
	if err != nil {
		t.Fatalf("parse storage id: %v", err)
	}

	var count int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM audit_log
		WHERE tenant_id = $1
		  AND entity_type = 'storage_record'
		  AND entity_id = $2
		  AND action = 'storage_record.update'
	`, tenantID, storageUUID).Scan(&count); err != nil {
		t.Fatalf("count storage_record.update audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 storage_record.update audit row, got %d", count)
	}
}

func TestImportExportRBACDeniedWithoutPermissions(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-import-rbac", "Tenant Import RBAC", "import-rbac-admin@example.com", "Password123!", []string{"imports.write", "imports.read", "exports.read"})
	_, _ = seedUserInTenant(t, ctx, env.pool, tenantID, "import-rbac-limited@example.com", "Password123!", []string{"storage.read"})

	limitedCookie := login(t, env.router, "import-rbac-limited@example.com", "Password123!")
	limitedCsrf := csrfToken(t, env.router, limitedCookie)

	status, _ := multipartImportRequest(t, env.router, "/api/imports/dry-run", limitedCookie, limitedCsrf, "rbac.csv", validImportCSV("J-RBAC-001", "E-RBAC-001", "rbac@example.com"), importMapping())
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing imports.write, got %d", status)
	}

	status, _ = request(t, env.router, http.MethodGet, "/api/exports/customers.csv", nil, limitedCookie, "")
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for missing exports.read, got %d", status)
	}
}

func TestImportRunTenantIsolationAndExportTenantScope(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-import-a", "Tenant Import A", "import-a@example.com", "Password123!", []string{"imports.write", "imports.read", "exports.read"})
	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-import-b", "Tenant Import B", "import-b@example.com", "Password123!", []string{"imports.write", "imports.read", "exports.read"})

	cookieA := login(t, env.router, "import-a@example.com", "Password123!")
	csrfA := csrfToken(t, env.router, cookieA)
	status, body := multipartImportRequest(t, env.router, "/api/imports/apply", cookieA, csrfA, "tenant-a.csv", validImportCSV("J-TENANT-A-001", "E-TENANT-A-001", "tenant-a-customer@example.com"), importMapping())
	if status != http.StatusOK {
		t.Fatalf("tenant A apply import expected 200, got %d (%s)", status, string(body))
	}
	runA := parseImportRun(t, body)

	cookieB := login(t, env.router, "import-b@example.com", "Password123!")
	csrfB := csrfToken(t, env.router, cookieB)
	status, body = multipartImportRequest(t, env.router, "/api/imports/apply", cookieB, csrfB, "tenant-b.csv", validImportCSV("J-TENANT-B-001", "E-TENANT-B-001", "tenant-b-customer@example.com"), importMapping())
	if status != http.StatusOK {
		t.Fatalf("tenant B apply import expected 200, got %d (%s)", status, string(body))
	}

	status, _ = request(t, env.router, http.MethodGet, "/api/imports/"+runA.ImportRunID, nil, cookieB, "")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant import run fetch, got %d", status)
	}

	status, body = request(t, env.router, http.MethodGet, "/api/exports/jobs.csv", nil, cookieA, "")
	if status != http.StatusOK {
		t.Fatalf("tenant A jobs export expected 200, got %d (%s)", status, string(body))
	}
	csvBody := string(body)
	if !strings.Contains(csvBody, "J-TENANT-A-001") {
		t.Fatalf("expected tenant A export to include J-TENANT-A-001")
	}
	if strings.Contains(csvBody, "J-TENANT-B-001") {
		t.Fatalf("expected tenant A export to exclude tenant B job data")
	}
}

func TestImportApplyIsIdempotentAcrossRuns(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantID, _ := seedTenantUser(t, ctx, env.pool, "tenant-import-idem", "Tenant Import Idem", "import-idem@example.com", "Password123!", []string{"imports.write", "imports.read"})

	cookie := login(t, env.router, "import-idem@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)
	csvData := validImportCSV("J-IDEM-001", "E-IDEM-001", "idem-customer@example.com")

	status, body := multipartImportRequest(t, env.router, "/api/imports/apply", cookie, csrf, "idem.csv", csvData, importMapping())
	if status != http.StatusOK {
		t.Fatalf("first apply import expected 200, got %d (%s)", status, string(body))
	}

	status, body = multipartImportRequest(t, env.router, "/api/imports/apply", cookie, csrf, "idem.csv", csvData, importMapping())
	if status != http.StatusOK {
		t.Fatalf("second apply import expected 200, got %d (%s)", status, string(body))
	}
	secondRun := parseImportRun(t, body)
	if secondRun.Summary.Customer.Created != 0 || secondRun.Summary.Estimate.Created != 0 || secondRun.Summary.Job.Created != 0 || secondRun.Summary.StorageRecord.Created != 0 {
		t.Fatalf("expected second run created counts to be zero, got %+v", secondRun.Summary)
	}

	var customerCount, estimateCount, jobCount, storageCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM customers WHERE tenant_id = $1`, tenantID).Scan(&customerCount); err != nil {
		t.Fatalf("count customers: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM estimates WHERE tenant_id = $1`, tenantID).Scan(&estimateCount); err != nil {
		t.Fatalf("count estimates: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE tenant_id = $1`, tenantID).Scan(&jobCount); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM storage_record WHERE tenant_id = $1`, tenantID).Scan(&storageCount); err != nil {
		t.Fatalf("count storage records: %v", err)
	}

	if customerCount != 1 || estimateCount != 1 || jobCount != 1 || storageCount != 1 {
		t.Fatalf("expected entity counts to remain 1 after re-import, got customers=%d estimates=%d jobs=%d storage=%d", customerCount, estimateCount, jobCount, storageCount)
	}
}

func TestImportDryRunValidationProvidesErrorsCSV(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = seedTenantUser(t, ctx, env.pool, "tenant-import-errors", "Tenant Import Errors", "import-errors@example.com", "Password123!", []string{"imports.write", "imports.read"})

	cookie := login(t, env.router, "import-errors@example.com", "Password123!")
	csrf := csrfToken(t, env.router, cookie)

	status, body := multipartImportRequest(t, env.router, "/api/imports/dry-run", cookie, csrf, "errors.csv", invalidImportCSVMissingEstimateRequired(), invalidImportMapping())
	if status != http.StatusOK {
		t.Fatalf("dry-run expected 200, got %d (%s)", status, string(body))
	}
	run := parseImportRun(t, body)
	if run.Summary.RowsError == 0 {
		t.Fatalf("expected dry-run to report row errors")
	}

	status, body = request(t, env.router, http.MethodGet, "/api/imports/"+run.ImportRunID+"/errors.csv", nil, cookie, "")
	if status != http.StatusOK {
		t.Fatalf("errors.csv expected 200, got %d (%s)", status, string(body))
	}

	errorsCSV := string(body)
	if !strings.Contains(errorsCSV, "row_number,severity,entity_type,result,field,message") {
		t.Fatalf("expected errors.csv header row, got %q", errorsCSV)
	}
	if !strings.Contains(errorsCSV, "origin_zip, destination_zip, and requested_pickup_date are required") {
		t.Fatalf("expected validation error message in errors.csv, got %q", errorsCSV)
	}
}

type testEnv struct {
	pool   *pgxpool.Pool
	router http.Handler
}

func setupTestEnv(t *testing.T) testEnv {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	resetSchema(t, ctx, pool)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		Addr:               ":0",
		DatabaseURL:        databaseURL,
		SessionCookieName:  "mo_sess",
		SessionTTL:         12 * time.Hour,
		SecureCookies:      false,
		CSRFEnforce:        true,
		ImportMaxFileBytes: 15 * 1024 * 1024,
		ImportMaxRows:      5000,
		Env:                "test",
	}

	router, err := NewRouter(cfg, gen.New(pool), pool, logger)
	if err != nil {
		t.Fatalf("create router: %v", err)
	}

	return testEnv{pool: pool, router: router}
}

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("drop schema: %v", err)
	}

	schemaPath := filepath.Join("..", "..", "sql", "schema.sql")
	if _, err := os.Stat(schemaPath); err != nil {
		schemaPath = filepath.Join("sql", "schema.sql")
	}
	sqlBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
}

func seedTenantUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, slug, name, email, password string, permissions []string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var tenantID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO tenants (slug, name) VALUES ($1, $2) RETURNING id`, slug, name).Scan(&tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	userID, _ := seedUserInTenant(t, ctx, pool, tenantID, email, password, permissions)
	return tenantID, userID
}

func seedUserInTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, email, password string, permissions []string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, full_name, password_hash, is_active)
		VALUES ($1, $2, $3, $4, TRUE)
		RETURNING id
	`, tenantID, email, email, passwordHash).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var roleID uuid.UUID
	roleName := "role_" + userID.String()[:8]
	if err := pool.QueryRow(ctx, `
		INSERT INTO roles (tenant_id, name, description)
		VALUES ($1, $2, 'test role')
		RETURNING id
	`, tenantID, roleName).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id, tenant_id) VALUES ($1, $2, $3)`, userID, roleID, tenantID); err != nil {
		t.Fatalf("seed user role: %v", err)
	}

	for _, perm := range permissions {
		if _, err := pool.Exec(ctx, `INSERT INTO permissions (name, description) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, perm, "test"); err != nil {
			t.Fatalf("seed permission: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions WHERE name = $2
		`, roleID, perm); err != nil {
			t.Fatalf("seed role permission: %v", err)
		}
	}

	return userID, roleID
}

func login(t *testing.T, router http.Handler, email, password string) *http.Cookie {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"email": email, "password": password})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Result().Body)
		t.Fatalf("login expected 200, got %d with body: %s", rec.Code, string(body))
	}

	for _, c := range rec.Result().Cookies() {
		if c.Name == "mo_sess" {
			return c
		}
	}
	t.Fatal("session cookie not set")
	return nil
}

func csrfToken(t *testing.T, router http.Handler, session *http.Cookie) string {
	t.Helper()
	status, body := request(t, router, http.MethodGet, "/api/auth/csrf", nil, session, "")
	if status != http.StatusOK {
		t.Fatalf("csrf expected 200, got %d (%s)", status, string(body))
	}
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse csrf body: %v", err)
	}
	return payload["csrfToken"]
}

func createCustomer(t *testing.T, router http.Handler, session *http.Cookie, csrf, firstName, lastName string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"firstName": firstName, "lastName": lastName})
	status, body := request(t, router, http.MethodPost, "/api/customers", payload, session, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create customer expected 201, got %d (%s)", status, string(body))
	}
	var customer map[string]any
	if err := json.Unmarshal(body, &customer); err != nil {
		t.Fatalf("parse customer body: %v", err)
	}
	id, ok := customer["id"].(string)
	if !ok {
		t.Fatalf("customer id missing")
	}
	return id
}

func createEstimate(t *testing.T, router http.Handler, session *http.Cookie, csrf, idempotencyKey string) string {
	t.Helper()
	status, body := request(t, router, http.MethodPost, "/api/estimates", estimatePayload("Integration Customer"), session, csrf, withIdempotency(idempotencyKey))
	if status != http.StatusCreated {
		t.Fatalf("create estimate expected 201, got %d (%s)", status, string(body))
	}
	return parseEstimateID(t, body)
}

func estimatePayload(customerName string) []byte {
	payload, _ := json.Marshal(map[string]any{
		"customerName":            customerName,
		"primaryPhone":            "+1-555-0100",
		"email":                   "customer@example.com",
		"originAddressLine1":      "100 Origin St",
		"originCity":              "Austin",
		"originState":             "TX",
		"originPostalCode":        "78701",
		"destinationAddressLine1": "900 Destination Ave",
		"destinationCity":         "Dallas",
		"destinationState":        "TX",
		"destinationPostalCode":   "75001",
		"moveDate":                "2026-03-20",
		"leadSource":              "Website",
		"pickupTime":              "09:00",
	})
	return payload
}

func parseEstimateID(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Estimate struct {
			ID string `json:"id"`
		} `json:"estimate"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse estimate body: %v", err)
	}
	if payload.Estimate.ID == "" {
		t.Fatalf("estimate id missing")
	}
	return payload.Estimate.ID
}

func parseJobID(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse job body: %v", err)
	}
	if payload.Job.ID == "" {
		t.Fatalf("job id missing")
	}
	return payload.Job.ID
}

type calendarJobPayload struct {
	JobID         string `json:"jobId"`
	ScheduledDate string `json:"scheduledDate"`
}

func parseCalendarJobs(t *testing.T, body []byte) []calendarJobPayload {
	t.Helper()
	var payload struct {
		Jobs []calendarJobPayload `json:"jobs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse calendar body: %v", err)
	}
	return payload.Jobs
}

type storageRecordPayload struct {
	ID                  string `json:"id"`
	JobNumber           string `json:"jobNumber"`
	Facility            string `json:"facility"`
	Status              string `json:"status"`
	Vaults              int    `json:"vaults"`
	Pads                int    `json:"pads"`
	Items               int    `json:"items"`
	OversizeItems       int    `json:"oversizeItems"`
	StorageBalanceCents int64  `json:"storageBalanceCents"`
	MoveBalanceCents    int64  `json:"moveBalanceCents"`
}

type storageListItemPayload struct {
	StorageRecordID string `json:"storageRecordId"`
}

func parseStorageRecordID(t *testing.T, body []byte) string {
	t.Helper()
	record := parseStorageRecord(t, body)
	if record.ID == "" {
		t.Fatalf("storage id missing")
	}
	return record.ID
}

func parseStorageRecord(t *testing.T, body []byte) storageRecordPayload {
	t.Helper()
	var payload struct {
		Storage storageRecordPayload `json:"storage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse storage body: %v", err)
	}
	return payload.Storage
}

func parseStorageListItems(t *testing.T, body []byte) []storageListItemPayload {
	t.Helper()
	var payload struct {
		Items []storageListItemPayload `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse storage list body: %v", err)
	}
	return payload.Items
}

func createStorageRecord(t *testing.T, router http.Handler, session *http.Cookie, csrf, jobID, facility string) string {
	t.Helper()
	status, body := request(t, router, http.MethodPost, "/api/jobs/"+jobID+"/storage", storageCreatePayload(facility), session, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create storage record expected 201, got %d (%s)", status, string(body))
	}
	return parseStorageRecordID(t, body)
}

func storageCreatePayload(facility string) []byte {
	payload, _ := json.Marshal(map[string]any{
		"facility":            facility,
		"status":              "in_storage",
		"dateIn":              "2026-03-21",
		"nextBillDate":        "2026-04-21",
		"lotNumber":           "LOT-12",
		"locationLabel":       "Aisle 4",
		"vaults":              2,
		"pads":                1,
		"items":               18,
		"oversizeItems":       2,
		"volume":              120,
		"monthlyRateCents":    32900,
		"storageBalanceCents": 28000,
		"moveBalanceCents":    7000,
		"notes":               "Initial intake",
	})
	return payload
}

func storageUpdatePayload(facility string, vaults, pads, items, oversize, volume int, storageBalanceCents, moveBalanceCents int64, notes string) []byte {
	payload, _ := json.Marshal(map[string]any{
		"facility":            facility,
		"status":              "in_storage",
		"dateIn":              "2026-03-21",
		"nextBillDate":        "2026-05-21",
		"lotNumber":           "LOT-99",
		"locationLabel":       "Aisle 8",
		"vaults":              vaults,
		"pads":                pads,
		"items":               items,
		"oversizeItems":       oversize,
		"volume":              volume,
		"monthlyRateCents":    47900,
		"storageBalanceCents": storageBalanceCents,
		"moveBalanceCents":    moveBalanceCents,
		"notes":               notes,
	})
	return payload
}

func convertEstimateToJob(t *testing.T, router http.Handler, session *http.Cookie, csrf, estimateID, idempotencyKey string) string {
	t.Helper()
	status, body := request(t, router, http.MethodPost, "/api/estimates/"+estimateID+"/convert", nil, session, csrf, withIdempotency(idempotencyKey))
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("convert estimate expected 201/200, got %d (%s)", status, string(body))
	}
	return parseJobID(t, body)
}

func withIdempotency(key string) map[string]string {
	return map[string]string{"Idempotency-Key": key}
}

type importRunResponsePayload struct {
	ImportRunID string `json:"importRunId"`
	Summary     struct {
		RowsTotal int `json:"rowsTotal"`
		RowsValid int `json:"rowsValid"`
		RowsError int `json:"rowsError"`
		Customer  struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Error   int `json:"error"`
		} `json:"customer"`
		Estimate struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Error   int `json:"error"`
		} `json:"estimate"`
		Job struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Error   int `json:"error"`
		} `json:"job"`
		StorageRecord struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Error   int `json:"error"`
		} `json:"storageRecord"`
	} `json:"summary"`
}

func parseImportRun(t *testing.T, body []byte) importRunResponsePayload {
	t.Helper()
	var payload importRunResponsePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse import run response: %v", err)
	}
	if payload.ImportRunID == "" {
		t.Fatalf("importRunId missing from response")
	}
	return payload
}

func importMapping() map[string]any {
	return map[string]any{
		"job_number":            "job_number",
		"estimate_number":       "estimate_number",
		"customer_name":         "customer_name",
		"email":                 "email",
		"phone_primary":         "phone_primary",
		"origin_zip":            "origin_zip",
		"destination_zip":       "destination_zip",
		"requested_pickup_date": "requested_pickup_date",
		"requested_pickup_time": "requested_pickup_time",
		"scheduled_date":        "scheduled_date",
		"pickup_time":           "pickup_time",
		"status":                "status",
		"job_type":              "job_type",
		"lead_source":           "lead_source",
		"estimated_total":       "estimated_total",
		"deposit":               "deposit",
		"pricing_notes":         "pricing_notes",
		"facility":              "facility",
		"storage_status":        "storage_status",
		"date_in":               "date_in",
		"next_bill_date":        "next_bill_date",
		"vaults":                "vaults",
		"pads":                  "pads",
		"items":                 "items",
		"oversize_items":        "oversize_items",
		"volume":                "volume",
		"monthly_rate":          "monthly_rate",
		"storage_balance":       "storage_balance",
		"move_balance":          "move_balance",
	}
}

func invalidImportMapping() map[string]any {
	return map[string]any{
		"job_number":            "job_number",
		"estimate_number":       "estimate_number",
		"customer_name":         "customer_name",
		"email":                 "email",
		"phone_primary":         "phone_primary",
		"origin_zip":            "origin_zip",
		"destination_zip":       "destination_zip",
		"requested_pickup_date": "requested_pickup_date",
		"scheduled_date":        "scheduled_date",
		"status":                "status",
		"job_type":              "job_type",
	}
}

func validImportCSV(jobNumber, estimateNumber, email string) string {
	return strings.Join([]string{
		"job_number,estimate_number,customer_name,email,phone_primary,origin_zip,destination_zip,requested_pickup_date,requested_pickup_time,scheduled_date,pickup_time,status,job_type,lead_source,estimated_total,deposit,pricing_notes,facility,storage_status,date_in,next_bill_date,vaults,pads,items,oversize_items,volume,monthly_rate,storage_balance,move_balance",
		fmt.Sprintf("%s,%s,Import Customer,%s,5125550100,78701,75001,2026-03-22,09:00,2026-03-22,09:00,scheduled,local,Referral,250000,25000,Imported note,Main Facility,in_storage,2026-03-23,2026-04-23,2,1,18,2,120,32900,28000,7000", jobNumber, estimateNumber, email),
	}, "\n")
}

func invalidImportCSVMissingEstimateRequired() string {
	return strings.Join([]string{
		"job_number,estimate_number,customer_name,email,phone_primary,origin_zip,destination_zip,requested_pickup_date,scheduled_date,status,job_type",
		"J-ERR-001,E-ERR-001,Error Customer,error-customer@example.com,5125550100,,,,2026-03-22,scheduled,local",
	}, "\n")
}

func multipartImportRequest(t *testing.T, router http.Handler, path string, session *http.Cookie, csrf, filename, csvContent string, mapping map[string]any) (int, []byte) {
	t.Helper()

	options := map[string]any{
		"source":    "generic",
		"hasHeader": true,
		"mapping":   mapping,
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("marshal import options: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileHeader := textproto.MIMEHeader{}
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	fileHeader.Set("Content-Type", "text/csv")
	filePart, err := writer.CreatePart(fileHeader)
	if err != nil {
		t.Fatalf("create multipart file part: %v", err)
	}
	if _, err := io.Copy(filePart, strings.NewReader(csvContent)); err != nil {
		t.Fatalf("write multipart file body: %v", err)
	}
	optionsHeader := textproto.MIMEHeader{}
	optionsHeader.Set("Content-Disposition", `form-data; name="options"`)
	optionsHeader.Set("Content-Type", "application/json")
	optionsPart, err := writer.CreatePart(optionsHeader)
	if err != nil {
		t.Fatalf("create multipart options part: %v", err)
	}
	if _, err := optionsPart.Write(optionsJSON); err != nil {
		t.Fatalf("write multipart options body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, body)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if session != nil {
		req.AddCookie(session)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resBody, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, resBody
}

func request(t *testing.T, router http.Handler, method, path string, body []byte, session *http.Cookie, csrf string, extraHeaders ...map[string]string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if session != nil {
		req.AddCookie(session)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	for _, headers := range extraHeaders {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resBody, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, resBody
}
