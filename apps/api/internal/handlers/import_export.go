package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
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
	importSeverityError = "error"
	importSeverityWarn  = "warn"
	importSeverityInfo  = "info"
)

var supportedCSVContentTypes = map[string]struct{}{
	"text/csv":                 {},
	"application/csv":          {},
	"application/vnd.ms-excel": {},
}

type importMode string

const (
	importModeDryRun importMode = "dry_run"
	importModeApply  importMode = "apply"
)

type importOptionsPayload struct {
	Source    string         `json:"source"`
	HasHeader *bool          `json:"hasHeader,omitempty"`
	Mapping   map[string]any `json:"mapping"`
	Mode      string         `json:"mode,omitempty"`
}

type importRunSummary struct {
	RowsTotal     int64              `json:"rowsTotal"`
	RowsValid     int64              `json:"rowsValid"`
	RowsError     int64              `json:"rowsError"`
	Customer      importResultCounts `json:"customer"`
	Estimate      importResultCounts `json:"estimate"`
	Job           importResultCounts `json:"job"`
	StorageRecord importResultCounts `json:"storageRecord"`
}

type importResultCounts struct {
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
	Skipped int64 `json:"skipped"`
	Error   int64 `json:"error"`
}

type importRowMessage struct {
	RowNumber      int                 `json:"rowNumber"`
	Severity       string              `json:"severity"`
	EntityType     string              `json:"entityType"`
	Result         string              `json:"result"`
	IdempotencyKey string              `json:"idempotencyKey"`
	Field          *string             `json:"field,omitempty"`
	Message        string              `json:"message"`
	RawValue       *string             `json:"rawValue,omitempty"`
	TargetEntityID *openapi_types.UUID `json:"targetEntityId,omitempty"`
}

type importRunReport struct {
	Run       oapi.ImportRunResponse `json:"run"`
	Rows      []importRowMessage     `json:"rows"`
	RequestID string                 `json:"requestId"`
}

type canonicalImportRow struct {
	CustomerName        string
	Email               string
	PhonePrimary        string
	PhoneSecondary      string
	EstimateNumber      string
	OriginZIP           string
	DestinationZIP      string
	OriginCity          string
	DestinationCity     string
	OriginState         string
	DestinationState    string
	RequestedPickupDate string
	RequestedPickupTime string
	LeadSource          string
	EstimatedTotal      string
	Deposit             string
	PricingNotes        string
	JobNumber           string
	ScheduledDate       string
	PickupTime          string
	Phase               string
	Status              string
	JobType             string
	Facility            string
	StorageStatus       string
	DateIn              string
	DateOut             string
	NextBillDate        string
	LotNumber           string
	LocationLabel       string
	Vaults              string
	Pads                string
	Items               string
	OversizeItems       string
	Volume              string
	MonthlyRate         string
	StorageBalance      string
	MoveBalance         string
}

type rowOutcome struct {
	entityType     string
	severity       string
	result         string
	idempotencyKey string
	field          *string
	message        string
	rawValue       *string
	targetEntityID *uuid.UUID
}

type parsedImportFile struct {
	filename   string
	fileSHA256 string
	options    importOptionsPayload
	headers    []string
	rows       [][]string
	mapping    map[string]int
	hasHeader  bool
}

func (s *Server) PostImportsDryRun(w http.ResponseWriter, r *http.Request) {
	s.handleImport(w, r, importModeDryRun)
}

func (s *Server) PostImportsApply(w http.ResponseWriter, r *http.Request) {
	s.handleImport(w, r, importModeApply)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request, mode importMode) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	parsed, appErr := parseImportUpload(r, s.Config.ImportMaxRows)
	if appErr != nil {
		httpx.WriteError(w, r, appErr.Status, appErr.Code, appErr.Message, appErr.Details)
		return
	}

	mappingJSON, _ := json.Marshal(map[string]any{
		"source":    parsed.options.Source,
		"hasHeader": parsed.hasHeader,
		"mapping":   parsed.options.Mapping,
	})
	initialSummary := []byte(`{}`)
	run, err := s.Q.CreateImportRun(r.Context(), gen.CreateImportRunParams{
		TenantID:        tenantID,
		CreatedByUserID: &userID,
		Source:          parsed.options.Source,
		Filename:        parsed.filename,
		FileSha256:      parsed.fileSHA256,
		Mode:            string(mode),
		Status:          "failed",
		MappingJson:     mappingJSON,
		SummaryJson:     initialSummary,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create import run", nil)
		return
	}

	runID := run.ID
	requestID := middleware.RequestIDFromContext(r.Context())
	startAction := "import.dry_run_started"
	completeAction := "import.dry_run_completed"
	if mode == importModeApply {
		startAction = "import.apply_started"
		completeAction = "import.apply_completed"
	}
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     startAction,
		EntityType: "import_run",
		EntityID:   &runID,
		RequestID:  requestID,
		Metadata: map[string]any{
			"mode":       mode,
			"source":     parsed.options.Source,
			"filename":   parsed.filename,
			"fileSha256": parsed.fileSHA256,
			"rowsTotal":  len(parsed.rows),
		},
	})

	summary, outcomes, processErr := s.processImportRows(r, tenantID, userID, mode, run.ID, parsed)
	summaryJSON, _ := json.Marshal(summary)
	finalStatus := "completed"
	if processErr != nil {
		finalStatus = "failed"
	}

	updatedRun, updateErr := s.Q.CompleteImportRun(r.Context(), gen.CompleteImportRunParams{
		Status:      finalStatus,
		SummaryJson: summaryJSON,
		ID:          run.ID,
		TenantID:    tenantID,
	})
	if updateErr != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to complete import run", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     completeAction,
		EntityType: "import_run",
		EntityID:   &runID,
		RequestID:  requestID,
		Metadata: map[string]any{
			"mode":       mode,
			"source":     parsed.options.Source,
			"filename":   parsed.filename,
			"fileSha256": parsed.fileSHA256,
			"status":     finalStatus,
			"summary":    summary,
		},
	})

	if processErr != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "import_failed", processErr.Error(), map[string]any{"importRunId": run.ID})
		return
	}

	topWarnings := topOutcomesBySeverity(outcomes, importSeverityWarn, 100)
	topErrors := topOutcomesBySeverity(outcomes, importSeverityError, 100)
	httpx.WriteJSON(w, http.StatusOK, mapImportRunResponse(updatedRun, summary, topWarnings, topErrors, requestID))
}

func (s *Server) processImportRows(
	r *http.Request,
	tenantID uuid.UUID,
	userID uuid.UUID,
	mode importMode,
	importRunID uuid.UUID,
	parsed parsedImportFile,
) (importRunSummary, []importRowMessage, error) {
	summary := importRunSummary{}
	outcomes := make([]importRowMessage, 0, len(parsed.rows)*3)

	for idx, row := range parsed.rows {
		summary.RowsTotal++
		rowNumber := idx + 1
		if parsed.hasHeader {
			rowNumber = idx + 2
		}

		canonical := buildCanonicalImportRow(row, parsed.mapping)
		rowOutcomes, rowErr := s.processImportRow(r, tenantID, userID, mode, canonical)
		rowHasError := rowErr != nil
		if rowErr != nil && len(rowOutcomes) == 0 {
			rowOutcomes = append(rowOutcomes, rowOutcome{
				entityType:     "customer",
				severity:       importSeverityError,
				result:         "error",
				idempotencyKey: fmt.Sprintf("row:%d", rowNumber),
				message:        rowErr.Error(),
			})
		}

		for _, outcome := range rowOutcomes {
			targetEntityID := (*uuid.UUID)(nil)
			if outcome.targetEntityID != nil {
				targetEntityID = outcome.targetEntityID
			}

			_, err := s.Q.UpsertImportRowResult(r.Context(), gen.UpsertImportRowResultParams{
				TenantID:       tenantID,
				ImportRunID:    importRunID,
				RowNumber:      int32(rowNumber),
				Severity:       outcome.severity,
				EntityType:     outcome.entityType,
				IdempotencyKey: outcome.idempotencyKey,
				Result:         outcome.result,
				Field:          outcome.field,
				Message:        truncateText(outcome.message, 500),
				RawValue:       outcome.rawValue,
				TargetEntityID: targetEntityID,
			})
			if err != nil {
				return summary, outcomes, fmt.Errorf("persist import row result: %w", err)
			}

			mappedOutcome := mapOutcome(rowNumber, outcome)
			outcomes = append(outcomes, mappedOutcome)
			incrementSummary(&summary, outcome.entityType, outcome.result)
			if outcome.severity == importSeverityError {
				rowHasError = true
			}
		}

		if rowHasError {
			summary.RowsError++
		} else {
			summary.RowsValid++
		}
	}

	return summary, outcomes, nil
}

func (s *Server) processImportRow(
	r *http.Request,
	tenantID uuid.UUID,
	userID uuid.UUID,
	mode importMode,
	row canonicalImportRow,
) ([]rowOutcome, error) {
	outcomes := make([]rowOutcome, 0, 4)

	customerName := strings.TrimSpace(row.CustomerName)
	email := normalizeEmail(row.Email)
	phonePrimary := normalizePhone(row.PhonePrimary)
	phoneSecondary := normalizePhone(row.PhoneSecondary)

	if customerName == "" && email == "" && phonePrimary == "" {
		return outcomes, errors.New("customer_name or email/phone_primary is required")
	}

	customerKey := buildCustomerKey(customerName, email, phonePrimary)
	customerOutcome, customerID, customerErr := s.upsertOrSimulateCustomer(r, tenantID, userID, mode, customerName, email, phonePrimary, phoneSecondary, customerKey)
	outcomes = append(outcomes, customerOutcome)
	if customerErr != nil {
		return outcomes, customerErr
	}

	estimateOutcome, estimateID, estimateErr := s.upsertOrSimulateEstimate(r, tenantID, userID, mode, row, customerID, customerName, email, phonePrimary)
	if estimateOutcome.idempotencyKey != "" {
		outcomes = append(outcomes, estimateOutcome)
	}
	if estimateErr != nil {
		return outcomes, estimateErr
	}

	jobOutcome, jobID, jobErr := s.upsertOrSimulateJob(r, tenantID, userID, mode, row, customerID, estimateID)
	if jobOutcome.idempotencyKey != "" {
		outcomes = append(outcomes, jobOutcome)
	}
	if jobErr != nil {
		return outcomes, jobErr
	}

	if hasStorageFields(row) {
		storageOutcome, storageErr := s.upsertOrSimulateStorage(r, tenantID, userID, mode, row, jobID, jobOutcome.idempotencyKey)
		if storageOutcome.idempotencyKey != "" {
			outcomes = append(outcomes, storageOutcome)
		}
		if storageErr != nil {
			return outcomes, storageErr
		}
	}

	return outcomes, nil
}

func (s *Server) upsertOrSimulateCustomer(
	r *http.Request,
	tenantID, userID uuid.UUID,
	mode importMode,
	customerName, email, phonePrimary, phoneSecondary, customerKey string,
) (rowOutcome, uuid.UUID, error) {
	outcome := rowOutcome{
		entityType:     "customer",
		severity:       importSeverityInfo,
		result:         "skipped",
		idempotencyKey: customerKey,
		message:        "Customer unchanged",
	}

	var existing *gen.Customer
	if mapped, err := s.Q.GetImportIdempotency(r.Context(), gen.GetImportIdempotencyParams{
		TenantID:       tenantID,
		EntityType:     "customer",
		IdempotencyKey: customerKey,
	}); err == nil {
		customer, err := s.Q.GetCustomerByID(r.Context(), gen.GetCustomerByIDParams{ID: mapped.TargetEntityID, TenantID: tenantID})
		if err == nil {
			existing = &customer
		}
	}

	if existing == nil && email != "" {
		if customer, err := s.Q.FindCustomerByEmail(r.Context(), gen.FindCustomerByEmailParams{TenantID: tenantID, Email: email}); err == nil {
			existing = &customer
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return outcome, uuid.Nil, err
		}
	}
	if existing == nil && phonePrimary != "" {
		if customer, err := s.Q.FindCustomerByPhone(r.Context(), gen.FindCustomerByPhoneParams{TenantID: tenantID, Phone: &phonePrimary}); err == nil {
			existing = &customer
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return outcome, uuid.Nil, err
		}
	}

	firstName, lastName := splitName(nonEmpty(customerName, "Imported Customer"))
	emailPtr := stringPtrOrNil(email)
	phonePtr := stringPtrOrNil(phonePrimary)
	secondaryPtr := stringPtrOrNil(phoneSecondary)
	if secondaryPtr != nil && phonePtr == nil {
		phonePtr = secondaryPtr
	}

	if existing == nil {
		outcome.result = "created"
		outcome.message = "Customer will be created"
		outcome.severity = importSeverityWarn
		if email == "" && phonePrimary == "" {
			outcome.message = "Customer created without email/phone using name fallback"
		} else {
			outcome.severity = importSeverityInfo
		}

		if mode == importModeDryRun {
			return outcome, uuid.Nil, nil
		}

		created, err := s.Q.CreateCustomer(r.Context(), gen.CreateCustomerParams{
			TenantID:  tenantID,
			FirstName: firstName,
			LastName:  lastName,
			Email:     emailPtr,
			Phone:     phonePtr,
			CreatedBy: &userID,
			UpdatedBy: &userID,
		})
		if err != nil {
			if isUniqueConstraint(err, "customers_tenant_email_uidx") && email != "" {
				existingCustomer, lookupErr := s.Q.FindCustomerByEmail(r.Context(), gen.FindCustomerByEmailParams{TenantID: tenantID, Email: email})
				if lookupErr != nil {
					return outcome, uuid.Nil, err
				}
				existing = &existingCustomer
			} else {
				return outcome, uuid.Nil, err
			}
		} else {
			id := created.ID
			outcome.targetEntityID = &id
			_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
				TenantID:       tenantID,
				EntityType:     "customer",
				IdempotencyKey: customerKey,
				TargetEntityID: created.ID,
			})
			return outcome, created.ID, nil
		}
	}

	outcome.result = "updated"
	outcome.message = "Customer updated"
	if mode == importModeDryRun {
		if existing != nil {
			id := existing.ID
			outcome.targetEntityID = &id
			return outcome, existing.ID, nil
		}
		return outcome, uuid.Nil, nil
	}

	updated, err := s.Q.UpdateCustomerForEstimate(r.Context(), gen.UpdateCustomerForEstimateParams{
		FirstName: &firstName,
		LastName:  &lastName,
		Email:     emailPtr,
		Phone:     phonePtr,
		UpdatedBy: &userID,
		ID:        existing.ID,
		TenantID:  tenantID,
	})
	if err != nil {
		return outcome, uuid.Nil, err
	}
	id := updated.ID
	outcome.targetEntityID = &id
	_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
		TenantID:       tenantID,
		EntityType:     "customer",
		IdempotencyKey: customerKey,
		TargetEntityID: updated.ID,
	})
	return outcome, updated.ID, nil
}

func (s *Server) upsertOrSimulateEstimate(
	r *http.Request,
	tenantID, userID uuid.UUID,
	mode importMode,
	row canonicalImportRow,
	customerID uuid.UUID,
	customerName, email, phonePrimary string,
) (rowOutcome, *uuid.UUID, error) {
	if !hasEstimateFields(row) {
		return rowOutcome{}, nil, nil
	}

	estimateKey := buildEstimateKey(row, customerName, email, phonePrimary)
	outcome := rowOutcome{
		entityType:     "estimate",
		severity:       importSeverityInfo,
		result:         "skipped",
		idempotencyKey: estimateKey,
		message:        "Estimate unchanged",
	}

	moveDate, dateWarn, err := parseFlexibleDate(row.RequestedPickupDate)
	if err != nil {
		outcome.severity = importSeverityError
		outcome.result = "error"
		outcome.field = stringPtr("requested_pickup_date")
		outcome.message = "Invalid requested_pickup_date"
		return outcome, nil, nil
	}
	if row.OriginZIP == "" || row.DestinationZIP == "" || moveDate == nil {
		outcome.severity = importSeverityError
		outcome.result = "error"
		outcome.message = "origin_zip, destination_zip, and requested_pickup_date are required to create/update estimates"
		return outcome, nil, nil
	}
	if dateWarn != "" {
		outcome.severity = importSeverityWarn
		outcome.message = dateWarn
	}

	estimateNumber := strings.TrimSpace(row.EstimateNumber)
	if estimateNumber == "" {
		estimateNumber = "IMP-E-" + shortHash(estimateKey, 10)
	}

	var existing *gen.Estimate
	if mapped, err := s.Q.GetImportIdempotency(r.Context(), gen.GetImportIdempotencyParams{
		TenantID:       tenantID,
		EntityType:     "estimate",
		IdempotencyKey: estimateKey,
	}); err == nil {
		estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: mapped.TargetEntityID, TenantID: tenantID})
		if err == nil {
			existing = &estimate
		}
	}
	if existing == nil {
		if estimate, err := s.Q.GetEstimateByNumber(r.Context(), gen.GetEstimateByNumberParams{
			TenantID:       tenantID,
			EstimateNumber: estimateNumber,
		}); err == nil {
			existing = &estimate
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return outcome, nil, err
		}
	}

	originState := nonEmpty(row.OriginState, "NA")
	destinationState := nonEmpty(row.DestinationState, "NA")
	pickupTime := stringPtrOrNil(nonEmpty(row.PickupTime, row.RequestedPickupTime))
	leadSource := nonEmpty(row.LeadSource, "Import")
	notes := stringPtrOrNil(row.PricingNotes)
	estimatedTotalCents, _ := parseMoneyCents(row.EstimatedTotal)
	depositCents, _ := parseMoneyCents(row.Deposit)

	if existing == nil {
		outcome.result = "created"
		outcome.message = nonEmpty(outcome.message, "Estimate created")
		if mode == importModeDryRun {
			return outcome, nil, nil
		}

		created, err := s.Q.CreateEstimate(r.Context(), gen.CreateEstimateParams{
			TenantID:                tenantID,
			EstimateNumber:          estimateNumber,
			CustomerID:              customerID,
			Status:                  "draft",
			CustomerName:            nonEmpty(customerName, "Imported Customer"),
			PrimaryPhone:            nonEmpty(phonePrimary, "n/a"),
			SecondaryPhone:          nil,
			Email:                   nonEmpty(email, "import@moveops.local"),
			OriginAddressLine1:      "Imported origin",
			OriginCity:              nonEmpty(row.OriginCity, "Unknown"),
			OriginState:             originState,
			OriginPostalCode:        row.OriginZIP,
			DestinationAddressLine1: "Imported destination",
			DestinationCity:         nonEmpty(row.DestinationCity, "Unknown"),
			DestinationState:        destinationState,
			DestinationPostalCode:   row.DestinationZIP,
			MoveDate:                moveDate.UTC(),
			PickupTime:              pickupTime,
			LeadSource:              leadSource,
			MoveSize:                stringPtrOrNil(row.JobType),
			LocationType:            nil,
			EstimatedTotalCents:     estimatedTotalCents,
			DepositCents:            depositCents,
			Notes:                   notes,
			IdempotencyKey:          nil,
			IdempotencyPayloadHash:  nil,
			CreatedBy:               &userID,
			UpdatedBy:               &userID,
		})
		if err != nil {
			return outcome, nil, err
		}
		id := created.ID
		outcome.targetEntityID = &id
		_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
			TenantID:       tenantID,
			EntityType:     "estimate",
			IdempotencyKey: estimateKey,
			TargetEntityID: created.ID,
		})
		return outcome, &created.ID, nil
	}

	outcome.result = "updated"
	outcome.message = nonEmpty(outcome.message, "Estimate updated")
	if mode == importModeDryRun {
		id := existing.ID
		outcome.targetEntityID = &id
		return outcome, &existing.ID, nil
	}

	updated, err := s.Q.UpdateEstimateByNumber(r.Context(), gen.UpdateEstimateByNumberParams{
		CustomerID:              customerID,
		Status:                  stringPtr("draft"),
		CustomerName:            nonEmpty(customerName, "Imported Customer"),
		PrimaryPhone:            nonEmpty(phonePrimary, "n/a"),
		SecondaryPhone:          nil,
		Email:                   nonEmpty(email, "import@moveops.local"),
		OriginAddressLine1:      "Imported origin",
		OriginCity:              nonEmpty(row.OriginCity, "Unknown"),
		OriginState:             originState,
		OriginPostalCode:        row.OriginZIP,
		DestinationAddressLine1: "Imported destination",
		DestinationCity:         nonEmpty(row.DestinationCity, "Unknown"),
		DestinationState:        destinationState,
		DestinationPostalCode:   row.DestinationZIP,
		MoveDate:                moveDate.UTC(),
		PickupTime:              pickupTime,
		LeadSource:              leadSource,
		MoveSize:                stringPtrOrNil(row.JobType),
		LocationType:            nil,
		EstimatedTotalCents:     estimatedTotalCents,
		DepositCents:            depositCents,
		Notes:                   notes,
		UpdatedBy:               &userID,
		TenantID:                tenantID,
		EstimateNumber:          estimateNumber,
	})
	if err != nil {
		return outcome, nil, err
	}

	id := updated.ID
	outcome.targetEntityID = &id
	_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
		TenantID:       tenantID,
		EntityType:     "estimate",
		IdempotencyKey: estimateKey,
		TargetEntityID: updated.ID,
	})
	return outcome, &updated.ID, nil
}

func (s *Server) upsertOrSimulateJob(
	r *http.Request,
	tenantID, userID uuid.UUID,
	mode importMode,
	row canonicalImportRow,
	customerID uuid.UUID,
	estimateID *uuid.UUID,
) (rowOutcome, uuid.UUID, error) {
	jobNumber := strings.TrimSpace(row.JobNumber)
	jobKey := buildJobKey(row, customerID)
	outcome := rowOutcome{
		entityType:     "job",
		severity:       importSeverityInfo,
		result:         "skipped",
		idempotencyKey: jobKey,
		message:        "Job unchanged",
	}

	if jobNumber == "" {
		jobNumber = "IMP-J-" + shortHash(jobKey, 10)
		outcome.severity = importSeverityWarn
		outcome.message = "job_number missing, generated deterministic import job number"
	}

	var existing *gen.Job
	if mapped, err := s.Q.GetImportIdempotency(r.Context(), gen.GetImportIdempotencyParams{
		TenantID:       tenantID,
		EntityType:     "job",
		IdempotencyKey: jobKey,
	}); err == nil {
		job, err := s.Q.GetJobByID(r.Context(), gen.GetJobByIDParams{ID: mapped.TargetEntityID, TenantID: tenantID})
		if err == nil {
			existing = &job
		}
	}
	if existing == nil {
		if job, err := s.Q.GetJobByJobNumber(r.Context(), gen.GetJobByJobNumberParams{
			TenantID:  tenantID,
			JobNumber: jobNumber,
		}); err == nil {
			existing = &job
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return outcome, uuid.Nil, err
		}
	}

	scheduledDate, _, _ := parseFlexibleDate(nonEmpty(row.ScheduledDate, row.RequestedPickupDate))
	pickupTime := stringPtrOrNil(nonEmpty(row.PickupTime, row.RequestedPickupTime))
	status := normalizeJobStatus(nonEmpty(row.Status, row.Phase))
	estimateRef := estimateID

	if existing == nil {
		outcome.result = "created"
		outcome.message = nonEmpty(outcome.message, "Job created")
		if mode == importModeDryRun {
			return outcome, uuid.Nil, nil
		}

		created, err := s.Q.CreateJob(r.Context(), gen.CreateJobParams{
			TenantID:              tenantID,
			JobNumber:             jobNumber,
			EstimateID:            estimateRef,
			CustomerID:            customerID,
			Status:                status,
			ScheduledDate:         scheduledDate,
			PickupTime:            pickupTime,
			ConvertIdempotencyKey: nil,
			CreatedBy:             &userID,
			UpdatedBy:             &userID,
		})
		if err != nil {
			return outcome, uuid.Nil, err
		}
		id := created.ID
		outcome.targetEntityID = &id
		_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
			TenantID:       tenantID,
			EntityType:     "job",
			IdempotencyKey: jobKey,
			TargetEntityID: created.ID,
		})
		return outcome, created.ID, nil
	}

	outcome.result = "updated"
	outcome.message = nonEmpty(outcome.message, "Job updated")
	if mode == importModeDryRun {
		id := existing.ID
		outcome.targetEntityID = &id
		return outcome, existing.ID, nil
	}

	updated, err := s.Q.UpdateJobByJobNumber(r.Context(), gen.UpdateJobByJobNumberParams{
		EstimateID:    estimateRef,
		CustomerID:    customerID,
		Status:        &status,
		ScheduledDate: scheduledDate,
		PickupTime:    pickupTime,
		UpdatedBy:     &userID,
		TenantID:      tenantID,
		JobNumber:     jobNumber,
	})
	if err != nil {
		return outcome, uuid.Nil, err
	}

	id := updated.ID
	outcome.targetEntityID = &id
	_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
		TenantID:       tenantID,
		EntityType:     "job",
		IdempotencyKey: jobKey,
		TargetEntityID: updated.ID,
	})
	return outcome, updated.ID, nil
}

func (s *Server) upsertOrSimulateStorage(
	r *http.Request,
	tenantID, userID uuid.UUID,
	mode importMode,
	row canonicalImportRow,
	jobID uuid.UUID,
	jobKey string,
) (rowOutcome, error) {
	storageKey := "storage:" + jobKey
	outcome := rowOutcome{
		entityType:     "storage_record",
		severity:       importSeverityInfo,
		result:         "skipped",
		idempotencyKey: storageKey,
		message:        "Storage record unchanged",
	}

	facility := strings.TrimSpace(row.Facility)
	if facility == "" {
		facility = "Unassigned"
		outcome.severity = importSeverityWarn
		outcome.message = "facility missing, defaulted to Unassigned"
	}

	status := normalizeStorageStatus(strings.TrimSpace(row.StorageStatus))
	dateIn, _, _ := parseFlexibleDate(row.DateIn)
	dateOut, _, _ := parseFlexibleDate(row.DateOut)
	nextBillDate, _, _ := parseFlexibleDate(row.NextBillDate)

	vaults, _ := parseIntNonNegative(row.Vaults)
	pads, _ := parseIntNonNegative(row.Pads)
	items, _ := parseIntNonNegative(row.Items)
	oversizeItems, _ := parseIntNonNegative(row.OversizeItems)
	volume, _ := parseIntNonNegative(row.Volume)
	monthlyRateCents, _ := parseMoneyCents(row.MonthlyRate)
	storageBalanceCents, _ := parseMoneyCents(row.StorageBalance)
	moveBalanceCents, _ := parseMoneyCents(row.MoveBalance)

	existing, err := s.Q.GetStorageRecordByJobID(r.Context(), gen.GetStorageRecordByJobIDParams{
		JobID:    jobID,
		TenantID: tenantID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return outcome, err
	}

	if errors.Is(err, pgx.ErrNoRows) {
		outcome.result = "created"
		outcome.message = nonEmpty(outcome.message, "Storage record created")
		if mode == importModeDryRun {
			return outcome, nil
		}
		created, err := s.Q.CreateStorageRecord(r.Context(), gen.CreateStorageRecordParams{
			TenantID:            tenantID,
			JobID:               jobID,
			Facility:            facility,
			Status:              &status,
			DateIn:              dateIn,
			DateOut:             dateOut,
			NextBillDate:        nextBillDate,
			LotNumber:           stringPtrOrNil(row.LotNumber),
			LocationLabel:       stringPtrOrNil(row.LocationLabel),
			Vaults:              int32Ptr(vaults),
			Pads:                int32Ptr(pads),
			Items:               int32Ptr(items),
			OversizeItems:       int32Ptr(oversizeItems),
			Volume:              int32Ptr(volume),
			MonthlyRateCents:    monthlyRateCents,
			StorageBalanceCents: int64Ptr(defaultInt64(storageBalanceCents, 0)),
			MoveBalanceCents:    int64Ptr(defaultInt64(moveBalanceCents, 0)),
			LastPaymentAt:       nil,
			Notes:               stringPtrOrNil(row.PricingNotes),
		})
		if err != nil {
			return outcome, err
		}
		id := created.ID
		outcome.targetEntityID = &id
		_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
			TenantID:       tenantID,
			EntityType:     "storage_record",
			IdempotencyKey: storageKey,
			TargetEntityID: created.ID,
		})
		return outcome, nil
	}

	outcome.result = "updated"
	outcome.message = nonEmpty(outcome.message, "Storage record updated")
	if mode == importModeDryRun {
		id := existing.ID
		outcome.targetEntityID = &id
		return outcome, nil
	}

	updated, err := s.Q.UpdateStorageRecordByID(r.Context(), gen.UpdateStorageRecordByIDParams{
		Facility:            facility,
		Status:              status,
		DateIn:              dateIn,
		DateOut:             dateOut,
		NextBillDate:        nextBillDate,
		LotNumber:           stringPtrOrNil(row.LotNumber),
		LocationLabel:       stringPtrOrNil(row.LocationLabel),
		Vaults:              int32(vaults),
		Pads:                int32(pads),
		Items:               int32(items),
		OversizeItems:       int32(oversizeItems),
		Volume:              int32(volume),
		MonthlyRateCents:    monthlyRateCents,
		StorageBalanceCents: defaultInt64(storageBalanceCents, 0),
		MoveBalanceCents:    defaultInt64(moveBalanceCents, 0),
		LastPaymentAt:       nil,
		Notes:               stringPtrOrNil(row.PricingNotes),
		ID:                  existing.ID,
		TenantID:            tenantID,
	})
	if err != nil {
		return outcome, err
	}
	id := updated.ID
	outcome.targetEntityID = &id
	_, _ = s.Q.UpsertImportIdempotency(r.Context(), gen.UpsertImportIdempotencyParams{
		TenantID:       tenantID,
		EntityType:     "storage_record",
		IdempotencyKey: storageKey,
		TargetEntityID: updated.ID,
	})
	return outcome, nil
}

func (s *Server) GetImportsImportRunId(w http.ResponseWriter, r *http.Request, importRunId openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	run, err := s.Q.GetImportRunByID(r.Context(), gen.GetImportRunByIDParams{
		ID:       uuid.UUID(importRunId),
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "import_run_not_found", "Import run not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load import run", nil)
		return
	}

	summary := parseImportSummary(run.SummaryJson)
	warnings, _ := s.Q.ListImportRowResultsByRunAndSeverity(r.Context(), gen.ListImportRowResultsByRunAndSeverityParams{
		TenantID:    tenantID,
		ImportRunID: run.ID,
		Severity:    importSeverityWarn,
		LimitRows:   100,
	})
	errorsRows, _ := s.Q.ListImportRowResultsByRunAndSeverity(r.Context(), gen.ListImportRowResultsByRunAndSeverityParams{
		TenantID:    tenantID,
		ImportRunID: run.ID,
		Severity:    importSeverityError,
		LimitRows:   100,
	})

	httpx.WriteJSON(w, http.StatusOK, mapImportRunResponse(
		run,
		summary,
		mapRowResults(warnings),
		mapRowResults(errorsRows),
		middleware.RequestIDFromContext(r.Context()),
	))
}

func (s *Server) GetImportsImportRunIdErrorsCsv(w http.ResponseWriter, r *http.Request, importRunId openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	if _, err := s.Q.GetImportRunByID(r.Context(), gen.GetImportRunByIDParams{
		ID:       uuid.UUID(importRunId),
		TenantID: tenantID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "import_run_not_found", "Import run not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load import run", nil)
		return
	}

	rows, err := s.Q.ListImportRowResultsByRun(r.Context(), gen.ListImportRowResultsByRunParams{
		TenantID:    tenantID,
		ImportRunID: uuid.UUID(importRunId),
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load import rows", nil)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"import-%s-errors.csv\"", importRunId.String()))
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"row_number", "severity", "entity_type", "result", "field", "message", "raw_value", "idempotency_key", "target_entity_id"})
	for _, row := range rows {
		if row.Severity != importSeverityError && row.Severity != importSeverityWarn {
			continue
		}
		target := ""
		if row.TargetEntityID != nil {
			target = row.TargetEntityID.String()
		}
		_ = writer.Write([]string{
			strconv.Itoa(int(row.RowNumber)),
			row.Severity,
			row.EntityType,
			row.Result,
			derefString(row.Field),
			row.Message,
			derefString(row.RawValue),
			row.IdempotencyKey,
			target,
		})
	}
	writer.Flush()
}

func (s *Server) GetImportsImportRunIdReportJson(w http.ResponseWriter, r *http.Request, importRunId openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	run, err := s.Q.GetImportRunByID(r.Context(), gen.GetImportRunByIDParams{
		ID:       uuid.UUID(importRunId),
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "import_run_not_found", "Import run not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load import run", nil)
		return
	}
	rowResults, err := s.Q.ListImportRowResultsByRun(r.Context(), gen.ListImportRowResultsByRunParams{
		TenantID:    tenantID,
		ImportRunID: run.ID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load import rows", nil)
		return
	}

	summary := parseImportSummary(run.SummaryJson)
	warnings, _ := s.Q.ListImportRowResultsByRunAndSeverity(r.Context(), gen.ListImportRowResultsByRunAndSeverityParams{
		TenantID:    tenantID,
		ImportRunID: run.ID,
		Severity:    importSeverityWarn,
		LimitRows:   100,
	})
	errorsRows, _ := s.Q.ListImportRowResultsByRunAndSeverity(r.Context(), gen.ListImportRowResultsByRunAndSeverityParams{
		TenantID:    tenantID,
		ImportRunID: run.ID,
		Severity:    importSeverityError,
		LimitRows:   100,
	})

	payload := oapi.ImportRunReportResponse{
		Run:       mapImportRunResponse(run, summary, mapRowResults(warnings), mapRowResults(errorsRows), middleware.RequestIDFromContext(r.Context())),
		Rows:      mapRowResults(rowResults),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	}
	httpx.WriteJSON(w, http.StatusOK, payload)
}

func (s *Server) GetImportsTemplatesTemplateCsv(w http.ResponseWriter, r *http.Request, template oapi.ImportTemplate) {
	normalized := strings.ToLower(strings.TrimSpace(string(template)))
	content, ok := importTemplates[normalized]
	if !ok {
		httpx.WriteError(w, r, http.StatusNotFound, "template_not_found", "Import template not found", nil)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-template.csv\"", normalized))
	_, _ = w.Write([]byte(content))
}

func (s *Server) GetExportsCustomersCsv(w http.ResponseWriter, r *http.Request) {
	s.writeExportCSV(w, r, "customers", "customers.csv", func(writer *csv.Writer, tenantID uuid.UUID) error {
		rows, err := s.Q.ExportCustomersRows(r.Context(), tenantID)
		if err != nil {
			return err
		}
		_ = writer.Write([]string{"id", "first_name", "last_name", "email", "phone", "created_at", "updated_at"})
		for _, row := range rows {
			_ = writer.Write([]string{
				row.ID.String(),
				row.FirstName,
				row.LastName,
				derefString(row.Email),
				derefString(row.Phone),
				row.CreatedAt.UTC().Format(time.RFC3339),
				row.UpdatedAt.UTC().Format(time.RFC3339),
			})
		}
		return nil
	})
}

func (s *Server) GetExportsEstimatesCsv(w http.ResponseWriter, r *http.Request) {
	s.writeExportCSV(w, r, "estimates", "estimates.csv", func(writer *csv.Writer, tenantID uuid.UUID) error {
		rows, err := s.Q.ExportEstimatesRows(r.Context(), tenantID)
		if err != nil {
			return err
		}
		_ = writer.Write([]string{"id", "estimate_number", "customer_name", "email", "primary_phone", "secondary_phone", "status", "origin_city", "origin_state", "origin_postal_code", "destination_city", "destination_state", "destination_postal_code", "move_date", "pickup_time", "lead_source", "estimated_total_cents", "deposit_cents", "notes", "created_at", "updated_at"})
		for _, row := range rows {
			_ = writer.Write([]string{
				row.ID.String(),
				row.EstimateNumber,
				row.CustomerName,
				row.Email,
				row.PrimaryPhone,
				derefString(row.SecondaryPhone),
				row.Status,
				row.OriginCity,
				row.OriginState,
				row.OriginPostalCode,
				row.DestinationCity,
				row.DestinationState,
				row.DestinationPostalCode,
				row.MoveDate.UTC().Format("2006-01-02"),
				derefString(row.PickupTime),
				row.LeadSource,
				formatInt64Ptr(row.EstimatedTotalCents),
				formatInt64Ptr(row.DepositCents),
				derefString(row.Notes),
				row.CreatedAt.UTC().Format(time.RFC3339),
				row.UpdatedAt.UTC().Format(time.RFC3339),
			})
		}
		return nil
	})
}

func (s *Server) GetExportsJobsCsv(w http.ResponseWriter, r *http.Request) {
	s.writeExportCSV(w, r, "jobs", "jobs.csv", func(writer *csv.Writer, tenantID uuid.UUID) error {
		rows, err := s.Q.ExportJobsRows(r.Context(), tenantID)
		if err != nil {
			return err
		}
		_ = writer.Write([]string{"id", "job_number", "status", "scheduled_date", "pickup_time", "customer_name", "customer_email", "customer_phone", "estimate_number", "origin_city", "origin_state", "origin_postal_code", "destination_city", "destination_state", "destination_postal_code", "created_at", "updated_at"})
		for _, row := range rows {
			customerName := strings.TrimSpace(row.FirstName + " " + row.LastName)
			if customerName == "" {
				customerName = "Customer"
			}
			_ = writer.Write([]string{
				row.ID.String(),
				row.JobNumber,
				row.Status,
				formatDatePtrCSV(row.ScheduledDate),
				derefString(row.PickupTime),
				customerName,
				derefString(row.Email),
				derefString(row.Phone),
				derefString(row.EstimateNumber),
				derefString(row.OriginCity),
				derefString(row.OriginState),
				derefString(row.OriginPostalCode),
				derefString(row.DestinationCity),
				derefString(row.DestinationState),
				derefString(row.DestinationPostalCode),
				row.CreatedAt.UTC().Format(time.RFC3339),
				row.UpdatedAt.UTC().Format(time.RFC3339),
			})
		}
		return nil
	})
}

func (s *Server) GetExportsStorageCsv(w http.ResponseWriter, r *http.Request) {
	s.writeExportCSV(w, r, "storage", "storage.csv", func(writer *csv.Writer, tenantID uuid.UUID) error {
		rows, err := s.Q.ExportStorageRows(r.Context(), tenantID)
		if err != nil {
			return err
		}
		_ = writer.Write([]string{"id", "job_number", "facility", "status", "date_in", "date_out", "next_bill_date", "lot_number", "location_label", "vaults", "pads", "items", "oversize_items", "volume", "monthly_rate_cents", "storage_balance_cents", "move_balance_cents", "notes", "created_at", "updated_at"})
		for _, row := range rows {
			_ = writer.Write([]string{
				row.ID.String(),
				row.JobNumber,
				row.Facility,
				row.Status,
				formatDatePtrCSV(row.DateIn),
				formatDatePtrCSV(row.DateOut),
				formatDatePtrCSV(row.NextBillDate),
				derefString(row.LotNumber),
				derefString(row.LocationLabel),
				strconv.Itoa(int(row.Vaults)),
				strconv.Itoa(int(row.Pads)),
				strconv.Itoa(int(row.Items)),
				strconv.Itoa(int(row.OversizeItems)),
				strconv.Itoa(int(row.Volume)),
				formatInt64Ptr(row.MonthlyRateCents),
				strconv.FormatInt(row.StorageBalanceCents, 10),
				strconv.FormatInt(row.MoveBalanceCents, 10),
				derefString(row.Notes),
				row.CreatedAt.UTC().Format(time.RFC3339),
				row.UpdatedAt.UTC().Format(time.RFC3339),
			})
		}
		return nil
	})
}

func (s *Server) writeExportCSV(w http.ResponseWriter, r *http.Request, entityType, filename string, writerFunc func(writer *csv.Writer, tenantID uuid.UUID) error) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	writer := csv.NewWriter(w)
	if err := writerFunc(writer, tenantID); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate export CSV", nil)
		return
	}
	writer.Flush()
	if writer.Error() != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to stream export CSV", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "export.download",
		EntityType: entityType,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"filename": filename,
			"entity":   entityType,
		},
	})
}

type appError struct {
	Status  int
	Code    string
	Message string
	Details any
}

func parseImportUpload(r *http.Request, maxRows int) (parsedImportFile, *appError) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_content_type",
			Message: "Content-Type must be multipart/form-data",
		}
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_multipart",
			Message: "Failed to parse multipart form",
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "missing_file",
			Message: "file is required",
		}
	}
	defer file.Close()

	optionsRaw := strings.TrimSpace(r.FormValue("options"))
	if optionsRaw == "" {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "missing_options",
			Message: "options is required",
		}
	}

	var options importOptionsPayload
	if err := json.Unmarshal([]byte(optionsRaw), &options); err != nil {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_options",
			Message: "options must be valid JSON",
		}
	}
	if options.Source != "granot" && options.Source != "generic" {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "validation_error",
			Message: "options.source must be granot or generic",
		}
	}
	if len(options.Mapping) == 0 {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "validation_error",
			Message: "options.mapping is required",
		}
	}

	hasHeader := true
	if options.HasHeader != nil {
		hasHeader = *options.HasHeader
	}

	filename := header.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))

	switch ext {
	case ".csv":
		if contentType != "" {
			if _, ok := supportedCSVContentTypes[contentType]; !ok {
				return parsedImportFile{}, &appError{
					Status:  http.StatusBadRequest,
					Code:    "invalid_content_type",
					Message: "Unsupported CSV content type",
					Details: map[string]any{"contentType": contentType},
				}
			}
		}
	case ".xlsx":
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "XLSX_NOT_SUPPORTED",
			Message: "XLSX import is not supported in this phase. Please export and upload CSV.",
		}
	default:
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_file_type",
			Message: "Only .csv uploads are supported",
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_file",
			Message: "Failed to read uploaded file",
		}
	}
	digest := sha256.Sum256(data)
	fileSHA256 := hex.EncodeToString(digest[:])

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	rows := make([][]string, 0, 1024)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return parsedImportFile{}, &appError{
				Status:  http.StatusBadRequest,
				Code:    "invalid_csv",
				Message: "CSV parsing failed",
			}
		}
		rows = append(rows, record)
	}
	if len(rows) == 0 {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "empty_file",
			Message: "Uploaded CSV is empty",
		}
	}

	headers := []string{}
	dataRows := rows
	if hasHeader {
		headers = normalizeHeaderRow(rows[0])
		dataRows = rows[1:]
	}

	if maxRows > 0 && len(dataRows) > maxRows {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "row_limit_exceeded",
			Message: "CSV row limit exceeded",
			Details: map[string]any{"maxRows": maxRows},
		}
	}

	mapping, err := resolveColumnMapping(options.Mapping, headers, hasHeader)
	if err != nil {
		return parsedImportFile{}, &appError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_mapping",
			Message: err.Error(),
		}
	}

	return parsedImportFile{
		filename:   filename,
		fileSHA256: fileSHA256,
		options:    options,
		headers:    headers,
		rows:       dataRows,
		mapping:    mapping,
		hasHeader:  hasHeader,
	}, nil
}

func normalizeHeaderRow(row []string) []string {
	headers := make([]string, len(row))
	for i, col := range row {
		trimmed := strings.TrimPrefix(strings.TrimSpace(col), "\ufeff")
		headers[i] = trimmed
	}
	return headers
}

func resolveColumnMapping(mapping map[string]any, headers []string, hasHeader bool) (map[string]int, error) {
	resolved := map[string]int{}
	normalizedHeaders := map[string]int{}
	for idx, header := range headers {
		normalizedHeaders[normalizeHeaderKey(header)] = idx
	}

	for canonicalField, mappedValue := range mapping {
		switch value := mappedValue.(type) {
		case float64:
			index := int(value)
			if index < 0 {
				return nil, fmt.Errorf("mapping for %s has negative index", canonicalField)
			}
			resolved[canonicalField] = index
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if parsedIndex, err := strconv.Atoi(trimmed); err == nil && !hasHeader {
				if parsedIndex < 0 {
					return nil, fmt.Errorf("mapping for %s has negative index", canonicalField)
				}
				resolved[canonicalField] = parsedIndex
				continue
			}
			if hasHeader {
				if idx, ok := normalizedHeaders[normalizeHeaderKey(trimmed)]; ok {
					resolved[canonicalField] = idx
					continue
				}
				if parsedIndex, err := strconv.Atoi(trimmed); err == nil {
					if parsedIndex < 0 {
						return nil, fmt.Errorf("mapping for %s has negative index", canonicalField)
					}
					resolved[canonicalField] = parsedIndex
					continue
				}
			}
			if !hasHeader {
				return nil, fmt.Errorf("mapping for %s must use column index when hasHeader=false", canonicalField)
			}
			return nil, fmt.Errorf("column %q mapped to %s was not found in header", trimmed, canonicalField)
		default:
			return nil, fmt.Errorf("mapping for %s must be string or integer", canonicalField)
		}
	}

	return resolved, nil
}

func normalizeHeaderKey(raw string) string {
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "/", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(raw)))
}

func buildCanonicalImportRow(row []string, mapping map[string]int) canonicalImportRow {
	get := func(key string) string {
		idx, ok := mapping[key]
		if !ok || idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	return canonicalImportRow{
		CustomerName:        get("customer_name"),
		Email:               get("email"),
		PhonePrimary:        get("phone_primary"),
		PhoneSecondary:      get("phone_secondary"),
		EstimateNumber:      get("estimate_number"),
		OriginZIP:           get("origin_zip"),
		DestinationZIP:      get("destination_zip"),
		OriginCity:          get("origin_city"),
		DestinationCity:     get("destination_city"),
		OriginState:         get("origin_state"),
		DestinationState:    get("destination_state"),
		RequestedPickupDate: get("requested_pickup_date"),
		RequestedPickupTime: get("requested_pickup_time"),
		LeadSource:          get("lead_source"),
		EstimatedTotal:      get("estimated_total"),
		Deposit:             get("deposit"),
		PricingNotes:        get("pricing_notes"),
		JobNumber:           get("job_number"),
		ScheduledDate:       get("scheduled_date"),
		PickupTime:          get("pickup_time"),
		Phase:               get("phase"),
		Status:              get("status"),
		JobType:             get("job_type"),
		Facility:            get("facility"),
		StorageStatus:       get("storage_status"),
		DateIn:              get("date_in"),
		DateOut:             get("date_out"),
		NextBillDate:        get("next_bill_date"),
		LotNumber:           get("lot_number"),
		LocationLabel:       get("location_label"),
		Vaults:              get("vaults"),
		Pads:                get("pads"),
		Items:               get("items"),
		OversizeItems:       get("oversize_items"),
		Volume:              get("volume"),
		MonthlyRate:         get("monthly_rate"),
		StorageBalance:      get("storage_balance"),
		MoveBalance:         get("move_balance"),
	}
}

func hasEstimateFields(row canonicalImportRow) bool {
	return row.EstimateNumber != "" ||
		row.OriginZIP != "" ||
		row.DestinationZIP != "" ||
		row.RequestedPickupDate != "" ||
		row.RequestedPickupTime != "" ||
		row.EstimatedTotal != "" ||
		row.Deposit != ""
}

func hasStorageFields(row canonicalImportRow) bool {
	return row.Facility != "" ||
		row.StorageStatus != "" ||
		row.DateIn != "" ||
		row.DateOut != "" ||
		row.NextBillDate != "" ||
		row.LotNumber != "" ||
		row.LocationLabel != "" ||
		row.Vaults != "" ||
		row.Pads != "" ||
		row.Items != "" ||
		row.OversizeItems != "" ||
		row.Volume != "" ||
		row.MonthlyRate != "" ||
		row.StorageBalance != "" ||
		row.MoveBalance != ""
}

func parseFlexibleDate(value string) (*time.Time, string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, "", nil
	}
	formats := []string{"2006-01-02", "01/02/2006", "1/2/2006", "2006/01/02", "01-02-2006", "1-2-2006"}
	for _, format := range formats {
		parsed, err := time.Parse(format, raw)
		if err != nil {
			continue
		}
		date := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		warning := ""
		if (format == "01/02/2006" || format == "1/2/2006" || format == "01-02-2006" || format == "1-2-2006") && strings.ContainsAny(raw, "/-") {
			parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == '-' })
			if len(parts) >= 2 {
				month, _ := strconv.Atoi(parts[0])
				day, _ := strconv.Atoi(parts[1])
				if month >= 1 && month <= 12 && day >= 1 && day <= 12 {
					warning = "Ambiguous date interpreted as MM/DD/YYYY"
				}
			}
		}
		return &date, warning, nil
	}
	return nil, "", fmt.Errorf("invalid date format")
}

func parseMoneyCents(value string) (*int64, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}
	cleaned := strings.NewReplacer("$", "", ",", "", " ", "").Replace(raw)
	if strings.Contains(cleaned, ".") {
		floatValue, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return nil, err
		}
		if floatValue < 0 {
			zero := int64(0)
			return &zero, nil
		}
		rounded := int64(floatValue*100 + 0.5)
		return &rounded, nil
	}

	parsed, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return nil, err
	}
	if parsed < 0 {
		parsed = 0
	}
	return &parsed, nil
}

func parseIntNonNegative(value string) (int, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, nil
	}
	return parsed, nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizePhone(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	digits := make([]rune, 0, len(raw))
	for _, char := range raw {
		if char >= '0' && char <= '9' {
			digits = append(digits, char)
		}
	}
	return string(digits)
}

func buildCustomerKey(customerName, email, phone string) string {
	if email != "" {
		return "email:" + email
	}
	if phone != "" {
		return "phone:" + phone
	}
	return "name:" + strings.ToLower(strings.TrimSpace(customerName))
}

func buildEstimateKey(row canonicalImportRow, customerName, email, phone string) string {
	if strings.TrimSpace(row.EstimateNumber) != "" {
		return "estimate_number:" + strings.TrimSpace(row.EstimateNumber)
	}
	if strings.TrimSpace(row.JobNumber) != "" {
		return "job_number:" + strings.TrimSpace(row.JobNumber)
	}
	customerKey := buildCustomerKey(customerName, email, phone)
	base := strings.Join([]string{customerKey, strings.TrimSpace(row.RequestedPickupDate), strings.TrimSpace(row.OriginZIP), strings.TrimSpace(row.DestinationZIP)}, "|")
	return "estimate_hash:" + shortHash(base, 20)
}

func buildJobKey(row canonicalImportRow, customerID uuid.UUID) string {
	if strings.TrimSpace(row.JobNumber) != "" {
		return "job_number:" + strings.TrimSpace(row.JobNumber)
	}
	base := strings.Join([]string{customerID.String(), strings.TrimSpace(row.ScheduledDate), strings.TrimSpace(row.RequestedPickupDate), strings.TrimSpace(row.OriginZIP), strings.TrimSpace(row.DestinationZIP)}, "|")
	return "job_hash:" + shortHash(base, 20)
}

func normalizeJobStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "booked":
		return "booked"
	case "scheduled":
		return "scheduled"
	case "completed":
		return "completed"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return "booked"
	}
}

func normalizeStorageStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sit":
		return "sit"
	case "out":
		return "out"
	default:
		return "in_storage"
	}
}

func mapOutcome(rowNumber int, outcome rowOutcome) importRowMessage {
	var targetID *openapi_types.UUID
	if outcome.targetEntityID != nil {
		id := openapi_types.UUID(*outcome.targetEntityID)
		targetID = &id
	}
	return importRowMessage{
		RowNumber:      rowNumber,
		Severity:       outcome.severity,
		EntityType:     outcome.entityType,
		Result:         outcome.result,
		IdempotencyKey: outcome.idempotencyKey,
		Field:          outcome.field,
		Message:        outcome.message,
		RawValue:       outcome.rawValue,
		TargetEntityID: targetID,
	}
}

func mapRowResults(rows []gen.ImportRowResult) []oapi.ImportRowMessage {
	items := make([]oapi.ImportRowMessage, 0, len(rows))
	for _, row := range rows {
		var targetID *openapi_types.UUID
		if row.TargetEntityID != nil {
			id := openapi_types.UUID(*row.TargetEntityID)
			targetID = &id
		}
		items = append(items, oapi.ImportRowMessage{
			RowNumber:      int(row.RowNumber),
			Severity:       oapi.ImportRowMessageSeverity(row.Severity),
			EntityType:     oapi.ImportRowMessageEntityType(row.EntityType),
			Result:         oapi.ImportRowMessageResult(row.Result),
			IdempotencyKey: row.IdempotencyKey,
			Field:          row.Field,
			Message:        row.Message,
			RawValue:       row.RawValue,
			TargetEntityId: targetID,
		})
	}
	return items
}

func topOutcomesBySeverity(outcomes []importRowMessage, severity string, limit int) []oapi.ImportRowMessage {
	filtered := make([]importRowMessage, 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.Severity == severity {
			filtered = append(filtered, outcome)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].RowNumber == filtered[j].RowNumber {
			return filtered[i].EntityType < filtered[j].EntityType
		}
		return filtered[i].RowNumber < filtered[j].RowNumber
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	result := make([]oapi.ImportRowMessage, 0, len(filtered))
	for _, outcome := range filtered {
		result = append(result, oapi.ImportRowMessage{
			RowNumber:      outcome.RowNumber,
			Severity:       oapi.ImportRowMessageSeverity(outcome.Severity),
			EntityType:     oapi.ImportRowMessageEntityType(outcome.EntityType),
			Result:         oapi.ImportRowMessageResult(outcome.Result),
			IdempotencyKey: outcome.IdempotencyKey,
			Field:          outcome.Field,
			Message:        outcome.Message,
			RawValue:       outcome.RawValue,
			TargetEntityId: outcome.TargetEntityID,
		})
	}
	return result
}

func mapImportRunResponse(run gen.ImportRun, summary importRunSummary, topWarnings, topErrors []oapi.ImportRowMessage, requestID string) oapi.ImportRunResponse {
	downloads := oapi.ImportDownloadUrls{
		ErrorsCsv:  fmt.Sprintf("/api/imports/%s/errors.csv", run.ID.String()),
		ReportJson: fmt.Sprintf("/api/imports/%s/report.json", run.ID.String()),
	}
	response := oapi.ImportRunResponse{
		ImportRunId: run.ID,
		Mode:        oapi.ImportMode(run.Mode),
		Status:      oapi.ImportRunStatus(run.Status),
		Source:      oapi.ImportSource(run.Source),
		Filename:    run.Filename,
		Summary: oapi.ImportSummary{
			RowsTotal: int(summary.RowsTotal),
			RowsValid: int(summary.RowsValid),
			RowsError: int(summary.RowsError),
			Customer: oapi.ImportResultCounts{
				Created: int(summary.Customer.Created),
				Updated: int(summary.Customer.Updated),
				Skipped: int(summary.Customer.Skipped),
				Error:   int(summary.Customer.Error),
			},
			Estimate: oapi.ImportResultCounts{
				Created: int(summary.Estimate.Created),
				Updated: int(summary.Estimate.Updated),
				Skipped: int(summary.Estimate.Skipped),
				Error:   int(summary.Estimate.Error),
			},
			Job: oapi.ImportResultCounts{
				Created: int(summary.Job.Created),
				Updated: int(summary.Job.Updated),
				Skipped: int(summary.Job.Skipped),
				Error:   int(summary.Job.Error),
			},
			StorageRecord: oapi.ImportResultCounts{
				Created: int(summary.StorageRecord.Created),
				Updated: int(summary.StorageRecord.Updated),
				Skipped: int(summary.StorageRecord.Skipped),
				Error:   int(summary.StorageRecord.Error),
			},
		},
		TopWarnings:  topWarnings,
		TopErrors:    topErrors,
		DownloadUrls: downloads,
		CreatedAt:    run.CreatedAt.UTC(),
		RequestId:    requestID,
	}
	if run.CompletedAt != nil {
		completed := run.CompletedAt.UTC()
		response.CompletedAt = &completed
	}
	return response
}

func parseImportSummary(raw []byte) importRunSummary {
	summary := importRunSummary{}
	if len(raw) == 0 {
		return summary
	}
	_ = json.Unmarshal(raw, &summary)
	return summary
}

func incrementSummary(summary *importRunSummary, entity, result string) {
	var counts *importResultCounts
	switch entity {
	case "customer":
		counts = &summary.Customer
	case "estimate":
		counts = &summary.Estimate
	case "job":
		counts = &summary.Job
	case "storage_record":
		counts = &summary.StorageRecord
	default:
		return
	}

	switch result {
	case "created":
		counts.Created++
	case "updated":
		counts.Updated++
	case "error":
		counts.Error++
	default:
		counts.Skipped++
	}
}

func truncateText(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func shortHash(value string, size int) string {
	sum := sha256.Sum256([]byte(value))
	encoded := hex.EncodeToString(sum[:])
	if size <= 0 || size >= len(encoded) {
		return encoded
	}
	return encoded[:size]
}

func nonEmpty(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func stringPtr(value string) *string {
	return &value
}

func stringPtrOrNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func int32Ptr(value int) *int32 {
	v := int32(value)
	return &v
}

func int64Ptr(value int64) *int64 {
	return &value
}

func defaultInt64(value *int64, fallback int64) int64 {
	if value == nil {
		return fallback
	}
	return *value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatInt64Ptr(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func formatDatePtrCSV(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format("2006-01-02")
}

var importTemplates = map[string]string{
	"customers": strings.Join([]string{
		"customer_name,email,phone_primary,phone_secondary",
		"Jane Doe,jane@example.com,5125550100,",
	}, "\n"),
	"estimates": strings.Join([]string{
		"estimate_number,customer_name,email,phone_primary,origin_zip,destination_zip,origin_city,destination_city,requested_pickup_date,requested_pickup_time,lead_source,estimated_total,deposit,pricing_notes",
		"E-LEG-001,Jane Doe,jane@example.com,5125550100,78701,75001,Austin,Dallas,2026-03-22,09:00,Referral,250000,25000,Legacy import",
	}, "\n"),
	"jobs": strings.Join([]string{
		"job_number,customer_name,email,phone_primary,scheduled_date,pickup_time,status,job_type,origin_zip,destination_zip,origin_city,destination_city,requested_pickup_date",
		"J-LEG-001,Jane Doe,jane@example.com,5125550100,2026-03-22,09:00,booked,local,78701,75001,Austin,Dallas,2026-03-22",
	}, "\n"),
	"storage": strings.Join([]string{
		"job_number,facility,storage_status,date_in,date_out,next_bill_date,lot_number,location_label,vaults,pads,items,oversize_items,volume,monthly_rate,storage_balance,move_balance",
		"J-LEG-001,Main Facility,in_storage,2026-03-23,,2026-04-23,LOT-12,Aisle 4,2,1,18,2,120,32900,28000,7000",
	}, "\n"),
	"combined": strings.Join([]string{
		"job_number,estimate_number,customer_name,email,phone_primary,phone_secondary,origin_zip,destination_zip,origin_city,destination_city,origin_state,destination_state,requested_pickup_date,requested_pickup_time,scheduled_date,pickup_time,status,job_type,lead_source,estimated_total,deposit,pricing_notes,facility,storage_status,date_in,date_out,next_bill_date,lot_number,location_label,vaults,pads,items,oversize_items,volume,monthly_rate,storage_balance,move_balance",
		"J-LEG-001,E-LEG-001,Jane Doe,jane@example.com,5125550100,,78701,75001,Austin,Dallas,TX,TX,2026-03-22,09:00,2026-03-22,09:00,booked,local,Referral,250000,25000,Legacy import,Main Facility,in_storage,2026-03-23,,2026-04-23,LOT-12,Aisle 4,2,1,18,2,120,32900,28000,7000",
	}, "\n"),
}
