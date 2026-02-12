package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moveops-platform/apps/api/internal/audit"
	"github.com/moveops-platform/apps/api/internal/auth"
	"github.com/moveops-platform/apps/api/internal/config"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/gen/oapi"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type Server struct {
	Config config.Config
	Q      *gen.Queries
	Audit  *audit.Logger
	Logger *slog.Logger
	DB     *pgxpool.Pool
}

func NewServer(cfg config.Config, q *gen.Queries, auditLogger *audit.Logger, logger *slog.Logger, db *pgxpool.Pool) *Server {
	return &Server{Config: cfg, Q: q, Audit: auditLogger, Logger: logger, DB: db}
}

func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) PostAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req oapi.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	users, err := s.Q.ListUsersByEmail(r.Context(), string(req.Email))
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load user", nil)
		return
	}

	var matched *gen.ListUsersByEmailRow
	for i := range users {
		user := users[i]
		if !user.IsActive {
			continue
		}
		ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
		if err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Password verification failed", nil)
			return
		}
		if ok {
			matched = &user
			break
		}
	}

	if matched == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password", nil)
		return
	}

	if old, err := r.Cookie(s.Config.SessionCookieName); err == nil && old.Value != "" {
		_, _ = s.Q.RevokeSessionByTokenHash(r.Context(), auth.HashToken(old.Value))
	}

	sessionToken, err := auth.GenerateToken()
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create session", nil)
		return
	}
	csrfToken, err := auth.GenerateToken()
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create CSRF token", nil)
		return
	}

	_, err = s.Q.CreateSession(r.Context(), gen.CreateSessionParams{
		TenantID:  matched.TenantID,
		UserID:    matched.ID,
		TokenHash: auth.HashToken(sessionToken),
		CsrfToken: csrfToken,
		ExpiresAt: time.Now().Add(s.Config.SessionTTL),
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save session", nil)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.Config.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.Config.SecureCookies,
		Expires:  time.Now().Add(s.Config.SessionTTL),
	})

	requestID := middleware.RequestIDFromContext(r.Context())
	userID := matched.ID
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   matched.TenantID,
		UserID:     &userID,
		Action:     "auth.login",
		EntityType: "session",
		RequestID:  requestID,
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.AuthSessionResponse{
		User: oapi.User{
			Id:       matched.ID,
			Email:    openapi_types.Email(matched.Email),
			FullName: matched.FullName,
		},
		Tenant: oapi.Tenant{
			Id:   matched.TenantID,
			Slug: matched.TenantSlug,
			Name: matched.TenantName,
		},
	})
}

func (s *Server) PostAuthLogout(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	sessionID, err := uuid.Parse(actor.SessionID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid session", nil)
		return
	}
	tenantID, err := uuid.Parse(actor.TenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid tenant", nil)
		return
	}

	if _, err := s.Q.RevokeSessionByID(r.Context(), gen.RevokeSessionByIDParams{ID: sessionID, TenantID: tenantID}); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to revoke session", nil)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.Config.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.Config.SecureCookies,
		MaxAge:   -1,
	})

	userID, _ := uuid.Parse(actor.UserID)
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "auth.logout",
		EntityType: "session",
		RequestID:  middleware.RequestIDFromContext(r.Context()),
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetAuthMe(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	userID, err := uuid.Parse(actor.UserID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid user", nil)
		return
	}
	tenantID, err := uuid.Parse(actor.TenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid tenant", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.AuthSessionResponse{
		User:   oapi.User{Id: userID, Email: openapi_types.Email(actor.Email), FullName: actor.FullName},
		Tenant: oapi.Tenant{Id: tenantID, Slug: actor.TenantSlug, Name: actor.TenantName},
	})
}

func (s *Server) GetAuthCsrf(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"csrfToken": actor.CSRFToken})
}

func (s *Server) PostCustomers(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	var req oapi.CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "firstName and lastName are required", nil)
		return
	}

	tenantID, err := uuid.Parse(actor.TenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid tenant", nil)
		return
	}
	userID, err := uuid.Parse(actor.UserID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid user", nil)
		return
	}

	var email *string
	if req.Email != nil {
		e := string(*req.Email)
		email = &e
	}

	customer, err := s.Q.CreateCustomer(r.Context(), gen.CreateCustomerParams{
		TenantID:  tenantID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     email,
		Phone:     req.Phone,
		CreatedBy: &userID,
		UpdatedBy: &userID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create customer", nil)
		return
	}

	customerID := customer.ID
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "customers.create",
		EntityType: "customer",
		EntityID:   &customerID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
	})

	httpx.WriteJSON(w, http.StatusCreated, mapCustomer(customer))
}

func (s *Server) GetCustomersCustomerId(w http.ResponseWriter, r *http.Request, customerId openapi_types.UUID) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	tenantID, err := uuid.Parse(actor.TenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid tenant", nil)
		return
	}

	customerUUID := uuid.UUID(customerId)
	customer, err := s.Q.GetCustomerByID(r.Context(), gen.GetCustomerByIDParams{ID: customerUUID, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "customer_not_found", "Customer was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load customer", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, mapCustomer(customer))
}

func mapCustomer(customer gen.Customer) oapi.Customer {
	var email *openapi_types.Email
	if customer.Email != nil {
		e := openapi_types.Email(*customer.Email)
		email = &e
	}

	return oapi.Customer{
		Id:        customer.ID,
		TenantId:  customer.TenantID,
		FirstName: customer.FirstName,
		LastName:  customer.LastName,
		Email:     email,
		Phone:     customer.Phone,
		CreatedAt: customer.CreatedAt.UTC(),
		UpdatedAt: customer.UpdatedAt.UTC(),
	}
}
