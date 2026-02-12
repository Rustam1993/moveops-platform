package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/moveops-platform/apps/api/internal/audit"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/gen/oapi"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	defaultStorageListLimit = 25
	maxStorageListLimit     = 100
)

func (s *Server) GetStorage(w http.ResponseWriter, r *http.Request, params oapi.GetStorageParams) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	facility := strings.TrimSpace(params.Facility)
	if facility == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "facility query parameter is required", nil)
		return
	}

	limit := defaultStorageListLimit
	if params.Limit != nil {
		switch {
		case *params.Limit < 1:
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "limit must be at least 1", nil)
			return
		case *params.Limit > maxStorageListLimit:
			limit = maxStorageListLimit
		default:
			limit = *params.Limit
		}
	}

	searchQ := sanitizeOptional(params.Q)
	status := storageStatusToPtr(params.Status)

	var cursorUpdatedAt *time.Time
	var cursorJobID *uuid.UUID
	if params.Cursor != nil && strings.TrimSpace(*params.Cursor) != "" {
		decodedAt, decodedJobID, err := decodeStorageCursor(*params.Cursor)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "invalid_cursor", "cursor is invalid", nil)
			return
		}
		cursorUpdatedAt = &decodedAt
		cursorJobID = &decodedJobID
	}

	var pastDueDays *int32
	if params.PastDueDays != nil {
		if *params.PastDueDays < 0 {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "pastDueDays must be greater than or equal to 0", nil)
			return
		}
		value := int32(*params.PastDueDays)
		pastDueDays = &value
	}

	rows, err := s.Q.ListStorageRows(r.Context(), gen.ListStorageRowsParams{
		Facility:        facility,
		TenantID:        tenantID,
		SearchQ:         searchQ,
		Status:          status,
		HasDateOut:      params.HasDateOut,
		BalanceDue:      params.BalanceDue,
		HasContainers:   params.HasContainers,
		PastDueDays:     pastDueDays,
		CursorUpdatedAt: cursorUpdatedAt,
		CursorJobID:     cursorJobID,
		LimitRows:       int32(limit + 1),
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load storage rows", nil)
		return
	}

	var nextCursor *string
	if len(rows) > limit {
		cursor := encodeStorageCursor(rows[limit-1].SortUpdatedAt, rows[limit-1].SortJobID)
		nextCursor = &cursor
		rows = rows[:limit]
	}

	items := make([]oapi.StorageListItem, 0, len(rows))
	for _, row := range rows {
		var storageRecordID *openapi_types.UUID
		if row.StorageRecordID != nil {
			id := openapi_types.UUID(*row.StorageRecordID)
			storageRecordID = &id
		}
		var statusPtr *oapi.StorageListItemStatus
		if row.Status != nil {
			sv := oapi.StorageListItemStatus(*row.Status)
			statusPtr = &sv
		}
		moveType := row.MoveType
		items = append(items, oapi.StorageListItem{
			StorageRecordId:     storageRecordID,
			JobId:               row.JobID,
			JobNumber:           row.JobNumber,
			CustomerName:        row.CustomerName,
			MoveType:            &moveType,
			FromShort:           row.FromShort,
			ToShort:             row.ToShort,
			Status:              statusPtr,
			DateIn:              dateToDatePtr(row.DateIn),
			DateOut:             dateToDatePtr(row.DateOut),
			NextBillDate:        dateToDatePtr(row.NextBillDate),
			LotNumber:           row.LotNumber,
			LocationLabel:       row.LocationLabel,
			Vaults:              int(row.Vaults),
			Pads:                int(row.Pads),
			Items:               int(row.Items),
			OversizeItems:       int(row.OversizeItems),
			Volume:              int(row.Volume),
			MonthlyRateCents:    row.MonthlyRateCents,
			StorageBalanceCents: row.StorageBalanceCents,
			MoveBalanceCents:    row.MoveBalanceCents,
			Facility:            row.Facility,
		})
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.StorageListResponse{
		Items:      items,
		NextCursor: nextCursor,
		RequestId:  middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetStorageStorageRecordId(w http.ResponseWriter, r *http.Request, storageRecordId openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	s.writeStorageRecordResponse(w, r, tenantID, uuid.UUID(storageRecordId), http.StatusOK)
}

func (s *Server) PutStorageStorageRecordId(w http.ResponseWriter, r *http.Request, storageRecordId openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.UpdateStorageRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	facility := strings.TrimSpace(req.Facility)
	if facility == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "facility is required", nil)
		return
	}
	if req.DateIn != nil && req.DateOut != nil && req.DateIn.Time.After(req.DateOut.Time) {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "dateIn must be on or before dateOut", nil)
		return
	}

	targetID := uuid.UUID(storageRecordId)
	before, err := s.Q.GetStorageRecordByID(r.Context(), gen.GetStorageRecordByIDParams{
		ID:       targetID,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "storage_record_not_found", "Storage record was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load storage record", nil)
		return
	}

	updated, err := s.Q.UpdateStorageRecordByID(r.Context(), gen.UpdateStorageRecordByIDParams{
		Facility:            facility,
		Status:              string(req.Status),
		DateIn:              dateToTimePtr(req.DateIn),
		DateOut:             dateToTimePtr(req.DateOut),
		NextBillDate:        dateToTimePtr(req.NextBillDate),
		LotNumber:           sanitizeOptional(req.LotNumber),
		LocationLabel:       sanitizeOptional(req.LocationLabel),
		Vaults:              int32(req.Vaults),
		Pads:                int32(req.Pads),
		Items:               int32(req.Items),
		OversizeItems:       int32(req.OversizeItems),
		Volume:              int32(req.Volume),
		MonthlyRateCents:    req.MonthlyRateCents,
		StorageBalanceCents: req.StorageBalanceCents,
		MoveBalanceCents:    req.MoveBalanceCents,
		LastPaymentAt:       req.LastPaymentAt,
		Notes:               sanitizeOptional(req.Notes),
		ID:                  targetID,
		TenantID:            tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "storage_record_not_found", "Storage record was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update storage record", nil)
		return
	}

	storageID := updated.ID
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "storage_record.update",
		EntityType: "storage_record",
		EntityID:   &storageID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"fieldsChanged": storageRecordChangedFields(before, updated),
		},
	})

	s.writeStorageRecordResponse(w, r, tenantID, targetID, http.StatusOK)
}

func (s *Server) PostJobsJobIdStorage(w http.ResponseWriter, r *http.Request, jobId openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.CreateStorageRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	facility := strings.TrimSpace(req.Facility)
	if facility == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "facility is required", nil)
		return
	}
	if req.DateIn != nil && req.DateOut != nil && req.DateIn.Time.After(req.DateOut.Time) {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "dateIn must be on or before dateOut", nil)
		return
	}

	targetJobID := uuid.UUID(jobId)
	if _, err := s.Q.GetJobByID(r.Context(), gen.GetJobByIDParams{
		ID:       targetJobID,
		TenantID: tenantID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "job_not_found", "Job was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load job", nil)
		return
	}

	existing, err := s.Q.GetStorageRecordByJobID(r.Context(), gen.GetStorageRecordByJobIDParams{
		JobID:    targetJobID,
		TenantID: tenantID,
	})
	if err == nil {
		existingID := existing.ID
		httpx.WriteError(w, r, http.StatusConflict, "storage_record_exists", "Storage record already exists for this job", map[string]any{
			"storageRecordId": existingID,
		})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check existing storage record", nil)
		return
	}

	status := storageStatusToPtr(req.Status)
	created, err := s.Q.CreateStorageRecord(r.Context(), gen.CreateStorageRecordParams{
		TenantID:            tenantID,
		JobID:               targetJobID,
		Facility:            facility,
		Status:              status,
		DateIn:              dateToTimePtr(req.DateIn),
		DateOut:             dateToTimePtr(req.DateOut),
		NextBillDate:        dateToTimePtr(req.NextBillDate),
		LotNumber:           sanitizeOptional(req.LotNumber),
		LocationLabel:       sanitizeOptional(req.LocationLabel),
		Vaults:              intToInt32Ptr(req.Vaults),
		Pads:                intToInt32Ptr(req.Pads),
		Items:               intToInt32Ptr(req.Items),
		OversizeItems:       intToInt32Ptr(req.OversizeItems),
		Volume:              intToInt32Ptr(req.Volume),
		MonthlyRateCents:    req.MonthlyRateCents,
		StorageBalanceCents: req.StorageBalanceCents,
		MoveBalanceCents:    req.MoveBalanceCents,
		LastPaymentAt:       req.LastPaymentAt,
		Notes:               sanitizeOptional(req.Notes),
	})
	if err != nil {
		if isUniqueConstraint(err, "storage_record_tenant_job_uidx") {
			httpx.WriteError(w, r, http.StatusConflict, "storage_record_exists", "Storage record already exists for this job", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create storage record", nil)
		return
	}

	storageID := created.ID
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "storage_record.create",
		EntityType: "storage_record",
		EntityID:   &storageID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"jobId":    created.JobID,
			"facility": created.Facility,
			"status":   created.Status,
		},
	})

	s.writeStorageRecordResponse(w, r, tenantID, storageID, http.StatusCreated)
}

func (s *Server) writeStorageRecordResponse(w http.ResponseWriter, r *http.Request, tenantID, storageRecordID uuid.UUID, status int) {
	record, err := s.Q.GetStorageRecordDetailByID(r.Context(), gen.GetStorageRecordDetailByIDParams{
		ID:       storageRecordID,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "storage_record_not_found", "Storage record was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load storage record", nil)
		return
	}

	httpx.WriteJSON(w, status, oapi.StorageRecordResponse{
		Storage:   mapStorageRecord(record),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func mapStorageRecord(detail gen.GetStorageRecordDetailByIDRow) oapi.StorageRecord {
	moveType := detail.MoveType
	return oapi.StorageRecord{
		Id:                  detail.ID,
		JobId:               detail.JobID,
		JobNumber:           detail.JobNumber,
		CustomerName:        detail.CustomerName,
		MoveType:            &moveType,
		FromShort:           detail.FromShort,
		ToShort:             detail.ToShort,
		Facility:            detail.Facility,
		Status:              oapi.StorageStatus(detail.Status),
		DateIn:              dateToDatePtr(detail.DateIn),
		DateOut:             dateToDatePtr(detail.DateOut),
		NextBillDate:        dateToDatePtr(detail.NextBillDate),
		LotNumber:           detail.LotNumber,
		LocationLabel:       detail.LocationLabel,
		Vaults:              int(detail.Vaults),
		Pads:                int(detail.Pads),
		Items:               int(detail.Items),
		OversizeItems:       int(detail.OversizeItems),
		Volume:              int(detail.Volume),
		MonthlyRateCents:    detail.MonthlyRateCents,
		StorageBalanceCents: detail.StorageBalanceCents,
		MoveBalanceCents:    detail.MoveBalanceCents,
		LastPaymentAt:       detail.LastPaymentAt,
		Notes:               detail.Notes,
		CreatedAt:           detail.CreatedAt.UTC(),
		UpdatedAt:           detail.UpdatedAt.UTC(),
	}
}

func storageStatusToPtr(status *oapi.StorageStatus) *string {
	if status == nil {
		return nil
	}
	value := string(*status)
	return &value
}

func dateToDatePtr(value *time.Time) *openapi_types.Date {
	if value == nil {
		return nil
	}
	date := dateOnly(*value)
	return &date
}

func intToInt32Ptr(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func encodeStorageCursor(updatedAt time.Time, jobID uuid.UUID) string {
	payload := updatedAt.UTC().Format(time.RFC3339Nano) + "|" + jobID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeStorageCursor(raw string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decode cursor: %w", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("cursor payload is malformed")
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor updatedAt: %w", err)
	}
	jobID, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor jobId: %w", err)
	}
	return updatedAt, jobID, nil
}

func storageRecordChangedFields(before, after gen.StorageRecord) map[string]any {
	changes := map[string]any{}

	add := func(field string, beforeValue, afterValue any) {
		changes[field] = map[string]any{
			"before": beforeValue,
			"after":  afterValue,
		}
	}

	if before.Facility != after.Facility {
		add("facility", before.Facility, after.Facility)
	}
	if before.Status != after.Status {
		add("status", before.Status, after.Status)
	}
	if !timePtrEqual(before.DateIn, after.DateIn) {
		add("dateIn", formatDatePtr(before.DateIn), formatDatePtr(after.DateIn))
	}
	if !timePtrEqual(before.DateOut, after.DateOut) {
		add("dateOut", formatDatePtr(before.DateOut), formatDatePtr(after.DateOut))
	}
	if !timePtrEqual(before.NextBillDate, after.NextBillDate) {
		add("nextBillDate", formatDatePtr(before.NextBillDate), formatDatePtr(after.NextBillDate))
	}
	if !strPtrEqual(before.LotNumber, after.LotNumber) {
		add("lotNumber", before.LotNumber, after.LotNumber)
	}
	if !strPtrEqual(before.LocationLabel, after.LocationLabel) {
		add("locationLabel", before.LocationLabel, after.LocationLabel)
	}
	if before.Vaults != after.Vaults {
		add("vaults", before.Vaults, after.Vaults)
	}
	if before.Pads != after.Pads {
		add("pads", before.Pads, after.Pads)
	}
	if before.Items != after.Items {
		add("items", before.Items, after.Items)
	}
	if before.OversizeItems != after.OversizeItems {
		add("oversizeItems", before.OversizeItems, after.OversizeItems)
	}
	if before.Volume != after.Volume {
		add("volume", before.Volume, after.Volume)
	}
	if !int64PtrEqual(before.MonthlyRateCents, after.MonthlyRateCents) {
		add("monthlyRateCents", before.MonthlyRateCents, after.MonthlyRateCents)
	}
	if before.StorageBalanceCents != after.StorageBalanceCents {
		add("storageBalanceCents", before.StorageBalanceCents, after.StorageBalanceCents)
	}
	if before.MoveBalanceCents != after.MoveBalanceCents {
		add("moveBalanceCents", before.MoveBalanceCents, after.MoveBalanceCents)
	}
	if !timePtrEqual(before.LastPaymentAt, after.LastPaymentAt) {
		add("lastPaymentAt", formatTimePtr(before.LastPaymentAt), formatTimePtr(after.LastPaymentAt))
	}
	if !strPtrEqual(before.Notes, after.Notes) {
		add("notesChanged", before.Notes != nil, after.Notes != nil)
	}

	return changes
}

func formatDatePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format("2006-01-02")
	return &formatted
}

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
