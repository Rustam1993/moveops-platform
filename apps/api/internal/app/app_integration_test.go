package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		Addr:              ":0",
		DatabaseURL:       databaseURL,
		SessionCookieName: "mo_sess",
		SessionTTL:        12 * time.Hour,
		SecureCookies:     false,
		CSRFEnforce:       true,
		Env:               "test",
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

func withIdempotency(key string) map[string]string {
	return map[string]string{"Idempotency-Key": key}
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
