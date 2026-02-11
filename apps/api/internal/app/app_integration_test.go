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

	router, err := NewRouter(cfg, gen.New(pool), logger)
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

func request(t *testing.T, router http.Handler, method, path string, body []byte, session *http.Cookie, csrf string) (int, []byte) {
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
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resBody, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, resBody
}
