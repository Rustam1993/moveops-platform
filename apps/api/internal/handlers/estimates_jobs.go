package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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

func (s *Server) PostEstimates(w http.ResponseWriter, r *http.Request, params oapi.PostEstimatesParams) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.CreateEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	if idempotencyKey == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required", nil)
		return
	}
	idempotencyKeyPtr := &idempotencyKey
	payloadHash := hashCreateEstimateRequest(req)

	existing, err := s.Q.GetEstimateByIdempotencyKey(r.Context(), gen.GetEstimateByIdempotencyKeyParams{
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKeyPtr,
	})
	if err == nil {
		if existing.IdempotencyPayloadHash != nil && *existing.IdempotencyPayloadHash == payloadHash {
			s.writeEstimateResponse(w, r, tenantID, existing.ID, http.StatusOK)
			return
		}
		httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "Idempotency key was already used with a different payload", nil)
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check idempotency key", nil)
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	qtx := s.Q.WithTx(tx)

	existing, err = qtx.GetEstimateByIdempotencyKey(r.Context(), gen.GetEstimateByIdempotencyKeyParams{
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKeyPtr,
	})
	if err == nil {
		if existing.IdempotencyPayloadHash != nil && *existing.IdempotencyPayloadHash == payloadHash {
			if err := tx.Commit(r.Context()); err != nil {
				httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to finalize idempotent request", nil)
				return
			}
			s.writeEstimateResponse(w, r, tenantID, existing.ID, http.StatusOK)
			return
		}
		httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "Idempotency key was already used with a different payload", nil)
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check idempotency key", nil)
		return
	}

	counter, err := qtx.IncrementTenantCounter(r.Context(), gen.IncrementTenantCounterParams{
		TenantID:    tenantID,
		CounterType: "estimate",
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to allocate estimate number", nil)
		return
	}

	estimateNumber := fmt.Sprintf("E-%06d", counter)
	firstName, lastName := splitName(req.CustomerName)

	customer, err := qtx.CreateCustomerForEstimate(r.Context(), gen.CreateCustomerForEstimateParams{
		TenantID:  tenantID,
		FirstName: firstName,
		LastName:  lastName,
		Email:     ptr(strings.TrimSpace(string(req.Email))),
		Phone:     ptr(strings.TrimSpace(req.PrimaryPhone)),
		CreatedBy: &userID,
		UpdatedBy: &userID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create customer", nil)
		return
	}

	estimate, err := qtx.CreateEstimate(r.Context(), gen.CreateEstimateParams{
		TenantID:                tenantID,
		EstimateNumber:          estimateNumber,
		CustomerID:              customer.ID,
		Status:                  string(oapi.Draft),
		CustomerName:            strings.TrimSpace(req.CustomerName),
		PrimaryPhone:            strings.TrimSpace(req.PrimaryPhone),
		SecondaryPhone:          sanitizeOptional(req.SecondaryPhone),
		Email:                   string(req.Email),
		OriginAddressLine1:      strings.TrimSpace(req.OriginAddressLine1),
		OriginCity:              strings.TrimSpace(req.OriginCity),
		OriginState:             strings.TrimSpace(req.OriginState),
		OriginPostalCode:        strings.TrimSpace(req.OriginPostalCode),
		DestinationAddressLine1: strings.TrimSpace(req.DestinationAddressLine1),
		DestinationCity:         strings.TrimSpace(req.DestinationCity),
		DestinationState:        strings.TrimSpace(req.DestinationState),
		DestinationPostalCode:   strings.TrimSpace(req.DestinationPostalCode),
		MoveDate:                req.MoveDate.Time,
		PickupTime:              sanitizeOptional(req.PickupTime),
		LeadSource:              strings.TrimSpace(req.LeadSource),
		MoveSize:                sanitizeOptional(req.MoveSize),
		LocationType:            sanitizeOptional(req.LocationType),
		EstimatedTotalCents:     req.EstimatedTotalCents,
		DepositCents:            req.DepositCents,
		Notes:                   sanitizeOptional(req.Notes),
		IdempotencyKey:          idempotencyKeyPtr,
		IdempotencyPayloadHash:  &payloadHash,
		CreatedBy:               &userID,
		UpdatedBy:               &userID,
	})
	if err != nil {
		if isUniqueConstraint(err, "estimates_tenant_idempotency_uidx") {
			existing, lookupErr := s.Q.GetEstimateByIdempotencyKey(r.Context(), gen.GetEstimateByIdempotencyKeyParams{
				TenantID:       tenantID,
				IdempotencyKey: idempotencyKeyPtr,
			})
			if lookupErr == nil && existing.IdempotencyPayloadHash != nil && *existing.IdempotencyPayloadHash == payloadHash {
				s.writeEstimateResponse(w, r, tenantID, existing.ID, http.StatusOK)
				return
			}
			httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "Idempotency key was already used with a different payload", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create estimate", nil)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit estimate", nil)
		return
	}

	estimateID := estimate.ID
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "estimate.create",
		EntityType: "estimate",
		EntityID:   &estimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"estimateNumber": estimateNumber,
			"status":         estimate.Status,
			"leadSource":     estimate.LeadSource,
			"moveDate":       estimate.MoveDate.Format("2006-01-02"),
		},
	})

	s.writeEstimateResponse(w, r, tenantID, estimate.ID, http.StatusCreated)
}

func (s *Server) GetEstimatesEstimateId(w http.ResponseWriter, r *http.Request, estimateId openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	s.writeEstimateResponse(w, r, tenantID, uuid.UUID(estimateId), http.StatusOK)
}

func (s *Server) PatchEstimatesEstimateId(w http.ResponseWriter, r *http.Request, estimateId openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.UpdateEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	if req.CustomerName == nil && req.PrimaryPhone == nil && req.SecondaryPhone == nil && req.Email == nil &&
		req.OriginAddressLine1 == nil && req.OriginCity == nil && req.OriginState == nil && req.OriginPostalCode == nil &&
		req.DestinationAddressLine1 == nil && req.DestinationCity == nil && req.DestinationState == nil && req.DestinationPostalCode == nil &&
		req.MoveDate == nil && req.PickupTime == nil && req.LeadSource == nil && req.MoveSize == nil && req.LocationType == nil &&
		req.EstimatedTotalCents == nil && req.DepositCents == nil && req.Notes == nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "At least one field must be provided", nil)
		return
	}

	targetEstimateID := uuid.UUID(estimateId)
	before, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: targetEstimateID, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	updated, err := s.Q.UpdateEstimate(r.Context(), gen.UpdateEstimateParams{
		CustomerName:            sanitizeOptional(req.CustomerName),
		PrimaryPhone:            sanitizeOptional(req.PrimaryPhone),
		SecondaryPhone:          sanitizeOptional(req.SecondaryPhone),
		Email:                   emailToStringPtr(req.Email),
		OriginAddressLine1:      sanitizeOptional(req.OriginAddressLine1),
		OriginCity:              sanitizeOptional(req.OriginCity),
		OriginState:             sanitizeOptional(req.OriginState),
		OriginPostalCode:        sanitizeOptional(req.OriginPostalCode),
		DestinationAddressLine1: sanitizeOptional(req.DestinationAddressLine1),
		DestinationCity:         sanitizeOptional(req.DestinationCity),
		DestinationState:        sanitizeOptional(req.DestinationState),
		DestinationPostalCode:   sanitizeOptional(req.DestinationPostalCode),
		MoveDate:                dateToTimePtr(req.MoveDate),
		PickupTime:              sanitizeOptional(req.PickupTime),
		LeadSource:              sanitizeOptional(req.LeadSource),
		MoveSize:                sanitizeOptional(req.MoveSize),
		LocationType:            sanitizeOptional(req.LocationType),
		EstimatedTotalCents:     req.EstimatedTotalCents,
		DepositCents:            req.DepositCents,
		Notes:                   sanitizeOptional(req.Notes),
		UpdatedBy:               &userID,
		ID:                      targetEstimateID,
		TenantID:                tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update estimate", nil)
		return
	}

	firstName, lastName := splitName(updated.CustomerName)
	_, err = s.Q.UpdateCustomerForEstimate(r.Context(), gen.UpdateCustomerForEstimateParams{
		FirstName: &firstName,
		LastName:  &lastName,
		Email:     &updated.Email,
		Phone:     &updated.PrimaryPhone,
		UpdatedBy: &userID,
		ID:        updated.CustomerID,
		TenantID:  tenantID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update customer", nil)
		return
	}

	estimateID := updated.ID
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "estimate.update",
		EntityType: "estimate",
		EntityID:   &estimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"fieldsChanged": estimateChangedFields(before, updated),
		},
	})

	s.writeEstimateResponse(w, r, tenantID, updated.ID, http.StatusOK)
}

func (s *Server) PostEstimatesEstimateIdConvert(w http.ResponseWriter, r *http.Request, estimateId openapi_types.UUID, params oapi.PostEstimatesEstimateIdConvertParams) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	if idempotencyKey == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required", nil)
		return
	}
	idempotencyKeyPtr := &idempotencyKey
	targetEstimateID := uuid.UUID(estimateId)

	if jobByKey, err := s.Q.GetJobByConvertIdempotencyKey(r.Context(), gen.GetJobByConvertIdempotencyKeyParams{
		TenantID:              tenantID,
		ConvertIdempotencyKey: idempotencyKeyPtr,
	}); err == nil {
		if jobByKey.EstimateID == nil || *jobByKey.EstimateID != targetEstimateID {
			httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "Idempotency key was already used for a different estimate conversion", nil)
			return
		}
		s.writeJobResponse(w, r, tenantID, jobByKey.ID, http.StatusOK)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check conversion idempotency", nil)
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.Q.WithTx(tx)

	if jobByKey, err := qtx.GetJobByConvertIdempotencyKey(r.Context(), gen.GetJobByConvertIdempotencyKeyParams{
		TenantID:              tenantID,
		ConvertIdempotencyKey: idempotencyKeyPtr,
	}); err == nil {
		if jobByKey.EstimateID == nil || *jobByKey.EstimateID != targetEstimateID {
			httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "Idempotency key was already used for a different estimate conversion", nil)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to finalize idempotent conversion", nil)
			return
		}
		s.writeJobResponse(w, r, tenantID, jobByKey.ID, http.StatusOK)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check conversion idempotency", nil)
		return
	}

	estimate, err := qtx.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: targetEstimateID, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	var (
		jobID       uuid.UUID
		jobNumber   string
		statusCode  = http.StatusCreated
		createdNew  = false
		estimateRef = &targetEstimateID
	)

	existingJob, err := qtx.GetJobByEstimateID(r.Context(), gen.GetJobByEstimateIDParams{TenantID: tenantID, EstimateID: estimateRef})
	if err == nil {
		jobID = existingJob.ID
		jobNumber = existingJob.JobNumber
		statusCode = http.StatusOK
	} else if errors.Is(err, pgx.ErrNoRows) {
		counter, counterErr := qtx.IncrementTenantCounter(r.Context(), gen.IncrementTenantCounterParams{
			TenantID:    tenantID,
			CounterType: "job",
		})
		if counterErr != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to allocate job number", nil)
			return
		}

		jobNumber = fmt.Sprintf("J-%06d", counter)
		scheduledDate := estimate.MoveDate
		job, createErr := qtx.CreateJob(r.Context(), gen.CreateJobParams{
			TenantID:              tenantID,
			JobNumber:             jobNumber,
			EstimateID:            estimateRef,
			CustomerID:            estimate.CustomerID,
			Status:                string(oapi.JobStatusBooked),
			ScheduledDate:         &scheduledDate,
			PickupTime:            estimate.PickupTime,
			ConvertIdempotencyKey: idempotencyKeyPtr,
			CreatedBy:             &userID,
			UpdatedBy:             &userID,
		})
		if createErr != nil {
			switch {
			case isUniqueConstraint(createErr, "jobs_tenant_estimate_uidx"):
				existingJob, lookupErr := qtx.GetJobByEstimateID(r.Context(), gen.GetJobByEstimateIDParams{TenantID: tenantID, EstimateID: estimateRef})
				if lookupErr != nil {
					httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load converted job", nil)
					return
				}
				jobID = existingJob.ID
				jobNumber = existingJob.JobNumber
				statusCode = http.StatusOK
			case isUniqueConstraint(createErr, "jobs_tenant_convert_idempotency_uidx"):
				existingByKey, lookupErr := qtx.GetJobByConvertIdempotencyKey(r.Context(), gen.GetJobByConvertIdempotencyKeyParams{
					TenantID:              tenantID,
					ConvertIdempotencyKey: idempotencyKeyPtr,
				})
				if lookupErr == nil && existingByKey.EstimateID != nil && *existingByKey.EstimateID == targetEstimateID {
					jobID = existingByKey.ID
					jobNumber = existingByKey.JobNumber
					statusCode = http.StatusOK
				} else {
					httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "Idempotency key was already used for a different estimate conversion", nil)
					return
				}
			default:
				httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create job", nil)
				return
			}
		} else {
			jobID = job.ID
			createdNew = true
		}
	} else {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check existing conversion", nil)
		return
	}

	_, _ = qtx.MarkEstimateConverted(r.Context(), gen.MarkEstimateConvertedParams{
		UpdatedBy: &userID,
		ID:        targetEstimateID,
		TenantID:  tenantID,
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit conversion", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "estimate.convert_to_job",
		EntityType: "estimate",
		EntityID:   &targetEstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"jobId":     jobID,
			"jobNumber": jobNumber,
			"created":   createdNew,
		},
	})

	s.writeJobResponse(w, r, tenantID, jobID, statusCode)
}

func (s *Server) GetCalendar(w http.ResponseWriter, r *http.Request, params oapi.GetCalendarParams) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	fromDate := dateOnly(params.From.Time).Time
	toDate := dateOnly(params.To.Time).Time
	if !toDate.After(fromDate) {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "`to` must be after `from`", nil)
		return
	}

	var phase *string
	if params.Phase != nil {
		v := string(*params.Phase)
		phase = &v
	}
	var jobType *string
	if params.JobType != nil {
		v := string(*params.JobType)
		jobType = &v
	}

	rows, err := s.Q.ListCalendarJobs(r.Context(), gen.ListCalendarJobsParams{
		TenantID: tenantID,
		FromDate: fromDate,
		ToDate:   toDate,
		Phase:    phase,
		JobType:  jobType,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load calendar jobs", nil)
		return
	}

	jobs := make([]oapi.CalendarJobCard, 0, len(rows))
	for _, row := range rows {
		if row.ScheduledDate == nil {
			continue
		}
		jobs = append(jobs, oapi.CalendarJobCard{
			JobId:            row.JobID,
			JobNumber:        row.JobNumber,
			ScheduledDate:    dateOnly(*row.ScheduledDate),
			PickupTime:       row.PickupTime,
			CustomerName:     row.CustomerName,
			OriginShort:      row.OriginShort,
			DestinationShort: row.DestinationShort,
			Status:           oapi.CalendarJobCardStatus(row.Status),
			HasStorage:       row.HasStorage,
			BalanceDueCents:  row.BalanceDueCents,
		})
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.CalendarResponse{
		Jobs:      jobs,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetJobsJobId(w http.ResponseWriter, r *http.Request, jobId openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	s.writeJobResponse(w, r, tenantID, uuid.UUID(jobId), http.StatusOK)
}

func (s *Server) PatchJobsJobId(w http.ResponseWriter, r *http.Request, jobId openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}
	if req.ScheduledDate == nil && req.PickupTime == nil && req.Status == nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "At least one field must be provided", nil)
		return
	}

	targetJobID := uuid.UUID(jobId)
	before, err := s.Q.GetJobByID(r.Context(), gen.GetJobByIDParams{ID: targetJobID, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "job_not_found", "Job was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load job", nil)
		return
	}

	updated, err := s.Q.UpdateJobScheduleStatus(r.Context(), gen.UpdateJobScheduleStatusParams{
		ScheduledDate: dateToTimePtr(req.ScheduledDate),
		PickupTime:    sanitizeOptional(req.PickupTime),
		Status:        updateJobStatusToPtr(req.Status),
		UpdatedBy:     &userID,
		ID:            targetJobID,
		TenantID:      tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "job_not_found", "Job was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update job", nil)
		return
	}

	jobID := updated.ID
	scheduleChanged := !timePtrEqual(before.ScheduledDate, updated.ScheduledDate) || !strPtrEqual(before.PickupTime, updated.PickupTime)
	phaseChanged := before.Status != updated.Status

	if scheduleChanged {
		_ = s.Audit.Log(r.Context(), audit.Entry{
			TenantID:   tenantID,
			UserID:     &userID,
			Action:     "job.schedule_update",
			EntityType: "job",
			EntityID:   &jobID,
			RequestID:  middleware.RequestIDFromContext(r.Context()),
			Metadata: map[string]any{
				"before": compactJobSchedule(before),
				"after":  compactJobSchedule(updated),
			},
		})
	}
	if phaseChanged {
		_ = s.Audit.Log(r.Context(), audit.Entry{
			TenantID:   tenantID,
			UserID:     &userID,
			Action:     "job.phase_update",
			EntityType: "job",
			EntityID:   &jobID,
			RequestID:  middleware.RequestIDFromContext(r.Context()),
			Metadata: map[string]any{
				"before": before.Status,
				"after":  updated.Status,
			},
		})
	}

	s.writeJobResponse(w, r, tenantID, updated.ID, http.StatusOK)
}

func (s *Server) writeEstimateResponse(w http.ResponseWriter, r *http.Request, tenantID, estimateID uuid.UUID, status int) {
	detail, err := s.Q.GetEstimateDetailByID(r.Context(), gen.GetEstimateDetailByIDParams{
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

	httpx.WriteJSON(w, status, oapi.EstimateResponse{
		Estimate:  mapEstimateDetail(detail),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) writeJobResponse(w http.ResponseWriter, r *http.Request, tenantID, jobID uuid.UUID, status int) {
	detail, err := s.Q.GetJobDetailByID(r.Context(), gen.GetJobDetailByIDParams{
		ID:       jobID,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "job_not_found", "Job was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load job", nil)
		return
	}

	httpx.WriteJSON(w, status, oapi.JobResponse{
		Job:       mapJobDetail(detail),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func mapEstimateDetail(detail gen.GetEstimateDetailByIDRow) oapi.Estimate {
	moveDate := dateOnly(detail.MoveDate)
	var convertedJobID *openapi_types.UUID
	if detail.ConvertedJobID != nil {
		id := openapi_types.UUID(*detail.ConvertedJobID)
		convertedJobID = &id
	}

	return oapi.Estimate{
		Id:                      detail.ID,
		TenantId:                detail.TenantID,
		EstimateNumber:          detail.EstimateNumber,
		CustomerId:              detail.CustomerID,
		CustomerName:            detail.CustomerName,
		PrimaryPhone:            detail.PrimaryPhone,
		SecondaryPhone:          detail.SecondaryPhone,
		Email:                   openapi_types.Email(detail.Email),
		Status:                  oapi.EstimateStatus(detail.Status),
		OriginAddressLine1:      detail.OriginAddressLine1,
		OriginCity:              detail.OriginCity,
		OriginState:             detail.OriginState,
		OriginPostalCode:        detail.OriginPostalCode,
		DestinationAddressLine1: detail.DestinationAddressLine1,
		DestinationCity:         detail.DestinationCity,
		DestinationState:        detail.DestinationState,
		DestinationPostalCode:   detail.DestinationPostalCode,
		MoveDate:                moveDate,
		PickupTime:              detail.PickupTime,
		LeadSource:              detail.LeadSource,
		MoveSize:                detail.MoveSize,
		LocationType:            detail.LocationType,
		EstimatedTotalCents:     detail.EstimatedTotalCents,
		DepositCents:            detail.DepositCents,
		Notes:                   detail.Notes,
		ConvertedJobId:          convertedJobID,
		CreatedAt:               detail.CreatedAt.UTC(),
		UpdatedAt:               detail.UpdatedAt.UTC(),
	}
}

func mapJobDetail(detail gen.GetJobDetailByIDRow) oapi.Job {
	var estimateID *openapi_types.UUID
	if detail.EstimateID != nil {
		id := openapi_types.UUID(*detail.EstimateID)
		estimateID = &id
	}
	var scheduledDate *openapi_types.Date
	if detail.ScheduledDate != nil {
		d := dateOnly(*detail.ScheduledDate)
		scheduledDate = &d
	}

	customerName := strings.TrimSpace(detail.FirstName + " " + detail.LastName)
	if customerName == "" {
		customerName = "Customer"
	}

	return oapi.Job{
		Id:            detail.ID,
		TenantId:      detail.TenantID,
		JobNumber:     detail.JobNumber,
		CustomerId:    detail.CustomerID,
		CustomerName:  customerName,
		PrimaryPhone:  detail.Phone,
		Email:         openapi_types.Email(detail.Email),
		EstimateId:    estimateID,
		Status:        oapi.JobStatus(detail.Status),
		ScheduledDate: scheduledDate,
		PickupTime:    detail.PickupTime,
		CreatedAt:     detail.CreatedAt.UTC(),
		UpdatedAt:     detail.UpdatedAt.UTC(),
	}
}

func hashCreateEstimateRequest(req oapi.CreateEstimateRequest) string {
	type fingerprint struct {
		CustomerName            string  `json:"customerName"`
		PrimaryPhone            string  `json:"primaryPhone"`
		SecondaryPhone          *string `json:"secondaryPhone,omitempty"`
		Email                   string  `json:"email"`
		OriginAddressLine1      string  `json:"originAddressLine1"`
		OriginCity              string  `json:"originCity"`
		OriginState             string  `json:"originState"`
		OriginPostalCode        string  `json:"originPostalCode"`
		DestinationAddressLine1 string  `json:"destinationAddressLine1"`
		DestinationCity         string  `json:"destinationCity"`
		DestinationState        string  `json:"destinationState"`
		DestinationPostalCode   string  `json:"destinationPostalCode"`
		MoveDate                string  `json:"moveDate"`
		PickupTime              *string `json:"pickupTime,omitempty"`
		LeadSource              string  `json:"leadSource"`
		MoveSize                *string `json:"moveSize,omitempty"`
		LocationType            *string `json:"locationType,omitempty"`
		EstimatedTotalCents     *int64  `json:"estimatedTotalCents,omitempty"`
		DepositCents            *int64  `json:"depositCents,omitempty"`
		Notes                   *string `json:"notes,omitempty"`
	}

	payload := fingerprint{
		CustomerName:            strings.TrimSpace(req.CustomerName),
		PrimaryPhone:            strings.TrimSpace(req.PrimaryPhone),
		SecondaryPhone:          sanitizeOptional(req.SecondaryPhone),
		Email:                   strings.TrimSpace(string(req.Email)),
		OriginAddressLine1:      strings.TrimSpace(req.OriginAddressLine1),
		OriginCity:              strings.TrimSpace(req.OriginCity),
		OriginState:             strings.TrimSpace(req.OriginState),
		OriginPostalCode:        strings.TrimSpace(req.OriginPostalCode),
		DestinationAddressLine1: strings.TrimSpace(req.DestinationAddressLine1),
		DestinationCity:         strings.TrimSpace(req.DestinationCity),
		DestinationState:        strings.TrimSpace(req.DestinationState),
		DestinationPostalCode:   strings.TrimSpace(req.DestinationPostalCode),
		MoveDate:                req.MoveDate.Time.Format("2006-01-02"),
		PickupTime:              sanitizeOptional(req.PickupTime),
		LeadSource:              strings.TrimSpace(req.LeadSource),
		MoveSize:                sanitizeOptional(req.MoveSize),
		LocationType:            sanitizeOptional(req.LocationType),
		EstimatedTotalCents:     req.EstimatedTotalCents,
		DepositCents:            req.DepositCents,
		Notes:                   sanitizeOptional(req.Notes),
	}

	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func estimateChangedFields(before, after gen.Estimate) []string {
	fields := make([]string, 0, 20)
	if before.CustomerName != after.CustomerName {
		fields = append(fields, "customerName")
	}
	if before.PrimaryPhone != after.PrimaryPhone {
		fields = append(fields, "primaryPhone")
	}
	if !strPtrEqual(before.SecondaryPhone, after.SecondaryPhone) {
		fields = append(fields, "secondaryPhone")
	}
	if before.Email != after.Email {
		fields = append(fields, "email")
	}
	if before.OriginAddressLine1 != after.OriginAddressLine1 {
		fields = append(fields, "originAddressLine1")
	}
	if before.OriginCity != after.OriginCity {
		fields = append(fields, "originCity")
	}
	if before.OriginState != after.OriginState {
		fields = append(fields, "originState")
	}
	if before.OriginPostalCode != after.OriginPostalCode {
		fields = append(fields, "originPostalCode")
	}
	if before.DestinationAddressLine1 != after.DestinationAddressLine1 {
		fields = append(fields, "destinationAddressLine1")
	}
	if before.DestinationCity != after.DestinationCity {
		fields = append(fields, "destinationCity")
	}
	if before.DestinationState != after.DestinationState {
		fields = append(fields, "destinationState")
	}
	if before.DestinationPostalCode != after.DestinationPostalCode {
		fields = append(fields, "destinationPostalCode")
	}
	if !before.MoveDate.Equal(after.MoveDate) {
		fields = append(fields, "moveDate")
	}
	if !strPtrEqual(before.PickupTime, after.PickupTime) {
		fields = append(fields, "pickupTime")
	}
	if before.LeadSource != after.LeadSource {
		fields = append(fields, "leadSource")
	}
	if !strPtrEqual(before.MoveSize, after.MoveSize) {
		fields = append(fields, "moveSize")
	}
	if !strPtrEqual(before.LocationType, after.LocationType) {
		fields = append(fields, "locationType")
	}
	if !int64PtrEqual(before.EstimatedTotalCents, after.EstimatedTotalCents) {
		fields = append(fields, "estimatedTotalCents")
	}
	if !int64PtrEqual(before.DepositCents, after.DepositCents) {
		fields = append(fields, "depositCents")
	}
	if !strPtrEqual(before.Notes, after.Notes) {
		fields = append(fields, "notes")
	}
	return fields
}

func compactJobSchedule(job gen.Job) map[string]any {
	out := map[string]any{}
	if job.ScheduledDate != nil {
		out["scheduledDate"] = job.ScheduledDate.Format("2006-01-02")
	}
	if job.PickupTime != nil {
		out["pickupTime"] = *job.PickupTime
	}
	return out
}

func requireActorIDs(w http.ResponseWriter, r *http.Request) (middleware.Actor, uuid.UUID, uuid.UUID, bool) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return middleware.Actor{}, uuid.Nil, uuid.Nil, false
	}
	tenantID, err := uuid.Parse(actor.TenantID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid tenant", nil)
		return middleware.Actor{}, uuid.Nil, uuid.Nil, false
	}
	userID, err := uuid.Parse(actor.UserID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid user", nil)
		return middleware.Actor{}, uuid.Nil, uuid.Nil, false
	}
	return actor, tenantID, userID, true
}

func splitName(full string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(full))
	switch len(parts) {
	case 0:
		return "Customer", "Record"
	case 1:
		return parts[0], "Record"
	default:
		return parts[0], strings.Join(parts[1:], " ")
	}
}

func sanitizeOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func emailToStringPtr(value *openapi_types.Email) *string {
	if value == nil {
		return nil
	}
	v := strings.TrimSpace(string(*value))
	if v == "" {
		return nil
	}
	return &v
}

func dateToTimePtr(value *openapi_types.Date) *time.Time {
	if value == nil {
		return nil
	}
	t := dateOnly(value.Time).Time
	return &t
}

func dateOnly(t time.Time) openapi_types.Date {
	utc := t.UTC()
	return openapi_types.Date{Time: time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)}
}

func updateJobStatusToPtr(value *oapi.UpdateJobRequestStatus) *string {
	if value == nil {
		return nil
	}
	v := string(*value)
	return &v
}

func isUniqueConstraint(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if constraint == "" || pgErr.ConstraintName == constraint {
			return true
		}
	}
	return false
}

func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func int64PtrEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func ptr[T any](v T) *T {
	return &v
}
