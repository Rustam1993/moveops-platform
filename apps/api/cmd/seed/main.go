package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/moveops-platform/apps/api/internal/auth"
)

func main() {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	email := envOrDefault("SEED_ADMIN_EMAIL", "admin@local.moveops")
	password := envOrDefault("SEED_ADMIN_PASSWORD", "Admin12345!")
	fullName := envOrDefault("SEED_ADMIN_NAME", "Local Admin")
	tenantSlug := envOrDefault("SEED_TENANT_SLUG", "local-dev")
	tenantName := envOrDefault("SEED_TENANT_NAME", "Local Dev Tenant")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	var tenantID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO tenants (slug, name)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, tenantSlug, tenantName).Scan(&tenantID); err != nil {
		log.Fatalf("upsert tenant: %v", err)
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO users (tenant_id, email, full_name, password_hash, is_active)
		VALUES ($1, $2, $3, $4, TRUE)
		ON CONFLICT DO NOTHING
	`, tenantID, email, fullName, passwordHash)
	if err != nil {
		log.Fatalf("insert user: %v", err)
	}

	var userID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT id FROM users WHERE tenant_id = $1 AND lower(email) = lower($2)
	`, tenantID, email).Scan(&userID); err != nil {
		log.Fatalf("find user: %v", err)
	}

	permissionDescriptions := map[string]string{
		"customers.read":    "Read customer records",
		"customers.write":   "Create and update customer records",
		"estimates.read":    "Read estimate records",
		"estimates.write":   "Create and update estimate records",
		"estimates.convert": "Convert estimates into jobs",
		"calendar.read":     "Read calendar and schedule views",
		"calendar.write":    "Update calendar schedule and phase values",
		"jobs.read":         "Read job records",
		"jobs.write":        "Update job scheduling and status",
	}

	for perm, description := range permissionDescriptions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permissions (name, description)
			VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description
		`, perm, description); err != nil {
			log.Fatalf("insert permission: %v", err)
		}
	}

	roles := map[string]struct {
		description string
		permissions []string
	}{
		"admin": {
			description: "Tenant administrator",
			permissions: []string{"customers.read", "customers.write", "estimates.read", "estimates.write", "estimates.convert", "calendar.read", "calendar.write", "jobs.read", "jobs.write"},
		},
		"sales": {
			description: "Sales role",
			permissions: []string{"estimates.read", "estimates.write", "estimates.convert", "calendar.read", "jobs.read"},
		},
		"ops": {
			description: "Operations role",
			permissions: []string{"estimates.read", "calendar.read", "calendar.write", "jobs.read", "jobs.write"},
		},
	}

	roleIDs := make(map[string]uuid.UUID, len(roles))
	for roleName, role := range roles {
		var roleID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO roles (tenant_id, name, description)
			VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, name) DO UPDATE SET description = EXCLUDED.description
			RETURNING id
		`, tenantID, roleName, role.description).Scan(&roleID); err != nil {
			log.Fatalf("upsert role %s: %v", roleName, err)
		}
		roleIDs[roleName] = roleID

		for _, perm := range role.permissions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO role_permissions (role_id, permission_id)
				SELECT $1, p.id FROM permissions p WHERE p.name = $2
				ON CONFLICT DO NOTHING
			`, roleID, perm); err != nil {
				log.Fatalf("insert role permission: %v", err)
			}
		}
	}

	adminRoleID := roleIDs["admin"]
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, tenant_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, userID, adminRoleID, tenantID); err != nil {
		log.Fatalf("insert user role: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit tx: %v", err)
	}

	fmt.Printf("Seed completed. Tenant=%s, admin=%s, password=%s\n", tenantSlug, email, password)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
