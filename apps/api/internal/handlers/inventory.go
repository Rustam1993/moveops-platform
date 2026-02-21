package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/smtp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/moveops-platform/apps/api/internal/audit"
	"github.com/moveops-platform/apps/api/internal/auth"
	"github.com/moveops-platform/apps/api/internal/config"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	maxInventoryItems = 500
)

type inventoryItemPayload struct {
	Category string  `json:"category"`
	ItemName string  `json:"itemName"`
	VolumeCf float64 `json:"volumeCf"`
	Qty      int     `json:"qty"`
	IsCustom bool    `json:"isCustom,omitempty"`
}

type replaceInventoryRequest struct {
	Items []inventoryItemPayload `json:"items"`
}

type estimateInventoryResponse struct {
	EstimateID    uuid.UUID              `json:"estimateId"`
	Items         []inventoryItemPayload `json:"items"`
	TotalVolumeCf float64                `json:"totalVolumeCf"`
	RequestID     string                 `json:"requestId"`
}

type createInventoryShareLinkRequest struct {
	ExpiresInDays *int `json:"expiresInDays,omitempty"`
}

type createInventoryShareLinkResponse struct {
	ShareLinkID    uuid.UUID `json:"shareLinkId"`
	EstimateID     uuid.UUID `json:"estimateId"`
	RecipientEmail string    `json:"recipientEmail"`
	ShareURL       string    `json:"shareUrl"`
	ExpiresAt      time.Time `json:"expiresAt"`
	DeliveryMode   string    `json:"deliveryMode"`
	RequestID      string    `json:"requestId"`
}

type publicInventoryResponse struct {
	EstimateID    uuid.UUID              `json:"estimateId"`
	CustomerName  string                 `json:"customerName"`
	MoveDate      openapi_types.Date     `json:"moveDate"`
	Items         []inventoryItemPayload `json:"items"`
	TotalVolumeCf float64                `json:"totalVolumeCf"`
	ExpiresAt     time.Time              `json:"expiresAt"`
	RequestID     string                 `json:"requestId"`
}

func (s *Server) GetEstimatesEstimateIdInventory(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{
		ID:       estimateID,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	items, err := s.Q.GetEstimateInventoryItems(r.Context(), gen.GetEstimateInventoryItemsParams{
		TenantID:   tenantID,
		EstimateID: estimateID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load inventory", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, estimateInventoryResponse{
		EstimateID:    estimateID,
		Items:         mapInventoryItems(items),
		TotalVolumeCf: roundCF(estimate.TotalVolumeCf),
		RequestID:     middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PutEstimatesEstimateIdInventory(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req replaceInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	items, totalCF, err := normalizeInventoryItems(req.Items)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	estimate, persistedItems, err := s.replaceEstimateInventory(r.Context(), tenantID, estimateID, &userID, items)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
		default:
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update inventory", nil)
		}
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "inventory.updated",
		EntityType: "estimate",
		EntityID:   &estimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"via":           "internal",
			"itemCount":     len(items),
			"totalVolumeCf": totalCF,
		},
	})
	s.trackAnalyticsEvent(r.Context(), tenantID, &userID, &estimateID, "estimate.inventory_updated", map[string]any{
		"via":        "internal",
		"item_count": len(items),
		"total_cf":   totalCF,
	})

	httpx.WriteJSON(w, http.StatusOK, estimateInventoryResponse{
		EstimateID:    estimateID,
		Items:         mapInventoryItems(persistedItems),
		TotalVolumeCf: roundCF(estimate.TotalVolumeCf),
		RequestID:     middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostEstimatesEstimateIdInventoryShareLinks(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	req := createInventoryShareLinkRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
			return
		}
	}

	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{
		ID:       estimateID,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	recipientEmail := strings.TrimSpace(estimate.Email)
	if recipientEmail == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "missing_customer_email", "Estimate has no customer email", nil)
		return
	}

	expiresAt := time.Now().UTC().Add(s.Config.InventoryShareTTL)
	if req.ExpiresInDays != nil {
		days := *req.ExpiresInDays
		if days < 1 || days > 30 {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "expiresInDays must be between 1 and 30", nil)
			return
		}
		expiresAt = time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	}

	token, err := auth.GenerateToken()
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create share token", nil)
		return
	}
	tokenHash := auth.HashToken(token)

	publicBaseURL := strings.TrimRight(strings.TrimSpace(s.Config.PublicWebBaseURL), "/")
	if publicBaseURL == "" {
		publicBaseURL = "http://localhost:3000"
	}
	shareURL := fmt.Sprintf("%s/public/inventory/%s", publicBaseURL, token)

	deliveryMode, deliveryErr := s.sendInventoryShareEmail(r, recipientEmail, shareURL)
	var deliveryErrMsg *string
	if deliveryErr != nil {
		msg := deliveryErr.Error()
		deliveryErrMsg = &msg
	}

	share, err := s.Q.CreateEstimateInventoryShareLink(r.Context(), gen.CreateEstimateInventoryShareLinkParams{
		TenantID:       tenantID,
		EstimateID:     estimateID,
		TokenHash:      tokenHash,
		RecipientEmail: recipientEmail,
		DeliveryMode:   deliveryMode,
		DeliveryError:  deliveryErrMsg,
		CreatedBy:      &userID,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create share link", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "inventory.share_link.created",
		EntityType: "estimate",
		EntityID:   &estimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"shareLinkId":    share.ID,
			"expiresAt":      share.ExpiresAt.UTC().Format(time.RFC3339),
			"recipientEmail": recipientEmail,
			"deliveryMode":   deliveryMode,
			"deliveryError":  deliveryErr != nil,
		},
	})
	s.trackAnalyticsEvent(r.Context(), tenantID, &userID, &estimateID, "estimate.inventory_link_sent", map[string]any{
		"via":           "inventory_tab",
		"delivery_mode": deliveryMode,
	})

	httpx.WriteJSON(w, http.StatusCreated, createInventoryShareLinkResponse{
		ShareLinkID:    share.ID,
		EstimateID:     estimateID,
		RecipientEmail: recipientEmail,
		ShareURL:       shareURL,
		ExpiresAt:      share.ExpiresAt.UTC(),
		DeliveryMode:   deliveryMode,
		RequestID:      middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetPublicInventoryToken(w http.ResponseWriter, r *http.Request, token string) {
	share, estimate, items, err := s.getPublicInventoryPayload(r, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "inventory_share_not_found", "Inventory share link is invalid", nil)
			return
		}
		if errors.Is(err, errExpiredInventoryToken) {
			httpx.WriteError(w, r, http.StatusGone, "inventory_share_expired", "Inventory share link expired", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load public inventory", nil)
		return
	}

	now := time.Now().UTC()
	_, _ = s.Q.TouchEstimateInventoryShareLink(r.Context(), gen.TouchEstimateInventoryShareLinkParams{
		ID:             share.ID,
		TenantID:       share.TenantID,
		LastAccessedAt: &now,
	})

	httpx.WriteJSON(w, http.StatusOK, publicInventoryResponse{
		EstimateID:    estimate.ID,
		CustomerName:  estimate.CustomerName,
		MoveDate:      dateOnly(estimate.MoveDate),
		Items:         mapInventoryItems(items),
		TotalVolumeCf: roundCF(estimate.TotalVolumeCf),
		ExpiresAt:     share.ExpiresAt.UTC(),
		RequestID:     middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PutPublicInventoryToken(w http.ResponseWriter, r *http.Request, token string) {
	var req replaceInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	items, totalCF, err := normalizeInventoryItems(req.Items)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	share, estimate, _, err := s.getPublicInventoryPayload(r, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "inventory_share_not_found", "Inventory share link is invalid", nil)
			return
		}
		if errors.Is(err, errExpiredInventoryToken) {
			httpx.WriteError(w, r, http.StatusGone, "inventory_share_expired", "Inventory share link expired", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load public inventory", nil)
		return
	}

	updatedEstimate, persistedItems, err := s.replaceEstimateInventory(r.Context(), share.TenantID, share.EstimateID, nil, items)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update inventory", nil)
		return
	}

	now := time.Now().UTC()
	_, _ = s.Q.TouchEstimateInventoryShareLink(r.Context(), gen.TouchEstimateInventoryShareLinkParams{
		ID:             share.ID,
		TenantID:       share.TenantID,
		LastAccessedAt: &now,
		LastUpdatedAt:  &now,
	})

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   share.TenantID,
		Action:     "inventory.updated",
		EntityType: "estimate",
		EntityID:   &share.EstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"via":           "public_link",
			"shareLinkId":   share.ID,
			"itemCount":     len(items),
			"totalVolumeCf": roundCF(updatedEstimate.TotalVolumeCf),
		},
	})
	s.trackAnalyticsEvent(r.Context(), share.TenantID, nil, &share.EstimateID, "estimate.inventory_updated", map[string]any{
		"via":        "public_link",
		"item_count": len(items),
		"total_cf":   totalCF,
	})

	httpx.WriteJSON(w, http.StatusOK, publicInventoryResponse{
		EstimateID:    updatedEstimate.ID,
		CustomerName:  estimate.CustomerName,
		MoveDate:      dateOnly(estimate.MoveDate),
		Items:         mapInventoryItems(persistedItems),
		TotalVolumeCf: roundCF(updatedEstimate.TotalVolumeCf),
		ExpiresAt:     share.ExpiresAt.UTC(),
		RequestID:     middleware.RequestIDFromContext(r.Context()),
	})
}

var errExpiredInventoryToken = errors.New("inventory token expired")

func (s *Server) getPublicInventoryPayload(r *http.Request, token string) (gen.EstimateInventoryShareLink, gen.Estimate, []gen.EstimateInventoryItem, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return gen.EstimateInventoryShareLink{}, gen.Estimate{}, nil, pgx.ErrNoRows
	}

	share, err := s.Q.GetEstimateInventoryShareLinkByTokenHash(r.Context(), auth.HashToken(trimmed))
	if err != nil {
		return gen.EstimateInventoryShareLink{}, gen.Estimate{}, nil, err
	}
	if time.Now().UTC().After(share.ExpiresAt.UTC()) {
		return gen.EstimateInventoryShareLink{}, gen.Estimate{}, nil, errExpiredInventoryToken
	}

	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{
		ID:       share.EstimateID,
		TenantID: share.TenantID,
	})
	if err != nil {
		return gen.EstimateInventoryShareLink{}, gen.Estimate{}, nil, err
	}

	items, err := s.Q.GetEstimateInventoryItems(r.Context(), gen.GetEstimateInventoryItemsParams{
		TenantID:   share.TenantID,
		EstimateID: share.EstimateID,
	})
	if err != nil {
		return gen.EstimateInventoryShareLink{}, gen.Estimate{}, nil, err
	}

	return share, estimate, items, nil
}

func (s *Server) replaceEstimateInventory(
	ctx context.Context,
	tenantID uuid.UUID,
	estimateID uuid.UUID,
	updatedBy *uuid.UUID,
	items []inventoryItemPayload,
) (gen.Estimate, []gen.EstimateInventoryItem, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return gen.Estimate{}, nil, err
	}
	defer tx.Rollback(ctx)
	qtx := s.Q.WithTx(tx)

	if _, err := qtx.GetEstimateByID(ctx, gen.GetEstimateByIDParams{
		ID:       estimateID,
		TenantID: tenantID,
	}); err != nil {
		return gen.Estimate{}, nil, err
	}

	if err := qtx.DeleteEstimateInventoryItems(ctx, gen.DeleteEstimateInventoryItemsParams{
		TenantID:   tenantID,
		EstimateID: estimateID,
	}); err != nil {
		return gen.Estimate{}, nil, err
	}

	inserted := make([]gen.EstimateInventoryItem, 0, len(items))
	totalCF := 0.0
	for _, item := range items {
		row, err := qtx.InsertEstimateInventoryItem(ctx, gen.InsertEstimateInventoryItemParams{
			TenantID:   tenantID,
			EstimateID: estimateID,
			Category:   item.Category,
			ItemName:   item.ItemName,
			VolumeCf:   item.VolumeCf,
			Qty:        int32(item.Qty),
			IsCustom:   &item.IsCustom,
		})
		if err != nil {
			return gen.Estimate{}, nil, err
		}
		inserted = append(inserted, row)
		totalCF += item.VolumeCf * float64(item.Qty)
	}
	totalCF = roundCF(totalCF)

	affected, err := qtx.UpdateEstimateTotalVolumeCf(ctx, gen.UpdateEstimateTotalVolumeCfParams{
		TotalVolumeCf: totalCF,
		UpdatedBy:     updatedBy,
		EstimateID:    estimateID,
		TenantID:      tenantID,
	})
	if err != nil {
		return gen.Estimate{}, nil, err
	}
	if affected == 0 {
		return gen.Estimate{}, nil, pgx.ErrNoRows
	}

	estimate, err := qtx.GetEstimateByID(ctx, gen.GetEstimateByIDParams{
		ID:       estimateID,
		TenantID: tenantID,
	})
	if err != nil {
		return gen.Estimate{}, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return gen.Estimate{}, nil, err
	}

	return estimate, inserted, nil
}

func normalizeInventoryItems(items []inventoryItemPayload) ([]inventoryItemPayload, float64, error) {
	if len(items) > maxInventoryItems {
		return nil, 0, fmt.Errorf("items must be %d or fewer", maxInventoryItems)
	}

	type dedupeValue struct {
		item inventoryItemPayload
	}
	deduped := make(map[string]dedupeValue, len(items))
	for _, raw := range items {
		category := strings.TrimSpace(raw.Category)
		itemName := strings.TrimSpace(raw.ItemName)
		if category == "" || itemName == "" {
			return nil, 0, errors.New("category and itemName are required")
		}
		if raw.VolumeCf < 0 {
			return nil, 0, errors.New("volumeCf must be greater than or equal to 0")
		}
		if raw.Qty < 0 {
			return nil, 0, errors.New("qty must be greater than or equal to 0")
		}
		if raw.Qty == 0 {
			continue
		}

		normalized := inventoryItemPayload{
			Category: category,
			ItemName: itemName,
			VolumeCf: roundCF(raw.VolumeCf),
			Qty:      raw.Qty,
			IsCustom: raw.IsCustom,
		}

		key := strings.ToLower(category) + "|" + strings.ToLower(itemName) + "|" + fmt.Sprint(raw.IsCustom)
		if existing, ok := deduped[key]; ok {
			existing.item.Qty += normalized.Qty
			deduped[key] = existing
			continue
		}
		deduped[key] = dedupeValue{item: normalized}
	}

	normalizedItems := make([]inventoryItemPayload, 0, len(deduped))
	totalCF := 0.0
	for _, value := range deduped {
		normalizedItems = append(normalizedItems, value.item)
		totalCF += value.item.VolumeCf * float64(value.item.Qty)
	}

	sort.Slice(normalizedItems, func(i, j int) bool {
		left := normalizedItems[i]
		right := normalizedItems[j]
		leftCategory := strings.ToLower(left.Category)
		rightCategory := strings.ToLower(right.Category)
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		leftName := strings.ToLower(left.ItemName)
		rightName := strings.ToLower(right.ItemName)
		if leftName != rightName {
			return leftName < rightName
		}
		if left.IsCustom != right.IsCustom {
			return !left.IsCustom && right.IsCustom
		}
		return false
	})

	return normalizedItems, roundCF(totalCF), nil
}

func mapInventoryItems(rows []gen.EstimateInventoryItem) []inventoryItemPayload {
	items := make([]inventoryItemPayload, 0, len(rows))
	for _, row := range rows {
		items = append(items, inventoryItemPayload{
			Category: row.Category,
			ItemName: row.ItemName,
			VolumeCf: roundCF(row.VolumeCf),
			Qty:      int(row.Qty),
			IsCustom: row.IsCustom,
		})
	}
	return items
}

func roundCF(value float64) float64 {
	return math.Round(value*100) / 100
}

func (s *Server) sendInventoryShareEmail(r *http.Request, recipientEmail, shareURL string) (string, error) {
	subject := "Please complete your inventory for your move"
	body := "Please complete your inventory for your move.\n\n" +
		"Use this secure link:\n" + shareURL + "\n\n" +
		"Thank you,\nMoveOps"

	mode := strings.ToLower(strings.TrimSpace(s.Config.EmailMode))
	if mode == "smtp" {
		if strings.TrimSpace(s.Config.SMTPHost) == "" {
			err := errors.New("smtp mode configured but SMTP_HOST is empty")
			s.Logger.Warn("inventory share smtp unavailable, falling back to log", "error", err.Error())
			s.Logger.Info("inventory share email fallback", "recipient", recipientEmail, "shareUrl", shareURL, "mode", "log", "requestId", middleware.RequestIDFromContext(r.Context()))
			return "log", err
		}
		if err := sendSMTPEmail(s.Config, recipientEmail, subject, body); err == nil {
			return "smtp", nil
		} else {
			s.Logger.Warn("inventory share smtp send failed, falling back to log", "error", err.Error())
			s.Logger.Info("inventory share email fallback", "recipient", recipientEmail, "shareUrl", shareURL, "mode", "log", "requestId", middleware.RequestIDFromContext(r.Context()))
			return "log", err
		}
	}

	s.Logger.Info("inventory share email fallback", "recipient", recipientEmail, "shareUrl", shareURL, "mode", "log", "requestId", middleware.RequestIDFromContext(r.Context()))
	return "log", nil
}

func sendSMTPEmail(cfg config.Config, recipientEmail, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	msg := []byte("From: " + cfg.EmailFrom + "\r\n" +
		"To: " + recipientEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	var auth smtp.Auth
	if strings.TrimSpace(cfg.SMTPUser) != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}

	return smtp.SendMail(addr, auth, cfg.EmailFrom, []string{recipientEmail}, msg)
}
