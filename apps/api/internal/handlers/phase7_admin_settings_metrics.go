package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/moveops-platform/apps/api/internal/audit"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/gen/oapi"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	adminAuditDefaultLimit = 50
	adminAuditMaxLimit     = 200
	catalogImportMaxRows   = 5000
	defaultStuckDays       = 7
)

var (
	allowedAnalyticsEvents = map[string]struct{}{
		"estimate.entry_started":       {},
		"estimate.inventory_updated":   {},
		"estimate.inventory_link_sent": {},
		"estimate.charges_updated":     {},
		"estimate.quote_sent":          {},
		"estimate.sign_requested":      {},
		"estimate.sign_completed":      {},
		"estimate.booked":              {},
		"estimate.follow_up_set":       {},
	}

	emailTemplateAllowedVariables = []string{
		"customer_name",
		"quote_link",
		"inventory_link",
		"signature_link",
		"estimate_number",
		"move_date",
	}

	disallowedPIIPropertyKeys = map[string]struct{}{
		"email":    {},
		"phone":    {},
		"address":  {},
		"notes":    {},
		"customer": {},
	}
)

type phase7CatalogItemRow interface {
	GetCategoryID() *uuid.UUID
	GetCategoryName() *string
	GetID() uuid.UUID
	GetName() string
	GetSortOrder() int32
	GetActive() bool
	GetVolumeCf() float64
}

func (s *Server) GetEstimatesEstimateIdInventoryCatalog(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	if _, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: estimateID, TenantID: tenantID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	categories, items, err := s.loadEffectiveCatalog(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load inventory catalog", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateCatalogResponse{
		Categories: categories,
		Items:      items,
		RequestId:  middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetAdminNewEstimateCatalogCategories(w http.ResponseWriter, r *http.Request) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	rows, err := s.Q.ListNewEstimateCatalogCategories(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load catalog categories", nil)
		return
	}

	categories := make([]oapi.NewEstimateCatalogCategory, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, mapCatalogCategory(row))
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateCatalogCategoryListResponse{
		Categories: categories,
		RequestId:  middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostAdminNewEstimateCatalogCategories(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.CreateNewEstimateCatalogCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "name is required", nil)
		return
	}

	sortOrder := int32(0)
	if req.SortOrder != nil {
		sortOrder = int32(*req.SortOrder)
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	row, err := s.Q.CreateNewEstimateCatalogCategory(r.Context(), gen.CreateNewEstimateCatalogCategoryParams{
		TenantID:  tenantID,
		Name:      name,
		SortOrder: sortOrder,
		Active:    active,
		CreatedBy: &userID,
		UpdatedBy: &userID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, r, http.StatusConflict, "catalog_category_exists", "Catalog category already exists", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create catalog category", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "catalog.category.created", "new_estimate_catalog_category", &row.ID, map[string]any{"name": row.Name})

	httpx.WriteJSON(w, http.StatusCreated, oapi.NewEstimateCatalogCategoryResponse{
		Category:  mapCatalogCategory(row),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PatchAdminNewEstimateCatalogCategoriesCategoryId(w http.ResponseWriter, r *http.Request, categoryID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.UpdateNewEstimateCatalogCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	var name *string
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "name cannot be empty", nil)
			return
		}
		name = &trimmed
	}

	var sortOrder *int32
	if req.SortOrder != nil {
		v := int32(*req.SortOrder)
		sortOrder = &v
	}

	row, err := s.Q.UpdateNewEstimateCatalogCategory(r.Context(), gen.UpdateNewEstimateCatalogCategoryParams{
		TenantID:  tenantID,
		ID:        categoryID,
		Name:      name,
		SortOrder: sortOrder,
		Active:    req.Active,
		UpdatedBy: &userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "catalog_category_not_found", "Catalog category was not found", nil)
			return
		}
		if isUniqueViolation(err) {
			httpx.WriteError(w, r, http.StatusConflict, "catalog_category_exists", "Catalog category already exists", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update catalog category", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "catalog.category.updated", "new_estimate_catalog_category", &row.ID, map[string]any{"name": row.Name})

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateCatalogCategoryResponse{
		Category:  mapCatalogCategory(row),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) DeleteAdminNewEstimateCatalogCategoriesCategoryId(w http.ResponseWriter, r *http.Request, categoryID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	affected, err := s.Q.DeleteNewEstimateCatalogCategory(r.Context(), gen.DeleteNewEstimateCatalogCategoryParams{TenantID: tenantID, ID: categoryID})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete catalog category", nil)
		return
	}
	if affected == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "catalog_category_not_found", "Catalog category was not found", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "catalog.category.deleted", "new_estimate_catalog_category", &categoryID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetAdminNewEstimateCatalogItems(w http.ResponseWriter, r *http.Request) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	rows, err := s.Q.ListNewEstimateCatalogItems(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load catalog items", nil)
		return
	}

	items := make([]oapi.NewEstimateCatalogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapCatalogItem(row.ID, row.CategoryID, row.CategoryName, row.Name, row.VolumeCf, row.SortOrder, row.Active))
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateCatalogItemListResponse{
		Items:     items,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostAdminNewEstimateCatalogItems(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.CreateNewEstimateCatalogItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	name := strings.TrimSpace(req.ItemName)
	if name == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "itemName is required", nil)
		return
	}
	if req.VolumeCf < 0 {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "volumeCf must be >= 0", nil)
		return
	}

	if req.CategoryId != nil {
		if _, err := s.Q.GetNewEstimateCatalogCategoryByID(r.Context(), gen.GetNewEstimateCatalogCategoryByIDParams{TenantID: tenantID, ID: uuid.UUID(*req.CategoryId)}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "categoryId is invalid", nil)
				return
			}
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to validate category", nil)
			return
		}
	}

	sortOrder := int32(0)
	if req.SortOrder != nil {
		sortOrder = int32(*req.SortOrder)
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	row, err := s.Q.CreateNewEstimateCatalogItem(r.Context(), gen.CreateNewEstimateCatalogItemParams{
		TenantID:   tenantID,
		CategoryID: uuidPtrFromOpenAPI(req.CategoryId),
		Name:       name,
		VolumeCf:   req.VolumeCf,
		SortOrder:  sortOrder,
		Active:     active,
		CreatedBy:  &userID,
		UpdatedBy:  &userID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, r, http.StatusConflict, "catalog_item_exists", "Catalog item already exists in this category", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create catalog item", nil)
		return
	}

	categoryName := ""
	if row.CategoryID != nil {
		cat, err := s.Q.GetNewEstimateCatalogCategoryByID(r.Context(), gen.GetNewEstimateCatalogCategoryByIDParams{TenantID: tenantID, ID: *row.CategoryID})
		if err == nil {
			categoryName = cat.Name
		}
	}

	s.logAdminAudit(r, tenantID, userID, "catalog.item.created", "new_estimate_catalog_item", &row.ID, map[string]any{"name": row.Name})

	mapped := mapCatalogItem(row.ID, row.CategoryID, optionalString(categoryName), row.Name, row.VolumeCf, row.SortOrder, row.Active)
	httpx.WriteJSON(w, http.StatusCreated, oapi.NewEstimateCatalogItemResponse{Item: mapped, RequestId: middleware.RequestIDFromContext(r.Context())})
}

func (s *Server) PatchAdminNewEstimateCatalogItemsItemId(w http.ResponseWriter, r *http.Request, itemID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.UpdateNewEstimateCatalogItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	if req.CategoryId != nil {
		if _, err := s.Q.GetNewEstimateCatalogCategoryByID(r.Context(), gen.GetNewEstimateCatalogCategoryByIDParams{TenantID: tenantID, ID: uuid.UUID(*req.CategoryId)}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "categoryId is invalid", nil)
				return
			}
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to validate category", nil)
			return
		}
	}

	var name *string
	if req.ItemName != nil {
		trimmed := strings.TrimSpace(*req.ItemName)
		if trimmed == "" {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "itemName cannot be empty", nil)
			return
		}
		name = &trimmed
	}

	var volumeCf *float64
	if req.VolumeCf != nil {
		if *req.VolumeCf < 0 {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "volumeCf must be >= 0", nil)
			return
		}
		volumeCf = req.VolumeCf
	}

	var sortOrder *int32
	if req.SortOrder != nil {
		v := int32(*req.SortOrder)
		sortOrder = &v
	}

	row, err := s.Q.UpdateNewEstimateCatalogItem(r.Context(), gen.UpdateNewEstimateCatalogItemParams{
		TenantID:   tenantID,
		ID:         itemID,
		CategoryID: uuidPtrFromOpenAPI(req.CategoryId),
		Name:       name,
		VolumeCf:   volumeCf,
		SortOrder:  sortOrder,
		Active:     req.Active,
		UpdatedBy:  &userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "catalog_item_not_found", "Catalog item was not found", nil)
			return
		}
		if isUniqueViolation(err) {
			httpx.WriteError(w, r, http.StatusConflict, "catalog_item_exists", "Catalog item already exists in this category", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update catalog item", nil)
		return
	}

	withCategory, err := s.Q.GetNewEstimateCatalogItemByID(r.Context(), gen.GetNewEstimateCatalogItemByIDParams{TenantID: tenantID, ID: row.ID})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load updated catalog item", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "catalog.item.updated", "new_estimate_catalog_item", &row.ID, map[string]any{"name": row.Name})

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateCatalogItemResponse{
		Item:      mapCatalogItem(withCategory.ID, withCategory.CategoryID, withCategory.CategoryName, withCategory.Name, withCategory.VolumeCf, withCategory.SortOrder, withCategory.Active),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) DeleteAdminNewEstimateCatalogItemsItemId(w http.ResponseWriter, r *http.Request, itemID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	affected, err := s.Q.DeleteNewEstimateCatalogItem(r.Context(), gen.DeleteNewEstimateCatalogItemParams{TenantID: tenantID, ID: itemID})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete catalog item", nil)
		return
	}
	if affected == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "catalog_item_not_found", "Catalog item was not found", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "catalog.item.deleted", "new_estimate_catalog_item", &itemID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) PostAdminNewEstimateCatalogImport(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if contentType != "" && !strings.Contains(contentType, "text/csv") && !strings.Contains(contentType, "application/csv") {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "Content-Type must be text/csv", nil)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Failed to read CSV upload", nil)
		return
	}

	categories, items, parseErrors := parseCatalogCSV(body)
	if len(parseErrors) > 0 {
		httpx.WriteError(w, r, http.StatusBadRequest, "csv_validation_error", "Catalog CSV is invalid", map[string]any{"errors": parseErrors})
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start catalog import", nil)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.Q.WithTx(tx)

	if err := qtx.DeleteNewEstimateCatalogItemsByTenant(r.Context(), tenantID); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to clear existing catalog items", nil)
		return
	}
	if err := qtx.DeleteNewEstimateCatalogCategoriesByTenant(r.Context(), tenantID); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to clear existing catalog categories", nil)
		return
	}

	categoryIDByName := make(map[string]uuid.UUID, len(categories))
	for idx, category := range categories {
		row, createErr := qtx.CreateNewEstimateCatalogCategory(r.Context(), gen.CreateNewEstimateCatalogCategoryParams{
			TenantID:  tenantID,
			Name:      category,
			SortOrder: int32(idx),
			Active:    true,
			CreatedBy: &userID,
			UpdatedBy: &userID,
		})
		if createErr != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to import catalog categories", nil)
			return
		}
		categoryIDByName[strings.ToLower(category)] = row.ID
	}

	for _, item := range items {
		catID := categoryIDByName[strings.ToLower(item.category)]
		if _, createErr := qtx.CreateNewEstimateCatalogItem(r.Context(), gen.CreateNewEstimateCatalogItemParams{
			TenantID:   tenantID,
			CategoryID: &catID,
			Name:       item.itemName,
			VolumeCf:   item.volumeCf,
			SortOrder:  int32(item.sortOrder),
			Active:     item.active,
			CreatedBy:  &userID,
			UpdatedBy:  &userID,
		}); createErr != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to import catalog items", nil)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit catalog import", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "catalog.imported", "new_estimate_catalog", nil, map[string]any{
		"categoriesImported": len(categories),
		"itemsImported":      len(items),
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateCatalogImportResponse{
		CategoriesImported: len(categories),
		ItemsImported:      len(items),
		RequestId:          middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetAdminNewEstimateCatalogExport(w http.ResponseWriter, r *http.Request) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	rows, err := s.Q.ListNewEstimateCatalogItems(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to export catalog", nil)
		return
	}

	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	_ = writer.Write([]string{"category", "item_name", "volume_cf", "active", "item_sort_order", "category_sort_order"})
	for _, row := range rows {
		categoryName := ""
		categorySortOrder := ""
		if row.CategoryName != nil {
			categoryName = *row.CategoryName
		}
		if row.CategorySortOrder != nil {
			categorySortOrder = strconv.Itoa(int(*row.CategorySortOrder))
		}
		_ = writer.Write([]string{
			categoryName,
			row.Name,
			strconv.FormatFloat(row.VolumeCf, 'f', -1, 64),
			strconv.FormatBool(row.Active),
			strconv.Itoa(int(row.SortOrder)),
			categorySortOrder,
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to build catalog CSV", nil)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"new-estimate-catalog.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) GetAdminNewEstimatePricing(w http.ResponseWriter, r *http.Request) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	pricing, err := s.getTenantPricingDefaults(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load pricing defaults", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimatePricingDefaultsResponse{
		Pricing:   pricing,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PutAdminNewEstimatePricing(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.NewEstimatePricingDefaults
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to encode pricing defaults", nil)
		return
	}

	if _, err := s.Q.UpsertTenantNewEstimateSettings(r.Context(), gen.UpsertTenantNewEstimateSettingsParams{
		TenantID:            tenantID,
		PricingDefaultsJson: raw,
		UpdatedBy:           &userID,
	}); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save pricing defaults", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "pricing_defaults.updated", "tenant_new_estimate_settings", nil, nil)

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimatePricingDefaultsResponse{
		Pricing:   req,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetAdminNewEstimateEmailTemplates(w http.ResponseWriter, r *http.Request) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	templates, err := s.getTenantEmailTemplates(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load email templates", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateEmailTemplatesResponse{
		Templates:        templates,
		AllowedVariables: emailTemplateAllowedVariables,
		RequestId:        middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PutAdminNewEstimateEmailTemplates(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.NewEstimateEmailTemplates
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to encode email templates", nil)
		return
	}

	if _, err := s.Q.UpsertTenantNewEstimateSettings(r.Context(), gen.UpsertTenantNewEstimateSettingsParams{
		TenantID:           tenantID,
		EmailTemplatesJson: raw,
		UpdatedBy:          &userID,
	}); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save email templates", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "email_templates.updated", "tenant_new_estimate_settings", nil, nil)

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateEmailTemplatesResponse{
		Templates:        req,
		AllowedVariables: emailTemplateAllowedVariables,
		RequestId:        middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostAdminNewEstimateEmailTemplatesTestSend(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.NewEstimateEmailTemplateTestSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	templates, err := s.getTenantEmailTemplates(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load email templates", nil)
		return
	}

	chosen := chooseEmailTemplateForTest(req.TemplateKey, templates)
	subject := strings.TrimSpace(valueOrDefault(chosen.Subject, "MoveOps test email"))
	bodyTemplate := valueOrDefault(chosen.TextBody, "Hello {{customer_name}},\n\nThis is a test message from MoveOps.\n\nMoveOps Team")
	body := renderEmailTemplateText(bodyTemplate, map[string]string{
		"customer_name":   "Sample Customer",
		"quote_link":      "https://example.com/public/estimate/token",
		"inventory_link":  "https://example.com/public/inventory/token",
		"signature_link":  "https://example.com/public/sign/token",
		"estimate_number": "E-000001",
		"move_date":       "2026-02-21",
	})

	delivery := s.sendTransactionalEmailWithContext(r.Context(), string(req.ToEmail), nil, subject, body)
	status := oapi.NewEstimateEmailTemplateTestSendResponseStatus("logged")
	if delivery.Status == string(oapi.EstimateEmailLogStatusFailed) {
		status = oapi.NewEstimateEmailTemplateTestSendResponseStatus("failed")
	} else if delivery.Mode == string(oapi.Smtp) {
		status = oapi.NewEstimateEmailTemplateTestSendResponseStatus("sent")
	}

	s.logAdminAudit(r, tenantID, userID, "email_templates.test_sent", "tenant_new_estimate_settings", nil, map[string]any{
		"templateKey":  req.TemplateKey,
		"status":       status,
		"deliveryMode": delivery.Mode,
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateEmailTemplateTestSendResponse{
		Status:    status,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetAdminNewEstimateDocuments(w http.ResponseWriter, r *http.Request) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	branding, err := s.getTenantDocumentBranding(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load document branding", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateDocumentBrandingResponse{
		Branding:  branding,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PutAdminNewEstimateDocuments(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.NewEstimateDocumentBranding
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to encode document branding", nil)
		return
	}

	if _, err := s.Q.UpsertTenantNewEstimateSettings(r.Context(), gen.UpsertTenantNewEstimateSettingsParams{
		TenantID:             tenantID,
		DocumentBrandingJson: raw,
		UpdatedBy:            &userID,
	}); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save document branding", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "document_branding.updated", "tenant_new_estimate_settings", nil, nil)

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateDocumentBrandingResponse{
		Branding:  req,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostAnalyticsEvents(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.TrackAnalyticsEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	eventName := strings.TrimSpace(req.EventName)
	if eventName == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "eventName is required", nil)
		return
	}
	if _, allowed := allowedAnalyticsEvents[eventName]; !allowed {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "eventName is not allowed", nil)
		return
	}

	var estimateID *uuid.UUID
	if req.EstimateId != nil {
		parsed := uuid.UUID(*req.EstimateId)
		if _, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: parsed, TenantID: tenantID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
				return
			}
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
			return
		}
		estimateID = &parsed
	}

	properties := map[string]interface{}{}
	if req.Properties != nil {
		properties = *req.Properties
	}
	safeProperties := sanitizeAnalyticsProperties(properties)
	raw, err := json.Marshal(safeProperties)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to encode analytics properties", nil)
		return
	}

	if err := s.Q.InsertAnalyticsEvent(r.Context(), gen.InsertAnalyticsEventParams{
		TenantID:       tenantID,
		EstimateID:     estimateID,
		UserID:         &userID,
		EventName:      eventName,
		PropertiesJson: raw,
	}); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to store analytics event", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetAdminNewEstimateMetrics(w http.ResponseWriter, r *http.Request, params oapi.GetAdminNewEstimateMetricsParams) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	median, err := s.Q.GetAnalyticsMedianTimeToQuoteMinutes(r.Context(), gen.GetAnalyticsMedianTimeToQuoteMinutesParams{TenantID: tenantID, FromTime: params.From, ToTime: params.To})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load median time-to-quote", nil)
		return
	}

	quoteToSign, err := s.Q.GetAnalyticsQuoteToSignCounts(r.Context(), gen.GetAnalyticsQuoteToSignCountsParams{TenantID: tenantID, FromTime: params.From, ToTime: params.To})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load quote-to-sign conversion", nil)
		return
	}

	signToBook, err := s.Q.GetAnalyticsSignToBookCounts(r.Context(), gen.GetAnalyticsSignToBookCountsParams{TenantID: tenantID, FromTime: params.From, ToTime: params.To})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load sign-to-book conversion", nil)
		return
	}

	inventoryCompletion, err := s.Q.GetAnalyticsInventoryCompletionCounts(r.Context(), gen.GetAnalyticsInventoryCompletionCountsParams{TenantID: tenantID, FromTime: params.From, ToTime: params.To})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load inventory completion", nil)
		return
	}

	stuckCount, err := s.Q.CountStuckEstimates(r.Context(), gen.CountStuckEstimatesParams{TenantID: tenantID, StuckDays: defaultStuckDays})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load stuck estimates", nil)
		return
	}

	s.logAdminAudit(r, tenantID, userID, "metrics.viewed", "analytics", nil, nil)

	httpx.WriteJSON(w, http.StatusOK, oapi.NewEstimateMetricsResponse{
		Metrics: oapi.NewEstimateMetrics{
			MedianTimeToQuoteMinutes: median,
			QuoteToSign:              toConversionMetric(quoteToSign.ConvertedCount, quoteToSign.TotalCount),
			SignToBook:               toConversionMetric(signToBook.ConvertedCount, signToBook.TotalCount),
			InventoryCompletion:      toConversionMetric(inventoryCompletion.ConvertedCount, inventoryCompletion.TotalCount),
			StuckEstimatesCount:      stuckCount,
		},
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetAdminAuditLogs(w http.ResponseWriter, r *http.Request, params oapi.GetAdminAuditLogsParams) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	limit := adminAuditDefaultLimit
	if params.Limit != nil {
		limit = *params.Limit
	}
	if limit < 1 {
		limit = 1
	}
	if limit > adminAuditMaxLimit {
		limit = adminAuditMaxLimit
	}

	offset := 0
	if params.Offset != nil && *params.Offset > 0 {
		offset = *params.Offset
	}

	var actorUserID *uuid.UUID
	if params.ActorUserId != nil {
		parsed := uuid.UUID(*params.ActorUserId)
		actorUserID = &parsed
	}

	var entityID *uuid.UUID
	if params.EntityId != nil {
		parsed := uuid.UUID(*params.EntityId)
		entityID = &parsed
	}

	rows, err := s.Q.ListAuditLogsForTenant(r.Context(), gen.ListAuditLogsForTenantParams{
		TenantID:   tenantID,
		FromTime:   params.From,
		ToTime:     params.To,
		UserID:     actorUserID,
		ActionLike: params.Action,
		EntityType: params.EntityType,
		EntityID:   entityID,
		OffsetRows: int32(offset),
		LimitRows:  int32(limit),
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load audit logs", nil)
		return
	}

	total, err := s.Q.CountAuditLogsForTenant(r.Context(), gen.CountAuditLogsForTenantParams{
		TenantID:   tenantID,
		FromTime:   params.From,
		ToTime:     params.To,
		UserID:     actorUserID,
		ActionLike: params.Action,
		EntityType: params.EntityType,
		EntityID:   entityID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count audit logs", nil)
		return
	}

	items := make([]oapi.AdminAuditLogEntry, 0, len(rows))
	for _, row := range rows {
		metadata := map[string]any{}
		if len(row.Metadata) > 0 {
			if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
				metadata = map[string]any{"raw": string(row.Metadata)}
			}
		}

		entry := oapi.AdminAuditLogEntry{
			Id:         row.ID,
			Action:     row.Action,
			EntityType: row.EntityType,
			Metadata:   metadata,
			CreatedAt:  row.CreatedAt.UTC(),
		}
		if row.UserID != nil {
			v := openapi_types.UUID(*row.UserID)
			entry.UserId = &v
		}
		if row.EntityID != nil {
			v := openapi_types.UUID(*row.EntityID)
			entry.EntityId = &v
		}
		entry.RequestId = row.RequestID
		items = append(items, entry)
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.AdminAuditLogListResponse{
		Items:     items,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) loadEffectiveCatalog(ctx context.Context, tenantID uuid.UUID) ([]oapi.NewEstimateCatalogCategory, []oapi.NewEstimateCatalogItem, error) {
	rows, err := s.Q.ListActiveNewEstimateCatalogItems(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return fallbackCatalog(), nil, nil
	}

	categories := make([]oapi.NewEstimateCatalogCategory, 0)
	seenCategories := make(map[string]struct{})
	items := make([]oapi.NewEstimateCatalogItem, 0, len(rows))
	for _, row := range rows {
		if row.CategoryName != nil {
			key := strings.ToLower(strings.TrimSpace(*row.CategoryName))
			if key != "" {
				if _, exists := seenCategories[key]; !exists {
					sortOrder := 0
					if row.CategorySortOrder != nil {
						sortOrder = int(*row.CategorySortOrder)
					}
					categories = append(categories, oapi.NewEstimateCatalogCategory{Id: uuid.New(), Name: *row.CategoryName, SortOrder: sortOrder, Active: true})
					seenCategories[key] = struct{}{}
				}
			}
		}
		items = append(items, mapCatalogItem(row.ID, row.CategoryID, row.CategoryName, row.Name, row.VolumeCf, row.SortOrder, row.Active))
	}

	sort.SliceStable(categories, func(i, j int) bool {
		if categories[i].SortOrder == categories[j].SortOrder {
			return strings.ToLower(categories[i].Name) < strings.ToLower(categories[j].Name)
		}
		return categories[i].SortOrder < categories[j].SortOrder
	})

	return categories, items, nil
}

func fallbackCatalog() []oapi.NewEstimateCatalogCategory {
	fallbackNames := []string{
		"Bedroom",
		"Living Room",
		"Dining Room",
		"Kitchen",
		"Appliances",
		"Office",
		"Garage",
		"Patio Furniture",
		"Boxes",
		"Miscellaneous",
		"Nursery",
		"Attic",
		"Basement",
		"Play Room",
		"Military",
	}
	out := make([]oapi.NewEstimateCatalogCategory, 0, len(fallbackNames))
	for idx, name := range fallbackNames {
		out = append(out, oapi.NewEstimateCatalogCategory{Id: uuid.New(), Name: name, SortOrder: idx, Active: true})
	}
	return out
}

func (s *Server) getTenantSettings(ctx context.Context, tenantID uuid.UUID) (gen.TenantNewEstimateSetting, error) {
	row, err := s.Q.GetTenantNewEstimateSettings(ctx, tenantID)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return gen.TenantNewEstimateSetting{}, err
	}

	return gen.TenantNewEstimateSetting{
		TenantID:             tenantID,
		PricingDefaultsJson:  []byte("{}"),
		EmailTemplatesJson:   []byte("{}"),
		DocumentBrandingJson: []byte("{}"),
	}, nil
}

func (s *Server) getTenantPricingDefaults(ctx context.Context, tenantID uuid.UUID) (oapi.NewEstimatePricingDefaults, error) {
	settings, err := s.getTenantSettings(ctx, tenantID)
	if err != nil {
		return oapi.NewEstimatePricingDefaults{}, err
	}
	if len(settings.PricingDefaultsJson) == 0 {
		return oapi.NewEstimatePricingDefaults{}, nil
	}

	pricing := oapi.NewEstimatePricingDefaults{}
	if err := json.Unmarshal(settings.PricingDefaultsJson, &pricing); err != nil {
		return oapi.NewEstimatePricingDefaults{}, nil
	}
	return pricing, nil
}

func (s *Server) getTenantEmailTemplates(ctx context.Context, tenantID uuid.UUID) (oapi.NewEstimateEmailTemplates, error) {
	settings, err := s.getTenantSettings(ctx, tenantID)
	if err != nil {
		return oapi.NewEstimateEmailTemplates{}, err
	}
	if len(settings.EmailTemplatesJson) == 0 {
		return defaultTenantEmailTemplates(), nil
	}
	templates := oapi.NewEstimateEmailTemplates{}
	if err := json.Unmarshal(settings.EmailTemplatesJson, &templates); err != nil {
		return defaultTenantEmailTemplates(), nil
	}
	return mergeEmailTemplates(defaultTenantEmailTemplates(), templates), nil
}

func (s *Server) getTenantDocumentBranding(ctx context.Context, tenantID uuid.UUID) (oapi.NewEstimateDocumentBranding, error) {
	settings, err := s.getTenantSettings(ctx, tenantID)
	if err != nil {
		return oapi.NewEstimateDocumentBranding{}, err
	}
	if len(settings.DocumentBrandingJson) == 0 {
		return oapi.NewEstimateDocumentBranding{}, nil
	}
	branding := oapi.NewEstimateDocumentBranding{}
	if err := json.Unmarshal(settings.DocumentBrandingJson, &branding); err != nil {
		return oapi.NewEstimateDocumentBranding{}, nil
	}
	return branding, nil
}

func defaultTenantEmailTemplates() oapi.NewEstimateEmailTemplates {
	eQuoteSubject := "Your moving estimate"
	eQuoteText := "Hi {{customer_name}},\n\nYour moving estimate is ready.\n\nView estimate: {{quote_link}}\n\nThank you,\nMoveOps"
	inventorySubject := "Please complete your inventory for your move"
	inventoryText := "Hi {{customer_name}},\n\nPlease complete your inventory so we can finalize your quote.\n\nUpdate inventory: {{inventory_link}}\n\nThank you,\nMoveOps"
	eSignSubject := "Please sign your estimate"
	eSignText := "Hi {{customer_name}},\n\nPlease sign your estimate using the secure link below.\n\nSign estimate: {{signature_link}}\n\nThank you,\nMoveOps"

	return oapi.NewEstimateEmailTemplates{
		EQuote:        &oapi.NewEstimateEmailTemplate{Subject: &eQuoteSubject, TextBody: &eQuoteText},
		InventoryLink: &oapi.NewEstimateEmailTemplate{Subject: &inventorySubject, TextBody: &inventoryText},
		ESign:         &oapi.NewEstimateEmailTemplate{Subject: &eSignSubject, TextBody: &eSignText},
	}
}

func mergeEmailTemplates(base oapi.NewEstimateEmailTemplates, overrides oapi.NewEstimateEmailTemplates) oapi.NewEstimateEmailTemplates {
	if overrides.EQuote != nil {
		base.EQuote = mergeSingleEmailTemplate(base.EQuote, overrides.EQuote)
	}
	if overrides.InventoryLink != nil {
		base.InventoryLink = mergeSingleEmailTemplate(base.InventoryLink, overrides.InventoryLink)
	}
	if overrides.ESign != nil {
		base.ESign = mergeSingleEmailTemplate(base.ESign, overrides.ESign)
	}
	return base
}

func mergeSingleEmailTemplate(base *oapi.NewEstimateEmailTemplate, override *oapi.NewEstimateEmailTemplate) *oapi.NewEstimateEmailTemplate {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		clone := *override
		return &clone
	}
	out := *base
	if override == nil {
		return &out
	}
	if override.Subject != nil {
		out.Subject = override.Subject
	}
	if override.TextBody != nil {
		out.TextBody = override.TextBody
	}
	if override.HtmlBody != nil {
		out.HtmlBody = override.HtmlBody
	}
	return &out
}

func chooseEmailTemplateForTest(key oapi.NewEstimateEmailTemplateTestSendRequestTemplateKey, templates oapi.NewEstimateEmailTemplates) *oapi.NewEstimateEmailTemplate {
	switch key {
	case oapi.EQuote:
		return templates.EQuote
	case oapi.InventoryLink:
		return templates.InventoryLink
	case oapi.ESign:
		return templates.ESign
	default:
		return templates.EQuote
	}
}

func resolveTenantTemplateCopy(
	templates oapi.NewEstimateEmailTemplates,
	templateKey oapi.EstimateEmailTemplateKey,
	fallbackSubject string,
	fallbackBody string,
) (string, string) {
	var template *oapi.NewEstimateEmailTemplate
	switch templateKey {
	case oapi.MovingEstimate:
		template = templates.EQuote
	case oapi.UpdateInventory:
		template = templates.InventoryLink
	case oapi.SignatureRequest:
		template = templates.ESign
	default:
		return fallbackSubject, fallbackBody
	}

	subject := fallbackSubject
	body := fallbackBody
	if template != nil {
		subject = valueOrDefault(template.Subject, fallbackSubject)
		body = valueOrDefault(template.TextBody, fallbackBody)
	}

	return subject, body
}

func renderEmailTemplateText(tmpl string, vars map[string]string) string {
	out := tmpl
	for key, value := range vars {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
		out = strings.ReplaceAll(out, "{{ "+key+" }}", value)
	}
	return out
}

func sanitizeAnalyticsProperties(input map[string]interface{}) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		lower := strings.ToLower(strings.TrimSpace(key))
		skip := false
		for disallowed := range disallowedPIIPropertyKeys {
			if strings.Contains(lower, disallowed) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out[key] = value
	}
	return out
}

func toConversionMetric(converted, total int64) oapi.NewEstimateConversionMetric {
	rate := 0.0
	if total > 0 {
		rate = float64(converted) / float64(total)
	}
	return oapi.NewEstimateConversionMetric{ConvertedCount: converted, TotalCount: total, Rate: rate}
}

func mapCatalogCategory(row gen.NewEstimateCatalogCategory) oapi.NewEstimateCatalogCategory {
	return oapi.NewEstimateCatalogCategory{
		Id:        row.ID,
		Name:      row.Name,
		SortOrder: int(row.SortOrder),
		Active:    row.Active,
	}
}

func mapCatalogItem(
	id uuid.UUID,
	categoryID *uuid.UUID,
	categoryName *string,
	name string,
	volumeCf float64,
	sortOrder int32,
	active bool,
) oapi.NewEstimateCatalogItem {
	item := oapi.NewEstimateCatalogItem{
		Id:        id,
		ItemName:  name,
		VolumeCf:  volumeCf,
		SortOrder: int(sortOrder),
		Active:    active,
	}
	if categoryID != nil {
		v := openapi_types.UUID(*categoryID)
		item.CategoryId = &v
	}
	item.CategoryName = categoryName
	return item
}

func uuidPtrFromOpenAPI(v *openapi_types.UUID) *uuid.UUID {
	if v == nil {
		return nil
	}
	u := uuid.UUID(*v)
	return &u
}

func valueOrDefault(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func cloneStringMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func optionalString(v string) *string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func emailToString(v *openapi_types.Email) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(*v))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Server) logAdminAudit(r *http.Request, tenantID uuid.UUID, userID uuid.UUID, action string, entityType string, entityID *uuid.UUID, metadata map[string]any) {
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata:   metadata,
	})
}

type catalogCSVItem struct {
	category  string
	itemName  string
	volumeCf  float64
	sortOrder int
	active    bool
}

func parseCatalogCSV(raw []byte) ([]string, []catalogCSVItem, []string) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, []string{fmt.Sprintf("failed to parse csv: %v", err)}
	}
	if len(records) < 2 {
		return nil, nil, []string{"csv must include header and at least one data row"}
	}
	if len(records)-1 > catalogImportMaxRows {
		return nil, nil, []string{fmt.Sprintf("csv row limit exceeded (%d)", catalogImportMaxRows)}
	}

	headers := make(map[string]int)
	for idx, col := range records[0] {
		headers[strings.ToLower(strings.TrimSpace(col))] = idx
	}

	required := []string{"category", "item_name", "volume_cf"}
	for _, key := range required {
		if _, ok := headers[key]; !ok {
			return nil, nil, []string{fmt.Sprintf("missing required column: %s", key)}
		}
	}

	categorySet := map[string]struct{}{}
	items := make([]catalogCSVItem, 0, len(records)-1)
	errs := make([]string, 0)

	for rowIndex := 1; rowIndex < len(records); rowIndex++ {
		row := records[rowIndex]
		valueAt := func(column string) string {
			idx := headers[column]
			if idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}

		category := valueAt("category")
		itemName := valueAt("item_name")
		if category == "" || itemName == "" {
			errs = append(errs, fmt.Sprintf("row %d: category and item_name are required", rowIndex+1))
			continue
		}

		volumeRaw := valueAt("volume_cf")
		volume, parseErr := strconv.ParseFloat(volumeRaw, 64)
		if parseErr != nil || volume < 0 {
			errs = append(errs, fmt.Sprintf("row %d: volume_cf must be a number >= 0", rowIndex+1))
			continue
		}

		sortOrder := rowIndex - 1
		if idx, ok := headers["item_sort_order"]; ok && idx < len(row) {
			trimmed := strings.TrimSpace(row[idx])
			if trimmed != "" {
				if parsed, convErr := strconv.Atoi(trimmed); convErr == nil {
					sortOrder = parsed
				}
			}
		}

		active := true
		if idx, ok := headers["active"]; ok && idx < len(row) {
			trimmed := strings.TrimSpace(strings.ToLower(row[idx]))
			if trimmed == "false" || trimmed == "0" || trimmed == "no" {
				active = false
			}
		}

		categorySet[strings.ToLower(category)] = struct{}{}
		items = append(items, catalogCSVItem{category: category, itemName: itemName, volumeCf: volume, sortOrder: sortOrder, active: active})
	}

	if len(errs) > 0 {
		return nil, nil, errs
	}

	categories := make([]string, 0, len(categorySet))
	for _, item := range items {
		lower := strings.ToLower(item.category)
		found := false
		for _, category := range categories {
			if strings.EqualFold(category, lower) {
				found = true
				break
			}
		}
		if !found {
			categories = append(categories, item.category)
		}
	}

	return categories, items, nil
}

func (s *Server) trackAnalyticsEvent(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, estimateID *uuid.UUID, eventName string, properties map[string]any) {
	if _, ok := allowedAnalyticsEvents[eventName]; !ok {
		return
	}
	raw, err := json.Marshal(sanitizeAnalyticsProperties(properties))
	if err != nil {
		return
	}
	_ = s.Q.InsertAnalyticsEvent(ctx, gen.InsertAnalyticsEventParams{
		TenantID:       tenantID,
		EstimateID:     estimateID,
		UserID:         userID,
		EventName:      eventName,
		PropertiesJson: raw,
	})
}
