package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/gen/oapi"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	defaultListLimit = 25
	maxListLimit     = 100
)

func (s *Server) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	allowedEstimates, err := s.Q.UserHasPermission(r.Context(), gen.UserHasPermissionParams{
		UserID:     userID,
		TenantID:   tenantID,
		Permission: "estimates.read",
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Permission check failed", nil)
		return
	}
	allowedJobs, err := s.Q.UserHasPermission(r.Context(), gen.UserHasPermissionParams{
		UserID:     userID,
		TenantID:   tenantID,
		Permission: "jobs.read",
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Permission check failed", nil)
		return
	}
	allowedStorage, err := s.Q.UserHasPermission(r.Context(), gen.UserHasPermissionParams{
		UserID:     userID,
		TenantID:   tenantID,
		Permission: "storage.read",
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Permission check failed", nil)
		return
	}

	var openEstimatesCount *int64
	if allowedEstimates {
		count, err := s.Q.CountOpenEstimates(r.Context(), tenantID)
		if err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimates count", nil)
			return
		}
		openEstimatesCount = &count
	}

	var upcomingJobsCount *int64
	if allowedJobs {
		count, err := s.Q.CountUpcomingJobs(r.Context(), tenantID)
		if err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load jobs count", nil)
			return
		}
		upcomingJobsCount = &count
	}

	var storageRecordsCount *int64
	if allowedStorage {
		count, err := s.Q.CountStorageRecords(r.Context(), tenantID)
		if err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load storage count", nil)
			return
		}
		storageRecordsCount = &count
	}

	var openEstimatesCountInt *int
	if openEstimatesCount != nil {
		v := int(*openEstimatesCount)
		openEstimatesCountInt = &v
	}
	var upcomingJobsCountInt *int
	if upcomingJobsCount != nil {
		v := int(*upcomingJobsCount)
		upcomingJobsCountInt = &v
	}
	var storageRecordsCountInt *int
	if storageRecordsCount != nil {
		v := int(*storageRecordsCount)
		storageRecordsCountInt = &v
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.DashboardSummaryResponse{
		Allowed: struct {
			Estimates bool `json:"estimates"`
			Jobs      bool `json:"jobs"`
			Storage   bool `json:"storage"`
		}{
			Estimates: allowedEstimates,
			Jobs:      allowedJobs,
			Storage:   allowedStorage,
		},
		OpenEstimatesCount:  openEstimatesCountInt,
		UpcomingJobsCount:   upcomingJobsCountInt,
		StorageRecordsCount: storageRecordsCountInt,
		RequestId:           middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetJobs(w http.ResponseWriter, r *http.Request, params oapi.GetJobsParams) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	limit := clampLimit(params.Limit)

	searchQ := sanitizeOptional(params.Q)
	var status *string
	if params.Status != nil {
		v := strings.TrimSpace(string(*params.Status))
		if v != "" {
			status = &v
		}
	}
	var jobType *string
	if params.JobType != nil {
		v := strings.TrimSpace(string(*params.JobType))
		if v != "" {
			jobType = &v
		}
	}

	var scheduledFrom *time.Time
	if params.ScheduledFrom != nil {
		v := dateOnly(params.ScheduledFrom.Time).Time
		scheduledFrom = &v
	}
	var scheduledTo *time.Time
	if params.ScheduledTo != nil {
		v := dateOnly(params.ScheduledTo.Time).Time
		scheduledTo = &v
	}

	var cursorCreatedAt *time.Time
	var cursorID *uuid.UUID
	if params.Cursor != nil && strings.TrimSpace(*params.Cursor) != "" {
		at, id, err := decodeListCursor(*params.Cursor)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "invalid_cursor", "cursor is invalid", nil)
			return
		}
		cursorCreatedAt = &at
		cursorID = &id
	}

	rows, err := s.Q.ListJobs(r.Context(), gen.ListJobsParams{
		TenantID:        tenantID,
		Status:          status,
		JobType:         jobType,
		Scheduled:       params.Scheduled,
		ScheduledFrom:   scheduledFrom,
		ScheduledTo:     scheduledTo,
		SearchQ:         searchQ,
		CursorCreatedAt: cursorCreatedAt,
		CursorJobID:     cursorID,
		LimitRows:       int32(limit + 1),
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load jobs", nil)
		return
	}

	var nextCursor *string
	if len(rows) > limit {
		cursor := encodeListCursor(rows[limit-1].SortCreatedAt, rows[limit-1].SortJobID)
		nextCursor = &cursor
		rows = rows[:limit]
	}

	items := make([]oapi.JobListItem, 0, len(rows))
	for _, row := range rows {
		var scheduledDate *openapi_types.Date
		if row.ScheduledDate != nil {
			scheduledDate = &openapi_types.Date{Time: dateOnly(*row.ScheduledDate).Time}
		}
		var pickupTime *string
		if row.PickupTime != nil && strings.TrimSpace(*row.PickupTime) != "" {
			v := strings.TrimSpace(*row.PickupTime)
			pickupTime = &v
		}

		items = append(items, oapi.JobListItem{
			JobId:            row.JobID,
			JobNumber:        row.JobNumber,
			Status:           oapi.JobListItemStatus(row.Status),
			ScheduledDate:    scheduledDate,
			PickupTime:       pickupTime,
			CustomerName:     row.CustomerName,
			OriginShort:      row.OriginShort,
			DestinationShort: row.DestinationShort,
			HasStorage:       row.HasStorage,
			BalanceDueCents:  row.BalanceDueCents,
			CreatedAt:        row.CreatedAt.UTC(),
			UpdatedAt:        row.UpdatedAt.UTC(),
		})
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.JobListResponse{
		Items:      items,
		NextCursor: nextCursor,
		RequestId:  middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetEstimates(w http.ResponseWriter, r *http.Request, params oapi.GetEstimatesParams) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	limit := clampLimit(params.Limit)
	searchQ := sanitizeOptional(params.Q)
	var status *string
	if params.Status != nil {
		v := strings.TrimSpace(string(*params.Status))
		if v != "" {
			status = &v
		}
	}

	var cursorCreatedAt *time.Time
	var cursorID *uuid.UUID
	if params.Cursor != nil && strings.TrimSpace(*params.Cursor) != "" {
		at, id, err := decodeListCursor(*params.Cursor)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "invalid_cursor", "cursor is invalid", nil)
			return
		}
		cursorCreatedAt = &at
		cursorID = &id
	}

	rows, err := s.Q.ListEstimates(r.Context(), gen.ListEstimatesParams{
		TenantID:         tenantID,
		Status:           status,
		SearchQ:          searchQ,
		CursorCreatedAt:  cursorCreatedAt,
		CursorEstimateID: cursorID,
		LimitRows:        int32(limit + 1),
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimates", nil)
		return
	}

	var nextCursor *string
	if len(rows) > limit {
		cursor := encodeListCursor(rows[limit-1].SortCreatedAt, rows[limit-1].SortEstimateID)
		nextCursor = &cursor
		rows = rows[:limit]
	}

	items := make([]oapi.EstimateListItem, 0, len(rows))
	for _, row := range rows {
		var convertedJobID *openapi_types.UUID
		if row.ConvertedJobID != nil {
			id := openapi_types.UUID(*row.ConvertedJobID)
			convertedJobID = &id
		}
		email := openapi_types.Email(row.Email)
		primaryPhone := row.PrimaryPhone
		items = append(items, oapi.EstimateListItem{
			EstimateId:     row.EstimateID,
			EstimateNumber: row.EstimateNumber,
			CustomerName:   row.CustomerName,
			Email:          &email,
			PrimaryPhone:   &primaryPhone,
			Status:         oapi.EstimateListItemStatus(row.Status),
			MoveDate:       openapi_types.Date{Time: dateOnly(row.MoveDate).Time},
			ConvertedJobId: convertedJobID,
			CreatedAt:      row.CreatedAt.UTC(),
			UpdatedAt:      row.UpdatedAt.UTC(),
		})
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateListResponse{
		Items:      items,
		NextCursor: nextCursor,
		RequestId:  middleware.RequestIDFromContext(r.Context()),
	})
}

func clampLimit(raw *int) int {
	limit := defaultListLimit
	if raw == nil {
		return limit
	}
	switch {
	case *raw < 1:
		return 1
	case *raw > maxListLimit:
		return maxListLimit
	default:
		return *raw
	}
}

func encodeListCursor(createdAt time.Time, id uuid.UUID) string {
	payload := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeListCursor(raw string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decode cursor: %w", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("cursor payload is malformed")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor createdAt: %w", err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor id: %w", err)
	}
	return createdAt, id, nil
}

// requireActorIDsNoAudit mirrors requireActorIDs but returns concrete IDs for local-only logic.
