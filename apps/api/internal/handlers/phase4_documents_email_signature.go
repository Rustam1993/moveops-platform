package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/moveops-platform/apps/api/internal/audit"
	"github.com/moveops-platform/apps/api/internal/auth"
	"github.com/moveops-platform/apps/api/internal/config"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
	"github.com/moveops-platform/apps/api/internal/gen/oapi"
	"github.com/moveops-platform/apps/api/internal/httpx"
	"github.com/moveops-platform/apps/api/internal/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/phpdave11/gofpdf"
)

const (
	emailLogLimit = 100
)

var (
	errExpiredQuoteToken     = errors.New("quote token expired")
	errExpiredSignatureToken = errors.New("signature token expired")
)

type signatureStamp struct {
	SignerName  string
	SignerEmail string
	SignedAt    time.Time
	Signature   string
}

type emailDeliveryResult struct {
	Mode              string
	Status            string
	ProviderMessageID *string
	ErrorMessage      *string
}

func (s *Server) PostEstimatesEstimateIdDocumentsEstimatePdf(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	_, document, err := s.generateAndStoreEstimatePDFDocument(r.Context(), s.Q, tenantID, estimateID, &userID, nil)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
		default:
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate estimate PDF", nil)
		}
		return
	}

	mapped, err := mapEstimateDocument(document)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to render document payload", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, oapi.EstimateDocumentResponse{
		Document:  mapped,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetEstimatesEstimateIdDocumentsEstimatePdf(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
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

	document, err := s.Q.GetLatestEstimateDocumentByType(r.Context(), gen.GetLatestEstimateDocumentByTypeParams{
		TenantID:     tenantID,
		EstimateID:   estimateID,
		DocumentType: string(oapi.EstimatePdf),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "document_not_found", "Estimate PDF has not been generated yet", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate PDF", nil)
		return
	}

	mapped, err := mapEstimateDocument(document)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to render document payload", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateDocumentResponse{
		Document:  mapped,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetEstimatesEstimateIdEmails(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
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

	rows, err := s.Q.ListEstimateEmailLogs(r.Context(), gen.ListEstimateEmailLogsParams{
		TenantID:   tenantID,
		EstimateID: estimateID,
		LimitRows:  emailLogLimit,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load email history", nil)
		return
	}

	emails := make([]oapi.EstimateEmailLog, 0, len(rows))
	for _, row := range rows {
		mapped, mapErr := mapEstimateEmailLog(row)
		if mapErr != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to render email history", nil)
			return
		}
		emails = append(emails, mapped)
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateEmailLogListResponse{
		Emails:    emails,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostEstimatesEstimateIdEmailsSend(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
	actor, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.SendEstimateEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: estimateID, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	recipient := strings.TrimSpace(estimate.Email)
	if req.ToEmail != nil {
		recipient = strings.TrimSpace(string(*req.ToEmail))
	}
	if recipient == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "missing_customer_email", "Estimate has no customer email", nil)
		return
	}

	var ccEmail *string
	if req.CcMe != nil && *req.CcMe {
		email := strings.TrimSpace(actor.Email)
		if email != "" {
			ccEmail = &email
		}
	}

	subject, body, links, renderedVars, err := s.prepareEstimateEmailTemplate(r.Context(), tenantID, userID, estimate, req.TemplateKey, recipient)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
		default:
			httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
		}
		return
	}

	delivery := s.sendTransactionalEmail(r, recipient, ccEmail, subject, body)

	renderedJSON, _ := json.Marshal(renderedVars)
	emailLog, err := s.Q.CreateEstimateEmailLog(r.Context(), gen.CreateEstimateEmailLogParams{
		TenantID:          tenantID,
		EstimateID:        estimateID,
		TemplateKey:       string(req.TemplateKey),
		EmailTo:           recipient,
		EmailCc:           ccEmail,
		EmailFrom:         s.Config.EmailFrom,
		Subject:           subject,
		Status:            delivery.Status,
		ProviderMessageID: delivery.ProviderMessageID,
		DeliveryMode:      delivery.Mode,
		ErrorMessage:      delivery.ErrorMessage,
		RenderedJson:      renderedJSON,
		CreatedBy:         &userID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to write email log", nil)
		return
	}

	auditAction := "email.sent"
	if delivery.Status == string(oapi.Failed) {
		auditAction = "email.failed"
	}
	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     auditAction,
		EntityType: "estimate",
		EntityID:   &estimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"templateKey":  req.TemplateKey,
			"to":           recipient,
			"status":       delivery.Status,
			"deliveryMode": delivery.Mode,
		},
	})

	mappedEmail, err := mapEstimateEmailLog(emailLog)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to render email response", nil)
		return
	}

	resp := oapi.SendEstimateEmailResponse{
		Email:     mappedEmail,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	}
	if links != nil {
		resp.GeneratedLinks = links
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) PostEstimatesEstimateIdSignatureRequests(w http.ResponseWriter, r *http.Request, estimateID uuid.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	req := oapi.CreateSignatureRequestRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
			return
		}
	}

	expiresAt, err := resolveTokenExpiry(s.Config.SignRequestTTL, req.ExpiresInDays)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{ID: estimateID, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate", nil)
		return
	}

	recipient := strings.TrimSpace(estimate.Email)
	if recipient == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "missing_customer_email", "Estimate has no customer email", nil)
		return
	}

	signatureRequest, signatureURL, err := s.createSignatureRequest(r.Context(), tenantID, estimate, &userID, recipient, expiresAt)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create signature request", nil)
		return
	}

	subject := "Please sign your estimate"
	body := "Please sign your estimate using this secure link:\n" + signatureURL + "\n\nMoveOps"
	delivery := s.sendTransactionalEmail(r, recipient, nil, subject, body)

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "signature.requested",
		EntityType: "estimate",
		EntityID:   &estimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"signatureRequestId": signatureRequest.ID,
			"recipientEmail":     recipient,
			"deliveryMode":       delivery.Mode,
		},
	})

	httpx.WriteJSON(w, http.StatusCreated, oapi.CreateSignatureRequestResponse{
		SignatureRequestId: signatureRequest.ID,
		EstimateId:         estimateID,
		RecipientEmail:     openapi_types.Email(recipient),
		SignatureUrl:       signatureURL,
		ExpiresAt:          signatureRequest.ExpiresAt.UTC(),
		DeliveryMode:       oapi.CreateSignatureRequestResponseDeliveryMode(delivery.Mode),
		RequestId:          middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetPublicEstimateToken(w http.ResponseWriter, r *http.Request, token string) {
	share, estimate, document, err := s.resolvePublicQuotePayload(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.WriteError(w, r, http.StatusNotFound, "quote_share_not_found", "Estimate link is invalid", nil)
		case errors.Is(err, errExpiredQuoteToken):
			httpx.WriteError(w, r, http.StatusGone, "quote_share_expired", "Estimate link expired", nil)
		default:
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load estimate link", nil)
		}
		return
	}

	now := time.Now().UTC()
	_, _ = s.Q.TouchEstimateQuoteShareLink(r.Context(), gen.TouchEstimateQuoteShareLinkParams{
		ID:             share.ID,
		TenantID:       share.TenantID,
		LastAccessedAt: &now,
	})

	mappedDoc, err := mapEstimateDocument(document)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to render estimate document", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.PublicEstimateDocumentResponse{
		EstimateId:         estimate.ID,
		CustomerName:       estimate.CustomerName,
		MoveDate:           dateOnly(estimate.MoveDate),
		TotalVolumeCf:      roundCF(estimate.TotalVolumeCf),
		TotalEstimateCents: estimate.EstimatedTotalCents,
		Document:           mappedDoc,
		ExpiresAt:          share.ExpiresAt.UTC(),
		RequestId:          middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) GetPublicSignToken(w http.ResponseWriter, r *http.Request, token string) {
	signReq, estimate, document, err := s.resolvePublicSignPayload(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.WriteError(w, r, http.StatusNotFound, "signature_request_not_found", "Signature link is invalid", nil)
		case errors.Is(err, errExpiredSignatureToken):
			httpx.WriteError(w, r, http.StatusGone, "signature_request_expired", "Signature link expired", nil)
		default:
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load signature request", nil)
		}
		return
	}

	now := time.Now().UTC()
	_, _ = s.Q.TouchEstimateSignatureRequest(r.Context(), gen.TouchEstimateSignatureRequestParams{
		ID:             signReq.ID,
		TenantID:       signReq.TenantID,
		LastAccessedAt: &now,
	})

	mappedDoc, err := mapEstimateDocument(document)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to render signature document", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.PublicSignContextResponse{
		EstimateId:         estimate.ID,
		CustomerName:       estimate.CustomerName,
		MoveDate:           dateOnly(estimate.MoveDate),
		TotalVolumeCf:      roundCF(estimate.TotalVolumeCf),
		TotalEstimateCents: estimate.EstimatedTotalCents,
		Document:           mappedDoc,
		SignerEmail:        openapi_types.Email(signReq.RecipientEmail),
		ExpiresAt:          signReq.ExpiresAt.UTC(),
		AlreadySigned:      signReq.UsedAt != nil,
		RequestId:          middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PostPublicSignToken(w http.ResponseWriter, r *http.Request, token string) {
	var req oapi.CompleteSignatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	req.SignerName = strings.TrimSpace(req.SignerName)
	req.SignerEmail = openapi_types.Email(strings.TrimSpace(string(req.SignerEmail)))
	req.SignatureText = strings.TrimSpace(req.SignatureText)

	if req.SignerName == "" || req.SignatureText == "" || strings.TrimSpace(string(req.SignerEmail)) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "signerName, signerEmail, and signatureText are required", nil)
		return
	}
	if !req.AgreeToTerms {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", "You must agree to terms before signing", nil)
		return
	}

	signReq, _, _, err := s.resolvePublicSignPayload(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.WriteError(w, r, http.StatusNotFound, "signature_request_not_found", "Signature link is invalid", nil)
		case errors.Is(err, errExpiredSignatureToken):
			httpx.WriteError(w, r, http.StatusGone, "signature_request_expired", "Signature link expired", nil)
		default:
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load signature request", nil)
		}
		return
	}

	logSignatureFailed := func(reason string) {
		_ = s.Audit.Log(r.Context(), audit.Entry{
			TenantID:   signReq.TenantID,
			Action:     "signature.failed",
			EntityType: "estimate",
			EntityID:   &signReq.EstimateID,
			RequestID:  middleware.RequestIDFromContext(r.Context()),
			Metadata: map[string]any{
				"signatureRequestId": signReq.ID,
				"reason":             reason,
			},
		})
	}

	if signReq.UsedAt != nil {
		logSignatureFailed("already_completed")
		httpx.WriteError(w, r, http.StatusConflict, "signature_already_completed", "This signature request has already been completed", nil)
		return
	}

	if !strings.EqualFold(strings.TrimSpace(string(req.SignerEmail)), signReq.RecipientEmail) {
		logSignatureFailed("email_mismatch")
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "signature_email_mismatch", "Signer email must match request recipient", nil)
		return
	}

	now := time.Now().UTC()
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		logSignatureFailed("transaction_start_failed")
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start signature transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.Q.WithTx(tx)

	signedEstimate, signedDoc, err := s.generateAndStoreEstimatePDFDocument(
		r.Context(),
		qtx,
		signReq.TenantID,
		signReq.EstimateID,
		nil,
		&signatureStamp{SignerName: req.SignerName, SignerEmail: string(req.SignerEmail), SignedAt: now, Signature: req.SignatureText},
	)
	if err != nil {
		logSignatureFailed("signed_document_generation_failed")
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate signed document", nil)
		return
	}

	affected, err := qtx.MarkEstimateSignatureRequestUsed(r.Context(), gen.MarkEstimateSignatureRequestUsedParams{
		UsedAt:   &now,
		ID:       signReq.ID,
		TenantID: signReq.TenantID,
	})
	if err != nil {
		logSignatureFailed("mark_request_used_failed")
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update signature request", nil)
		return
	}
	if affected == 0 {
		logSignatureFailed("already_completed")
		httpx.WriteError(w, r, http.StatusConflict, "signature_already_completed", "This signature request has already been completed", nil)
		return
	}

	ipAddress := clientIP(r)
	var userAgent *string
	if raw := strings.TrimSpace(r.Header.Get("User-Agent")); raw != "" {
		userAgent = &raw
	}

	signatureRow, err := qtx.CreateEstimateSignature(r.Context(), gen.CreateEstimateSignatureParams{
		TenantID:           signReq.TenantID,
		EstimateID:         signedEstimate.ID,
		SignatureRequestID: signReq.ID,
		DocumentID:         &signedDoc.ID,
		SignerName:         req.SignerName,
		SignerEmail:        strings.TrimSpace(string(req.SignerEmail)),
		SignatureType:      "typed",
		SignatureValue:     req.SignatureText,
		AgreedTerms:        req.AgreeToTerms,
		IpAddress:          ipAddress,
		UserAgent:          userAgent,
		SignedAt:           now,
	})
	if err != nil {
		logSignatureFailed("signature_insert_failed")
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to save signature", nil)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		logSignatureFailed("transaction_commit_failed")
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit signature", nil)
		return
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   signReq.TenantID,
		Action:     "signature.completed",
		EntityType: "estimate",
		EntityID:   &signedEstimate.ID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"signatureRequestId": signReq.ID,
			"signatureId":        signatureRow.ID,
			"signerEmail":        signatureRow.SignerEmail,
			"signedAt":           signatureRow.SignedAt.UTC().Format(time.RFC3339),
		},
	})

	mappedDoc, err := mapEstimateDocument(signedDoc)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to render signed document", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.CompleteSignatureResponse{
		EstimateId:  signedEstimate.ID,
		SignatureId: signatureRow.ID,
		SignerName:  signatureRow.SignerName,
		SignerEmail: openapi_types.Email(signatureRow.SignerEmail),
		SignedAt:    signatureRow.SignedAt.UTC(),
		Document:    mappedDoc,
		RequestId:   middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) prepareEstimateEmailTemplate(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	estimate gen.Estimate,
	templateKey oapi.EstimateEmailTemplateKey,
	recipientEmail string,
) (string, string, *oapi.EstimateEmailGeneratedLinks, map[string]any, error) {
	rendered := map[string]any{
		"estimateId": estimate.ID,
		"customer":   estimate.CustomerName,
	}

	switch templateKey {
	case oapi.MovingEstimate:
		doc, err := s.ensureEstimatePDFDocument(ctx, tenantID, estimate.ID, &userID)
		if err != nil {
			return "", "", nil, nil, err
		}
		token, tokenHash, quoteURL, err := s.newPublicTokenURL("/public/estimate/")
		if err != nil {
			return "", "", nil, nil, err
		}
		share, err := s.Q.CreateEstimateQuoteShareLink(ctx, gen.CreateEstimateQuoteShareLinkParams{
			TenantID:       tenantID,
			EstimateID:     estimate.ID,
			DocumentID:     &doc.ID,
			TokenHash:      tokenHash,
			RecipientEmail: recipientEmail,
			ExpiresAt:      time.Now().UTC().Add(s.Config.QuoteShareTTL),
			CreatedBy:      &userID,
		})
		if err != nil {
			_ = token
			return "", "", nil, nil, err
		}
		_ = share
		subject := "Your moving estimate"
		body := "Your moving estimate is ready.\n\n" +
			"View and download your estimate here:\n" + quoteURL + "\n\n" +
			"Thank you,\nMoveOps"
		links := &oapi.EstimateEmailGeneratedLinks{QuoteUrl: &quoteURL}
		rendered["quoteUrl"] = quoteURL
		return subject, body, links, rendered, nil
	case oapi.UpdateInventory:
		token, tokenHash, shareURL, err := s.newPublicTokenURL("/public/inventory/")
		if err != nil {
			return "", "", nil, nil, err
		}
		_ = token
		share, err := s.Q.CreateEstimateInventoryShareLink(ctx, gen.CreateEstimateInventoryShareLinkParams{
			TenantID:       tenantID,
			EstimateID:     estimate.ID,
			TokenHash:      tokenHash,
			RecipientEmail: recipientEmail,
			DeliveryMode:   "log",
			ExpiresAt:      time.Now().UTC().Add(s.Config.InventoryShareTTL),
			CreatedBy:      &userID,
		})
		if err != nil {
			return "", "", nil, nil, err
		}
		_ = s.Audit.Log(ctx, audit.Entry{
			TenantID:   tenantID,
			UserID:     &userID,
			Action:     "inventory.share_link.created",
			EntityType: "estimate",
			EntityID:   &estimate.ID,
			Metadata: map[string]any{
				"shareLinkId":    share.ID,
				"recipientEmail": recipientEmail,
				"deliveryMode":   "email_center",
			},
		})
		subject := "Please update your inventory"
		body := "Please complete your inventory for your move.\n\n" +
			"Use this secure link:\n" + shareURL + "\n\n" +
			"Thank you,\nMoveOps"
		links := &oapi.EstimateEmailGeneratedLinks{InventoryUrl: &shareURL}
		rendered["inventoryUrl"] = shareURL
		return subject, body, links, rendered, nil
	case oapi.SignatureRequest:
		expiresAt := time.Now().UTC().Add(s.Config.SignRequestTTL)
		signReq, signatureURL, err := s.createSignatureRequest(ctx, tenantID, estimate, &userID, recipientEmail, expiresAt)
		if err != nil {
			return "", "", nil, nil, err
		}
		_ = s.Audit.Log(ctx, audit.Entry{
			TenantID:   tenantID,
			UserID:     &userID,
			Action:     "signature.requested",
			EntityType: "estimate",
			EntityID:   &estimate.ID,
			RequestID:  middleware.RequestIDFromContext(ctx),
			Metadata: map[string]any{
				"signatureRequestId": signReq.ID,
				"recipientEmail":     recipientEmail,
				"via":                "email_center",
			},
		})
		subject := "Please sign your estimate"
		body := "Please sign your estimate using the secure link below.\n\n" +
			"Sign here:\n" + signatureURL + "\n\n" +
			"Thank you,\nMoveOps"
		links := &oapi.EstimateEmailGeneratedLinks{SignatureUrl: &signatureURL}
		rendered["signatureUrl"] = signatureURL
		return subject, body, links, rendered, nil
	case oapi.CreditCardAuthorization:
		subject := "Credit card authorization form"
		body := "A credit card authorization workflow will be available in a future release.\n\nMoveOps"
		return subject, body, nil, rendered, nil
	case oapi.WaiverCancellation:
		subject := "Waiver of cancellation period"
		body := "A cancellation waiver workflow will be available in a future release.\n\nMoveOps"
		return subject, body, nil, rendered, nil
	case oapi.FollowUpMove:
		subject := "Follow-up on your move"
		body := "Thank you for choosing MoveOps. A representative will follow up with you shortly.\n\nMoveOps"
		return subject, body, nil, rendered, nil
	default:
		return "", "", nil, nil, fmt.Errorf("unsupported templateKey: %s", templateKey)
	}
}

func (s *Server) createSignatureRequest(
	ctx context.Context,
	tenantID uuid.UUID,
	estimate gen.Estimate,
	createdBy *uuid.UUID,
	recipientEmail string,
	expiresAt time.Time,
) (gen.EstimateSignatureRequest, string, error) {
	doc, err := s.ensureEstimatePDFDocument(ctx, tenantID, estimate.ID, createdBy)
	if err != nil {
		return gen.EstimateSignatureRequest{}, "", err
	}

	_, tokenHash, signatureURL, err := s.newPublicTokenURL("/public/sign/")
	if err != nil {
		return gen.EstimateSignatureRequest{}, "", err
	}

	signReq, err := s.Q.CreateEstimateSignatureRequest(ctx, gen.CreateEstimateSignatureRequestParams{
		TenantID:       tenantID,
		EstimateID:     estimate.ID,
		DocumentID:     &doc.ID,
		TokenHash:      tokenHash,
		RecipientEmail: recipientEmail,
		ExpiresAt:      expiresAt,
		CreatedBy:      createdBy,
	})
	if err != nil {
		return gen.EstimateSignatureRequest{}, "", err
	}

	return signReq, signatureURL, nil
}

func (s *Server) ensureEstimatePDFDocument(ctx context.Context, tenantID, estimateID uuid.UUID, generatedBy *uuid.UUID) (gen.EstimateDocument, error) {
	doc, err := s.Q.GetLatestEstimateDocumentByType(ctx, gen.GetLatestEstimateDocumentByTypeParams{
		TenantID:     tenantID,
		EstimateID:   estimateID,
		DocumentType: string(oapi.EstimatePdf),
	})
	if err == nil {
		return doc, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return gen.EstimateDocument{}, err
	}

	_, doc, err = s.generateAndStoreEstimatePDFDocument(ctx, s.Q, tenantID, estimateID, generatedBy, nil)
	return doc, err
}

func (s *Server) resolvePublicQuotePayload(ctx context.Context, token string) (gen.EstimateQuoteShareLink, gen.Estimate, gen.EstimateDocument, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return gen.EstimateQuoteShareLink{}, gen.Estimate{}, gen.EstimateDocument{}, pgx.ErrNoRows
	}

	share, err := s.Q.GetEstimateQuoteShareLinkByTokenHash(ctx, auth.HashToken(trimmed))
	if err != nil {
		return gen.EstimateQuoteShareLink{}, gen.Estimate{}, gen.EstimateDocument{}, err
	}
	if time.Now().UTC().After(share.ExpiresAt.UTC()) {
		return gen.EstimateQuoteShareLink{}, gen.Estimate{}, gen.EstimateDocument{}, errExpiredQuoteToken
	}

	estimate, err := s.Q.GetEstimateByID(ctx, gen.GetEstimateByIDParams{ID: share.EstimateID, TenantID: share.TenantID})
	if err != nil {
		return gen.EstimateQuoteShareLink{}, gen.Estimate{}, gen.EstimateDocument{}, err
	}

	var document gen.EstimateDocument
	if share.DocumentID != nil {
		document, err = s.Q.GetEstimateDocumentByID(ctx, gen.GetEstimateDocumentByIDParams{ID: *share.DocumentID, TenantID: share.TenantID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return gen.EstimateQuoteShareLink{}, gen.Estimate{}, gen.EstimateDocument{}, err
		}
	}
	if document.ID == uuid.Nil {
		document, err = s.ensureEstimatePDFDocument(ctx, share.TenantID, share.EstimateID, nil)
		if err != nil {
			return gen.EstimateQuoteShareLink{}, gen.Estimate{}, gen.EstimateDocument{}, err
		}
	}

	return share, estimate, document, nil
}

func (s *Server) resolvePublicSignPayload(ctx context.Context, token string) (gen.EstimateSignatureRequest, gen.Estimate, gen.EstimateDocument, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return gen.EstimateSignatureRequest{}, gen.Estimate{}, gen.EstimateDocument{}, pgx.ErrNoRows
	}

	signReq, err := s.Q.GetEstimateSignatureRequestByTokenHash(ctx, auth.HashToken(trimmed))
	if err != nil {
		return gen.EstimateSignatureRequest{}, gen.Estimate{}, gen.EstimateDocument{}, err
	}
	if time.Now().UTC().After(signReq.ExpiresAt.UTC()) {
		return gen.EstimateSignatureRequest{}, gen.Estimate{}, gen.EstimateDocument{}, errExpiredSignatureToken
	}

	estimate, err := s.Q.GetEstimateByID(ctx, gen.GetEstimateByIDParams{ID: signReq.EstimateID, TenantID: signReq.TenantID})
	if err != nil {
		return gen.EstimateSignatureRequest{}, gen.Estimate{}, gen.EstimateDocument{}, err
	}

	var document gen.EstimateDocument
	if signReq.UsedAt != nil {
		document, err = s.Q.GetLatestEstimateDocumentByType(ctx, gen.GetLatestEstimateDocumentByTypeParams{
			TenantID:     signReq.TenantID,
			EstimateID:   signReq.EstimateID,
			DocumentType: string(oapi.SignedEstimatePdf),
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return gen.EstimateSignatureRequest{}, gen.Estimate{}, gen.EstimateDocument{}, err
		}
	}
	if document.ID == uuid.Nil {
		if signReq.DocumentID != nil {
			document, err = s.Q.GetEstimateDocumentByID(ctx, gen.GetEstimateDocumentByIDParams{ID: *signReq.DocumentID, TenantID: signReq.TenantID})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return gen.EstimateSignatureRequest{}, gen.Estimate{}, gen.EstimateDocument{}, err
			}
		}
	}
	if document.ID == uuid.Nil {
		document, err = s.ensureEstimatePDFDocument(ctx, signReq.TenantID, signReq.EstimateID, nil)
		if err != nil {
			return gen.EstimateSignatureRequest{}, gen.Estimate{}, gen.EstimateDocument{}, err
		}
	}

	return signReq, estimate, document, nil
}

func (s *Server) newPublicTokenURL(pathPrefix string) (string, string, string, error) {
	token, err := auth.GenerateToken()
	if err != nil {
		return "", "", "", err
	}
	tokenHash := auth.HashToken(token)
	url := fmt.Sprintf("%s%s%s", s.publicBaseURL(), strings.TrimRight(pathPrefix, "/"), "/"+token)
	return token, tokenHash, url, nil
}

func (s *Server) publicBaseURL() string {
	base := strings.TrimSpace(s.Config.PublicWebBaseURL)
	if base == "" {
		return "http://localhost:3000"
	}
	return strings.TrimRight(base, "/")
}

func resolveTokenExpiry(defaultTTL time.Duration, expiresInDays *int) (time.Time, error) {
	if expiresInDays == nil {
		return time.Now().UTC().Add(defaultTTL), nil
	}
	days := *expiresInDays
	if days < 1 || days > 30 {
		return time.Time{}, errors.New("expiresInDays must be between 1 and 30")
	}
	return time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour), nil
}

func (s *Server) sendTransactionalEmail(r *http.Request, to string, cc *string, subject, body string) emailDeliveryResult {
	return s.sendTransactionalEmailWithContext(r.Context(), to, cc, subject, body)
}

func (s *Server) sendTransactionalEmailWithContext(ctx context.Context, to string, cc *string, subject, body string) emailDeliveryResult {
	mode := strings.ToLower(strings.TrimSpace(s.Config.EmailMode))
	if mode == "smtp" {
		if strings.TrimSpace(s.Config.SMTPHost) == "" {
			errMsg := "smtp mode configured but SMTP_HOST is empty"
			s.Logger.Warn("smtp unavailable, email logged instead", "error", errMsg)
			s.Logger.Info("transactional email log fallback", "to", to, "cc", safeString(cc), "subject", subject)
			return emailDeliveryResult{Mode: string(oapi.Log), Status: string(oapi.Failed), ErrorMessage: &errMsg}
		}
		if err := sendSMTPEmailRich(s.Config, to, cc, subject, body); err != nil {
			errMsg := err.Error()
			s.Logger.Warn("smtp send failed, email logged instead", "error", errMsg)
			s.Logger.Info("transactional email log fallback", "to", to, "cc", safeString(cc), "subject", subject)
			return emailDeliveryResult{Mode: string(oapi.Log), Status: string(oapi.Failed), ErrorMessage: &errMsg}
		}
		_ = ctx
		return emailDeliveryResult{Mode: string(oapi.Smtp), Status: string(oapi.Sent)}
	}

	s.Logger.Info("transactional email logged", "to", to, "cc", safeString(cc), "subject", subject, "body", body)
	return emailDeliveryResult{Mode: string(oapi.Log), Status: string(oapi.Sent)}
}

func sendSMTPEmailRich(cfg config.Config, recipientEmail string, ccEmail *string, subject string, body string) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	fromAddress, err := sanitizeSMTPAddress(cfg.EmailFrom)
	if err != nil {
		return fmt.Errorf("invalid EMAIL_FROM: %w", err)
	}
	toAddress, err := sanitizeSMTPAddress(recipientEmail)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %w", err)
	}
	recipients := []string{toAddress}

	var ccAddress *string
	if ccEmail != nil && strings.TrimSpace(*ccEmail) != "" {
		cleanCC, err := sanitizeSMTPAddress(*ccEmail)
		if err != nil {
			return fmt.Errorf("invalid cc email: %w", err)
		}
		ccAddress = &cleanCC
		recipients = append(recipients, cleanCC)
	}

	cleanSubject, err := sanitizeSMTPHeaderValue(subject)
	if err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	var replyToHeader *string
	if strings.TrimSpace(cfg.EmailReplyTo) != "" {
		cleanReplyTo, err := sanitizeSMTPAddress(cfg.EmailReplyTo)
		if err != nil {
			return fmt.Errorf("invalid EMAIL_REPLY_TO: %w", err)
		}
		replyToHeader = &cleanReplyTo
	}

	headers := []string{
		"From: " + fromAddress,
		"To: undisclosed-recipients:;",
		"Subject: " + mime.QEncoding.Encode("utf-8", cleanSubject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
	}
	if replyToHeader != nil {
		headers = append(headers, "Reply-To: "+*replyToHeader)
	}

	cleanBody := strings.ReplaceAll(body, "\x00", "")
	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + cleanBody + "\r\n")

	var auth smtp.Auth
	if strings.TrimSpace(cfg.SMTPUser) != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}

	return smtp.SendMail(addr, auth, fromAddress, recipients, msg)
}

func sanitizeSMTPAddress(raw string) (string, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", errors.New("address is empty")
	}
	if strings.ContainsAny(candidate, "\r\n") {
		return "", errors.New("address contains prohibited newline characters")
	}
	parsed, err := mail.ParseAddress(candidate)
	if err != nil {
		return "", err
	}
	return parsed.Address, nil
}

func sanitizeSMTPHeaderValue(raw string) (string, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", errors.New("value is empty")
	}
	if strings.ContainsAny(candidate, "\r\n") {
		return "", errors.New("value contains prohibited newline characters")
	}
	return candidate, nil
}

func safeString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (s *Server) generateAndStoreEstimatePDFDocument(
	ctx context.Context,
	q *gen.Queries,
	tenantID uuid.UUID,
	estimateID uuid.UUID,
	generatedBy *uuid.UUID,
	signed *signatureStamp,
) (gen.Estimate, gen.EstimateDocument, error) {
	estimate, err := q.GetEstimateByID(ctx, gen.GetEstimateByIDParams{ID: estimateID, TenantID: tenantID})
	if err != nil {
		return gen.Estimate{}, gen.EstimateDocument{}, err
	}

	var charges *gen.EstimateCharge
	chargeRow, err := q.GetEstimateChargesByEstimateID(ctx, gen.GetEstimateChargesByEstimateIDParams{TenantID: tenantID, EstimateID: estimateID})
	if err == nil {
		charges = &chargeRow
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return gen.Estimate{}, gen.EstimateDocument{}, err
	}

	pdfBytes, err := renderEstimatePDF(estimate, charges, signed)
	if err != nil {
		return gen.Estimate{}, gen.EstimateDocument{}, err
	}

	docType := string(oapi.EstimatePdf)
	fileName := fmt.Sprintf("estimate-%s.pdf", estimate.EstimateNumber)
	metadata := map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339)}
	if signed != nil {
		docType = string(oapi.SignedEstimatePdf)
		fileName = fmt.Sprintf("estimate-%s-signed.pdf", estimate.EstimateNumber)
		metadata["signature"] = map[string]any{
			"signerName":  signed.SignerName,
			"signerEmail": signed.SignerEmail,
			"signedAt":    signed.SignedAt.UTC().Format(time.RFC3339),
		}
	}

	docRow, err := createEstimateDocumentRow(ctx, q, tenantID, estimateID, docType, fileName, "application/pdf", pdfBytes, metadata, generatedBy)
	if err != nil {
		return gen.Estimate{}, gen.EstimateDocument{}, err
	}

	_ = s.Audit.Log(ctx, audit.Entry{
		TenantID:   tenantID,
		UserID:     generatedBy,
		Action:     "document.generated",
		EntityType: "estimate",
		EntityID:   &estimate.ID,
		RequestID:  middleware.RequestIDFromContext(ctx),
		Metadata: map[string]any{
			"documentId":   docRow.ID,
			"documentType": docRow.DocumentType,
			"sizeBytes":    docRow.SizeBytes,
		},
	})

	return estimate, docRow, nil
}

func createEstimateDocumentRow(
	ctx context.Context,
	q *gen.Queries,
	tenantID uuid.UUID,
	estimateID uuid.UUID,
	documentType string,
	fileName string,
	mimeType string,
	content []byte,
	metadata map[string]any,
	generatedBy *uuid.UUID,
) (gen.EstimateDocument, error) {
	encodedMeta, err := json.Marshal(metadata)
	if err != nil {
		return gen.EstimateDocument{}, err
	}

	hash := sha256.Sum256(content)
	sha := hex.EncodeToString(hash[:])
	size := int32(len(content))

	return q.CreateEstimateDocument(ctx, gen.CreateEstimateDocumentParams{
		TenantID:      tenantID,
		EstimateID:    estimateID,
		DocumentType:  documentType,
		FileName:      fileName,
		MimeType:      mimeType,
		ContentBytes:  content,
		ContentSha256: sha,
		SizeBytes:     size,
		MetadataJson:  encodedMeta,
		GeneratedBy:   generatedBy,
	})
}

func renderEstimatePDF(estimate gen.Estimate, charges *gen.EstimateCharge, signed *signatureStamp) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "Moving Estimate", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Estimate %s", estimate.EstimateNumber), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Generated %s", time.Now().UTC().Format("Jan 2, 2006 15:04 UTC")), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 7, "Customer", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, estimate.CustomerName, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Email: %s", estimate.Email), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Phone: %s", estimate.PrimaryPhone), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 7, "Move Details", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, fmt.Sprintf("Moving From: %s, %s, %s %s", estimate.OriginAddressLine1, estimate.OriginCity, estimate.OriginState, estimate.OriginPostalCode), "", "L", false)
	pdf.MultiCell(0, 5, fmt.Sprintf("Moving To: %s, %s, %s %s", estimate.DestinationAddressLine1, estimate.DestinationCity, estimate.DestinationState, estimate.DestinationPostalCode), "", "L", false)
	pdf.CellFormat(0, 6, fmt.Sprintf("Move Date: %s", estimate.MoveDate.Format("2006-01-02")), "", 1, "L", false, 0, "")
	if estimate.LocationType != nil {
		pdf.CellFormat(0, 6, fmt.Sprintf("Service Type: %s", *estimate.LocationType), "", 1, "L", false, 0, "")
	}
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Volume: %.2f CF", roundCF(estimate.TotalVolumeCf)), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 7, "Pricing Summary", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	if charges == nil {
		if estimate.EstimatedTotalCents != nil {
			pdf.CellFormat(0, 6, fmt.Sprintf("Total Estimate: %s", formatCents(*estimate.EstimatedTotalCents)), "", 1, "L", false, 0, "")
		} else {
			pdf.CellFormat(0, 6, "Charges have not been finalized yet.", "", 1, "L", false, 0, "")
		}
	} else {
		mode := strings.ReplaceAll(charges.Mode, "_", " ")
		pdf.CellFormat(0, 6, fmt.Sprintf("Mode: %s", strings.Title(mode)), "", 1, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("Subtotal: %s", formatCents(charges.ComputedSubtotalCents)), "", 1, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("Discounts: %s", formatCents(charges.ComputedDiscountsCents)), "", 1, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("Tax: %s", formatCents(charges.ComputedTaxCents)), "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(0, 7, fmt.Sprintf("Total Estimate: %s", formatCents(charges.ComputedTotalCents)), "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
	}
	pdf.Ln(2)

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 7, "Terms", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(0, 5, "This estimate is provided for planning purposes and is subject to final confirmation.\nAll services are governed by MoveOps terms and applicable federal/state regulations.", "", "L", false)

	if signed != nil {
		pdf.Ln(3)
		pdf.SetFont("Arial", "B", 12)
		pdf.CellFormat(0, 7, "Electronic Signature", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(0, 6, fmt.Sprintf("Signer: %s", signed.SignerName), "", 1, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("Email: %s", signed.SignerEmail), "", 1, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("Signed At: %s", signed.SignedAt.UTC().Format(time.RFC3339)), "", 1, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("Typed Signature: %s", signed.Signature), "", 1, "L", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mapEstimateDocument(row gen.EstimateDocument) (oapi.EstimateDocument, error) {
	mapped := oapi.EstimateDocument{
		Id:            row.ID,
		EstimateId:    row.EstimateID,
		DocumentType:  oapi.EstimateDocumentType(row.DocumentType),
		FileName:      row.FileName,
		MimeType:      row.MimeType,
		SizeBytes:     int(row.SizeBytes),
		ContentBase64: base64.StdEncoding.EncodeToString(row.ContentBytes),
		CreatedAt:     row.CreatedAt.UTC(),
	}
	if len(row.MetadataJson) > 0 {
		var metadata map[string]any
		if err := json.Unmarshal(row.MetadataJson, &metadata); err != nil {
			return oapi.EstimateDocument{}, err
		}
		mapped.Metadata = &metadata
	}
	return mapped, nil
}

func mapEstimateEmailLog(row gen.EstimateEmailLog) (oapi.EstimateEmailLog, error) {
	mapped := oapi.EstimateEmailLog{
		Id:           row.ID,
		EstimateId:   row.EstimateID,
		TemplateKey:  oapi.EstimateEmailTemplateKey(row.TemplateKey),
		To:           openapi_types.Email(row.EmailTo),
		From:         openapi_types.Email(row.EmailFrom),
		Subject:      row.Subject,
		Status:       oapi.EstimateEmailLogStatus(row.Status),
		DeliveryMode: oapi.EstimateEmailLogDeliveryMode(row.DeliveryMode),
		CreatedAt:    row.CreatedAt.UTC(),
	}
	if row.EmailCc != nil {
		cc := openapi_types.Email(*row.EmailCc)
		mapped.Cc = &cc
	}
	if row.ProviderMessageID != nil {
		mapped.ProviderMessageId = row.ProviderMessageID
	}
	if row.ErrorMessage != nil {
		mapped.ErrorMessage = row.ErrorMessage
	}
	return mapped, nil
}

func formatCents(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	dollars := float64(cents) / 100
	formatted := fmt.Sprintf("$%.2f", dollars)
	if negative {
		return "-" + formatted
	}
	return formatted
}

func clientIP(r *http.Request) *string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return &ip
			}
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		host = strings.TrimSpace(host)
		if host != "" {
			return &host
		}
	}

	remote := strings.TrimSpace(r.RemoteAddr)
	if remote == "" {
		return nil
	}
	return &remote
}
