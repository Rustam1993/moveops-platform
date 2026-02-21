package handlers

import (
	"context"
	"encoding/json"
	"errors"
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
	workflowStatusDraft    = "draft"
	workflowStatusOpen     = "open"
	workflowStatusFollowUp = "follow_up"
	workflowStatusQuoted   = "quoted"
	workflowStatusBooked   = "booked"
	workflowStatusOnHold   = "on_hold"
	workflowStatusCanceled = "canceled"
)

type workflowState struct {
	Status       string
	Priority     int32
	FollowUpAt   *time.Time
	FollowUpNote *string
	Vip          bool
	BookedAt     *time.Time
	HoldReason   *string
	UpdatedAt    time.Time
}

func (s *Server) GetEstimatesEstimateIdWorkflow(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	estimate, workflow, err := s.loadEstimateWorkflow(r.Context(), tenantID, uuid.UUID(estimateID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate workflow", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateWorkflowResponse{
		Workflow:  mapEstimateWorkflow(estimate.ID, workflow),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PatchEstimatesEstimateIdWorkflow(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.UpdateEstimateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	estimate, workflow, err := s.loadEstimateWorkflow(r.Context(), tenantID, uuid.UUID(estimateID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate workflow", nil)
		return
	}

	previous := workflow
	changed := false

	if req.Status != nil {
		nextStatus := string(*req.Status)
		if !isValidWorkflowStatus(nextStatus) {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "status is invalid", nil)
			return
		}
		workflow.Status = nextStatus
		changed = true
	}
	if req.PriorityLevel != nil {
		if *req.PriorityLevel < 0 || *req.PriorityLevel > 8 {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "priorityLevel must be between 0 and 8", nil)
			return
		}
		workflow.Priority = int32(*req.PriorityLevel)
		changed = true
	}
	if req.ClearFollowUpAt != nil && *req.ClearFollowUpAt {
		workflow.FollowUpAt = nil
		changed = true
	}
	if req.FollowUpAt != nil {
		t := req.FollowUpAt.UTC()
		workflow.FollowUpAt = &t
		changed = true
	}
	if req.ClearFollowUpNote != nil && *req.ClearFollowUpNote {
		workflow.FollowUpNote = nil
		changed = true
	}
	if req.FollowUpNote != nil {
		note := strings.TrimSpace(*req.FollowUpNote)
		if note == "" {
			workflow.FollowUpNote = nil
		} else {
			workflow.FollowUpNote = &note
		}
		changed = true
	}
	if req.Vip != nil {
		workflow.Vip = *req.Vip
		changed = true
	}

	if !changed {
		httpx.WriteJSON(w, http.StatusOK, oapi.EstimateWorkflowResponse{
			Workflow:  mapEstimateWorkflow(estimate.ID, workflow),
			RequestId: middleware.RequestIDFromContext(r.Context()),
		})
		return
	}

	now := time.Now().UTC()
	if workflow.Status == workflowStatusBooked {
		if workflow.BookedAt == nil {
			workflow.BookedAt = &now
		}
	} else {
		workflow.BookedAt = nil
	}
	if workflow.Status != workflowStatusOnHold {
		workflow.HoldReason = nil
	}

	persisted, err := s.persistEstimateWorkflow(r.Context(), tenantID, userID, estimate.ID, workflow)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save workflow", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "workflow.updated",
		EntityType: "estimate",
		EntityID:   &estimate.ID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"fromStatus": previous.Status,
			"toStatus":   persisted.Status,
			"priority":   persisted.Priority,
		},
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateWorkflowResponse{
		Workflow:  mapEstimateWorkflow(estimate.ID, persisted),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostEstimatesEstimateIdBook(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	estimate, workflow, err := s.loadEstimateWorkflow(r.Context(), tenantID, uuid.UUID(estimateID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate workflow", nil)
		return
	}
	if workflow.Status == workflowStatusCanceled {
		httpx.WriteError(w, r, http.StatusConflict, "booking_blocked", "Canceled estimates cannot be booked", nil)
		return
	}

	previousStatus := workflow.Status
	now := time.Now().UTC()
	workflow.Status = workflowStatusBooked
	workflow.BookedAt = &now

	persisted, err := s.persistEstimateWorkflow(r.Context(), tenantID, userID, estimate.ID, workflow)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to book estimate", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "booking.booked",
		EntityType: "estimate",
		EntityID:   &estimate.ID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"fromStatus": previousStatus,
			"toStatus":   persisted.Status,
			"bookedAt":   persisted.BookedAt.UTC().Format(time.RFC3339),
		},
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateWorkflowResponse{
		Workflow:  mapEstimateWorkflow(estimate.ID, persisted),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostEstimatesEstimateIdReleaseBook(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	estimate, workflow, err := s.loadEstimateWorkflow(r.Context(), tenantID, uuid.UUID(estimateID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate workflow", nil)
		return
	}
	if workflow.Status != workflowStatusBooked {
		httpx.WriteError(w, r, http.StatusConflict, "booking_not_active", "Estimate is not currently booked", nil)
		return
	}

	previousBookedAt := workflow.BookedAt
	workflow.Status = workflowStatusOpen
	workflow.BookedAt = nil

	persisted, err := s.persistEstimateWorkflow(r.Context(), tenantID, userID, estimate.ID, workflow)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to release booking", nil)
		return
	}

	metadata := map[string]any{
		"fromStatus": workflowStatusBooked,
		"toStatus":   persisted.Status,
	}
	if previousBookedAt != nil {
		metadata["previousBookedAt"] = previousBookedAt.UTC().Format(time.RFC3339)
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "booking.released",
		EntityType: "estimate",
		EntityID:   &estimate.ID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata:   metadata,
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateWorkflowResponse{
		Workflow:  mapEstimateWorkflow(estimate.ID, persisted),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostEstimatesEstimateIdHold(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	req := oapi.HoldEstimateRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
			return
		}
	}

	estimate, workflow, err := s.loadEstimateWorkflow(r.Context(), tenantID, uuid.UUID(estimateID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate workflow", nil)
		return
	}
	if workflow.Status == workflowStatusCanceled {
		httpx.WriteError(w, r, http.StatusConflict, "hold_blocked", "Canceled estimates cannot be moved on hold", nil)
		return
	}

	workflow.Status = workflowStatusOnHold
	holdReason := normalizeOptionalText(req.HoldReason)
	workflow.HoldReason = holdReason

	persisted, err := s.persistEstimateWorkflow(r.Context(), tenantID, userID, estimate.ID, workflow)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to move estimate on hold", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "booking.held",
		EntityType: "estimate",
		EntityID:   &estimate.ID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"toStatus":   persisted.Status,
			"holdReason": safeString(persisted.HoldReason),
		},
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateWorkflowResponse{
		Workflow:  mapEstimateWorkflow(estimate.ID, persisted),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetEstimatesEstimateIdTasks(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	if _, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: uuid.UUID(estimateID), TenantID: tenantID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	rows, err := s.Q.ListEstimateTasks(r.Context(), gen.ListEstimateTasksParams{
		TenantID:   tenantID,
		EstimateID: uuid.UUID(estimateID),
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate tasks", nil)
		return
	}

	tasks := make([]oapi.EstimateTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, mapEstimateTask(row))
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateTaskListResponse{
		Tasks:     tasks,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostEstimatesEstimateIdTasks(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.CreateEstimateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "title is required", nil)
		return
	}

	targetEstimateID := uuid.UUID(estimateID)
	if _, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: targetEstimateID, TenantID: tenantID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	var dueAt *time.Time
	if req.DueAt != nil {
		t := req.DueAt.UTC()
		dueAt = &t
	}

	row, err := s.Q.CreateEstimateTask(r.Context(), gen.CreateEstimateTaskParams{
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
		Title:      title,
		IsDone:     nil,
		DueAt:      dueAt,
		CreatedBy:  &userID,
		UpdatedBy:  &userID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create task", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "task.created",
		EntityType: "estimate",
		EntityID:   &targetEstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"taskId": row.ID,
			"title":  row.Title,
		},
	})

	httpx.WriteJSON(w, http.StatusCreated, oapi.EstimateTaskResponse{
		Task:      mapEstimateTask(row),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PatchEstimatesEstimateIdTasksTaskId(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID, taskID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.UpdateEstimateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	if req.Title == nil && req.IsDone == nil && req.DueAt == nil && (req.ClearDueAt == nil || !*req.ClearDueAt) {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "No task updates were provided", nil)
		return
	}

	targetEstimateID := uuid.UUID(estimateID)
	targetTaskID := uuid.UUID(taskID)

	before, err := s.Q.GetEstimateTaskByID(r.Context(), gen.GetEstimateTaskByIDParams{
		ID:         targetTaskID,
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "task_not_found", "Task was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load task", nil)
		return
	}

	var title *string
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "title cannot be empty", nil)
			return
		}
		title = &trimmed
	}

	clearDueAt := req.ClearDueAt != nil && *req.ClearDueAt
	var dueAt *time.Time
	if req.DueAt != nil {
		t := req.DueAt.UTC()
		dueAt = &t
	}

	row, err := s.Q.UpdateEstimateTask(r.Context(), gen.UpdateEstimateTaskParams{
		Title:      title,
		IsDone:     req.IsDone,
		DueAt:      dueAt,
		ClearDueAt: clearDueAt,
		UpdatedBy:  &userID,
		ID:         targetTaskID,
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "task_not_found", "Task was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update task", nil)
		return
	}

	action := "task.updated"
	if !before.IsDone && row.IsDone {
		action = "task.completed"
	}
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     action,
		EntityType: "estimate",
		EntityID:   &targetEstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"taskId": row.ID,
			"title":  row.Title,
		},
	})

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateTaskResponse{
		Task:      mapEstimateTask(row),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) DeleteEstimatesEstimateIdTasksTaskId(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID, taskID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	targetEstimateID := uuid.UUID(estimateID)
	targetTaskID := uuid.UUID(taskID)

	affected, err := s.Q.SoftDeleteEstimateTask(r.Context(), gen.SoftDeleteEstimateTaskParams{
		ID:         targetTaskID,
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete task", nil)
		return
	}
	if affected == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "task_not_found", "Task was not found", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "task.deleted",
		EntityType: "estimate",
		EntityID:   &targetEstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"taskId": targetTaskID,
		},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetEstimatesEstimateIdPayments(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{
		ID:       uuid.UUID(estimateID),
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

	payments, err := s.Q.ListEstimatePayments(r.Context(), gen.ListEstimatePaymentsParams{
		TenantID:   tenantID,
		EstimateID: estimate.ID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load payments", nil)
		return
	}
	paid, err := s.Q.SumEstimatePayments(r.Context(), gen.SumEstimatePaymentsParams{
		TenantID:   tenantID,
		EstimateID: estimate.ID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to summarize payments", nil)
		return
	}

	out := make([]oapi.EstimatePayment, 0, len(payments))
	for _, row := range payments {
		out = append(out, mapEstimatePayment(row))
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimatePaymentListResponse{
		Summary:   mapEstimatePaymentSummary(estimate, paid),
		Payments:  out,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostEstimatesEstimateIdPayments(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.CreateEstimatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}
	if req.AmountCents < 1 {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "amountCents must be greater than 0", nil)
		return
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "method is required", nil)
		return
	}

	targetEstimateID := uuid.UUID(estimateID)
	if _, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: targetEstimateID, TenantID: tenantID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	paidAt := time.Now().UTC()
	if req.PaidAt != nil {
		paidAt = req.PaidAt.UTC()
	}
	notes := normalizeOptionalText(req.Notes)

	row, err := s.Q.CreateEstimatePayment(r.Context(), gen.CreateEstimatePaymentParams{
		TenantID:    tenantID,
		EstimateID:  targetEstimateID,
		AmountCents: req.AmountCents,
		Method:      method,
		PaidAt:      paidAt,
		Notes:       notes,
		CreatedBy:   &userID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to add payment", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "payment.added",
		EntityType: "estimate",
		EntityID:   &targetEstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"paymentId":     row.ID,
			"amountCents":   row.AmountCents,
			"paymentMethod": row.Method,
		},
	})

	httpx.WriteJSON(w, http.StatusCreated, oapi.EstimatePaymentResponse{
		Payment:   mapEstimatePayment(row),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) DeleteEstimatesEstimateIdPaymentsPaymentId(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID, paymentID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	targetEstimateID := uuid.UUID(estimateID)
	targetPaymentID := uuid.UUID(paymentID)

	payment, err := s.Q.GetEstimatePaymentByID(r.Context(), gen.GetEstimatePaymentByIDParams{
		ID:         targetPaymentID,
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "payment_not_found", "Payment was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load payment", nil)
		return
	}

	affected, err := s.Q.SoftDeleteEstimatePayment(r.Context(), gen.SoftDeleteEstimatePaymentParams{
		ID:         targetPaymentID,
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete payment", nil)
		return
	}
	if affected == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "payment_not_found", "Payment was not found", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "payment.deleted",
		EntityType: "estimate",
		EntityID:   &targetEstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"paymentId":     payment.ID,
			"amountCents":   payment.AmountCents,
			"paymentMethod": payment.Method,
		},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadEstimateWorkflow(ctx context.Context, tenantID, estimateID uuid.UUID) (gen.Estimate, workflowState, error) {
	estimate, err := s.Q.GetEstimateByID(ctx, gen.GetEstimateByIDParams{
		ID:       estimateID,
		TenantID: tenantID,
	})
	if err != nil {
		return gen.Estimate{}, workflowState{}, err
	}

	row, err := s.Q.GetEstimateWorkflowByEstimateID(ctx, gen.GetEstimateWorkflowByEstimateIDParams{
		TenantID:   tenantID,
		EstimateID: estimateID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return estimate, defaultWorkflowState(estimate.UpdatedAt), nil
		}
		return gen.Estimate{}, workflowState{}, err
	}

	return estimate, workflowState{
		Status:       row.Status,
		Priority:     row.PriorityLevel,
		FollowUpAt:   row.FollowUpAt,
		FollowUpNote: row.FollowUpNote,
		Vip:          row.Vip,
		BookedAt:     row.BookedAt,
		HoldReason:   row.HoldReason,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func (s *Server) persistEstimateWorkflow(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	estimateID uuid.UUID,
	state workflowState,
) (workflowState, error) {
	row, err := s.Q.UpsertEstimateWorkflow(ctx, gen.UpsertEstimateWorkflowParams{
		TenantID:      tenantID,
		EstimateID:    estimateID,
		Status:        state.Status,
		PriorityLevel: state.Priority,
		FollowUpAt:    state.FollowUpAt,
		FollowUpNote:  state.FollowUpNote,
		Vip:           state.Vip,
		BookedAt:      state.BookedAt,
		HoldReason:    state.HoldReason,
		CreatedBy:     &userID,
		UpdatedBy:     &userID,
	})
	if err != nil {
		return workflowState{}, err
	}
	return workflowState{
		Status:       row.Status,
		Priority:     row.PriorityLevel,
		FollowUpAt:   row.FollowUpAt,
		FollowUpNote: row.FollowUpNote,
		Vip:          row.Vip,
		BookedAt:     row.BookedAt,
		HoldReason:   row.HoldReason,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func defaultWorkflowState(updatedAt time.Time) workflowState {
	return workflowState{
		Status:    workflowStatusDraft,
		Priority:  0,
		Vip:       false,
		UpdatedAt: updatedAt.UTC(),
	}
}

func isValidWorkflowStatus(v string) bool {
	switch v {
	case workflowStatusDraft, workflowStatusOpen, workflowStatusFollowUp, workflowStatusQuoted, workflowStatusBooked, workflowStatusOnHold, workflowStatusCanceled:
		return true
	default:
		return false
	}
}

func mapEstimateWorkflow(estimateID uuid.UUID, state workflowState) oapi.EstimateWorkflow {
	priority := int(state.Priority)
	out := oapi.EstimateWorkflow{
		EstimateId:    estimateID,
		Status:        oapi.EstimateWorkflowStatus(state.Status),
		PriorityLevel: priority,
		Vip:           state.Vip,
		UpdatedAt:     state.UpdatedAt.UTC(),
	}
	if state.FollowUpAt != nil {
		t := state.FollowUpAt.UTC()
		out.FollowUpAt = &t
	}
	if state.FollowUpNote != nil {
		out.FollowUpNote = state.FollowUpNote
	}
	if state.BookedAt != nil {
		t := state.BookedAt.UTC()
		out.BookedAt = &t
	}
	if state.HoldReason != nil {
		out.HoldReason = state.HoldReason
	}
	return out
}

func mapEstimateTask(row gen.EstimateTask) oapi.EstimateTask {
	out := oapi.EstimateTask{
		Id:         row.ID,
		EstimateId: row.EstimateID,
		Title:      row.Title,
		IsDone:     row.IsDone,
		CreatedAt:  row.CreatedAt.UTC(),
		UpdatedAt:  row.UpdatedAt.UTC(),
	}
	if row.DueAt != nil {
		t := row.DueAt.UTC()
		out.DueAt = &t
	}
	return out
}

func mapEstimatePayment(row gen.EstimatePayment) oapi.EstimatePayment {
	out := oapi.EstimatePayment{
		Id:          row.ID,
		EstimateId:  row.EstimateID,
		AmountCents: row.AmountCents,
		Method:      row.Method,
		PaidAt:      row.PaidAt.UTC(),
		CreatedAt:   row.CreatedAt.UTC(),
	}
	if row.Notes != nil {
		out.Notes = row.Notes
	}
	return out
}

func mapEstimatePaymentSummary(estimate gen.Estimate, amountPaidCents int64) oapi.EstimatePaymentSummary {
	out := oapi.EstimatePaymentSummary{
		AmountPaidCents: amountPaidCents,
	}
	if estimate.DepositCents != nil {
		out.DepositRequiredCents = estimate.DepositCents
	}
	if estimate.EstimatedTotalCents != nil {
		out.TotalEstimateCents = estimate.EstimatedTotalCents
		remaining := *estimate.EstimatedTotalCents - amountPaidCents
		out.RemainingBalanceCents = &remaining
	}
	return out
}

func normalizeOptionalText(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
