package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	chargesCalculationVersion = "v1"
	defaultCfLbsRatio         = 7.0
	maxChargeLineItems        = 25
)

type normalizedChargesInput struct {
	Mode                          string
	CfLbsRatio                    float64
	FuelSurchargePct              float64
	LdRatePerCf                   float64
	LdFixedBaseAmountCents        *int64
	LocalTrucks                   int32
	LocalWorkers                  int32
	LocalLaborHours               float64
	LocalLaborRateCents           int64
	LocalTravelHours              float64
	LocalTravelRateCents          int64
	OtherLineItems                []oapi.EstimateChargesLineItem
	DiscountCouponPct             float64
	DiscountCouponAmountCents     int64
	DiscountSeniorPct             float64
	DiscountSeniorAmountCents     int64
	PackingPackers                int32
	PackingHours                  float64
	PackingRateCents              int64
	LiabilityType                 string
	LiabilityValuationChargeCents int64
	TaxRatePct                    float64
	DepositRequiredCents          *int64
	AmountPaidCents               int64
}

type chargesComputation struct {
	TotalCf            float64
	TotalLbs           float64
	BaseCents          int64
	FuelSurchargeCents int64
	OtherItemsCents    int64
	PackingCents       int64
	LiabilityCents     int64
	SubtotalCents      int64
	DiscountsCents     int64
	TaxCents           int64
	TotalCents         int64
}

func (s *Server) GetEstimatesEstimateIdCharges(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, _, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	targetEstimateID := uuid.UUID(estimateID)
	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{
		ID:       targetEstimateID,
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

	row, err := s.Q.GetEstimateChargesByEstimateID(r.Context(), gen.GetEstimateChargesByEstimateIDParams{
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
	})
	if err == nil {
		charges, mapErr := mapEstimateCharges(row)
		if mapErr != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load charges", nil)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, oapi.EstimateChargesResponse{
			Charges:   charges,
			RequestId: middleware.RequestIDFromContext(r.Context()),
		})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load charges", nil)
		return
	}

	defaultInput := s.defaultChargesInputForEstimate(r.Context(), tenantID, estimate)
	computed := calculateCharges(estimate.TotalVolumeCf, defaultInput)

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateChargesResponse{
		Charges:   mapDefaultEstimateCharges(estimate.ID, defaultInput, computed, estimate.UpdatedAt.UTC()),
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) PutEstimatesEstimateIdCharges(w http.ResponseWriter, r *http.Request, estimateID openapi_types.UUID) {
	_, tenantID, userID, ok := requireActorIDs(w, r)
	if !ok {
		return
	}

	var req oapi.ReplaceEstimateChargesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", nil)
		return
	}

	targetEstimateID := uuid.UUID(estimateID)
	estimate, err := s.Q.GetEstimateByID(r.Context(), gen.GetEstimateByIDParams{
		ID:       targetEstimateID,
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

	normalized, err := normalizeChargesInput(req)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	computed := calculateCharges(estimate.TotalVolumeCf, normalized)

	existing, err := s.Q.GetEstimateChargesByEstimateID(r.Context(), gen.GetEstimateChargesByEstimateIDParams{
		TenantID:   tenantID,
		EstimateID: targetEstimateID,
	})
	var previousMode *string
	if err == nil {
		mode := existing.Mode
		previousMode = &mode
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load existing charges", nil)
		return
	}

	lineItemsJSON, err := json.Marshal(normalized.OtherLineItems)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to encode line items", nil)
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.Q.WithTx(tx)

	row, err := qtx.UpsertEstimateCharges(r.Context(), gen.UpsertEstimateChargesParams{
		TenantID:                      tenantID,
		EstimateID:                    targetEstimateID,
		Mode:                          normalized.Mode,
		CalculationVersion:            chargesCalculationVersion,
		CfLbsRatio:                    normalized.CfLbsRatio,
		FuelSurchargePct:              normalized.FuelSurchargePct,
		LdRatePerCf:                   normalized.LdRatePerCf,
		LdFixedBaseAmountCents:        normalized.LdFixedBaseAmountCents,
		LocalTrucks:                   normalized.LocalTrucks,
		LocalWorkers:                  normalized.LocalWorkers,
		LocalLaborHours:               normalized.LocalLaborHours,
		LocalLaborRateCents:           normalized.LocalLaborRateCents,
		LocalTravelHours:              normalized.LocalTravelHours,
		LocalTravelRateCents:          normalized.LocalTravelRateCents,
		OtherLineItemsJson:            lineItemsJSON,
		DiscountCouponPct:             normalized.DiscountCouponPct,
		DiscountCouponAmountCents:     normalized.DiscountCouponAmountCents,
		DiscountSeniorPct:             normalized.DiscountSeniorPct,
		DiscountSeniorAmountCents:     normalized.DiscountSeniorAmountCents,
		PackingPackers:                normalized.PackingPackers,
		PackingHours:                  normalized.PackingHours,
		PackingRateCents:              normalized.PackingRateCents,
		LiabilityType:                 normalized.LiabilityType,
		LiabilityValuationChargeCents: normalized.LiabilityValuationChargeCents,
		TaxRatePct:                    normalized.TaxRatePct,
		DepositRequiredCents:          normalized.DepositRequiredCents,
		AmountPaidCents:               normalized.AmountPaidCents,
		ComputedTotalCf:               computed.TotalCf,
		ComputedTotalLbs:              computed.TotalLbs,
		ComputedBaseCents:             computed.BaseCents,
		ComputedFuelSurchargeCents:    computed.FuelSurchargeCents,
		ComputedOtherItemsCents:       computed.OtherItemsCents,
		ComputedPackingCents:          computed.PackingCents,
		ComputedLiabilityCents:        computed.LiabilityCents,
		ComputedSubtotalCents:         computed.SubtotalCents,
		ComputedDiscountsCents:        computed.DiscountsCents,
		ComputedTaxCents:              computed.TaxCents,
		ComputedTotalCents:            computed.TotalCents,
		CreatedBy:                     &userID,
		UpdatedBy:                     &userID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to persist charges", nil)
		return
	}

	totalCents := computed.TotalCents
	locationType := modeToLocationType(normalized.Mode)
	affected, err := qtx.UpdateEstimatePricingSummary(r.Context(), gen.UpdateEstimatePricingSummaryParams{
		EstimatedTotalCents: &totalCents,
		DepositCents:        normalized.DepositRequiredCents,
		LocationType:        &locationType,
		UpdatedBy:           &userID,
		EstimateID:          targetEstimateID,
		TenantID:            tenantID,
	})
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update estimate totals", nil)
		return
	}
	if affected == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "estimate_not_found", "Estimate was not found", nil)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit charges update", nil)
		return
	}

	if previousMode != nil && *previousMode != normalized.Mode {
		_ = s.Audit.Log(r.Context(), audit.Entry{
			TenantID:   tenantID,
			UserID:     &userID,
			Action:     "charges.mode_switched",
			EntityType: "estimate",
			EntityID:   &targetEstimateID,
			RequestID:  middleware.RequestIDFromContext(r.Context()),
			Metadata: map[string]any{
				"from": *previousMode,
				"to":   normalized.Mode,
			},
		})
	}

	_ = s.Audit.Log(r.Context(), audit.Entry{
		TenantID:   tenantID,
		UserID:     &userID,
		Action:     "charges.updated",
		EntityType: "estimate",
		EntityID:   &targetEstimateID,
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Metadata: map[string]any{
			"mode":                normalized.Mode,
			"calculationVersion":  chargesCalculationVersion,
			"totalCf":             computed.TotalCf,
			"totalCents":          computed.TotalCents,
			"discountsTotalCents": computed.DiscountsCents,
		},
	})
	s.trackAnalyticsEvent(r.Context(), tenantID, &userID, &targetEstimateID, "estimate.charges_updated", map[string]any{
		"mode":       normalized.Mode,
		"total_cents": computed.TotalCents,
		"total_cf":   computed.TotalCf,
	})

	charges, mapErr := mapEstimateCharges(row)
	if mapErr != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Failed to build charges response", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, oapi.EstimateChargesResponse{
		Charges:   charges,
		RequestId: middleware.RequestIDFromContext(r.Context()),
	})
}

func modeToLocationType(mode string) string {
	if mode == string(oapi.EstimateChargesModeLongDistance) {
		return "Long Distance"
	}
	return "Local"
}

func (s *Server) defaultChargesInputForEstimate(ctx context.Context, tenantID uuid.UUID, estimate gen.Estimate) normalizedChargesInput {
	mode := string(oapi.EstimateChargesModeLocal)
	if estimate.LocationType != nil && strings.Contains(strings.ToLower(*estimate.LocationType), "long") {
		mode = string(oapi.EstimateChargesModeLongDistance)
	}
	out := normalizedChargesInput{
		Mode:                 mode,
		CfLbsRatio:           defaultCfLbsRatio,
		OtherLineItems:       []oapi.EstimateChargesLineItem{},
		LiabilityType:        string(oapi.Release),
		DepositRequiredCents: nil,
	}

	pricingDefaults, err := s.getTenantPricingDefaults(ctx, tenantID)
	if err != nil {
		return out
	}

	if pricingDefaults.LongDistance != nil {
		if pricingDefaults.LongDistance.RatePerCf != nil {
			out.LdRatePerCf = *pricingDefaults.LongDistance.RatePerCf
		}
		if pricingDefaults.LongDistance.FuelSurchargePct != nil {
			out.FuelSurchargePct = *pricingDefaults.LongDistance.FuelSurchargePct
		}
		if pricingDefaults.LongDistance.TaxRatePct != nil {
			out.TaxRatePct = *pricingDefaults.LongDistance.TaxRatePct
		}
	}
	if pricingDefaults.Local != nil {
		if pricingDefaults.Local.LaborRateCents != nil {
			out.LocalLaborRateCents = *pricingDefaults.Local.LaborRateCents
		}
		if pricingDefaults.Local.TravelRateCents != nil {
			out.LocalTravelRateCents = *pricingDefaults.Local.TravelRateCents
		}
		if pricingDefaults.Local.FuelSurchargePct != nil && mode == string(oapi.EstimateChargesModeLocal) {
			out.FuelSurchargePct = *pricingDefaults.Local.FuelSurchargePct
		}
		if pricingDefaults.Local.TaxRatePct != nil && mode == string(oapi.EstimateChargesModeLocal) {
			out.TaxRatePct = *pricingDefaults.Local.TaxRatePct
		}
	}
	if pricingDefaults.Discounts != nil {
		if pricingDefaults.Discounts.CouponPct != nil {
			out.DiscountCouponPct = *pricingDefaults.Discounts.CouponPct
		}
		if pricingDefaults.Discounts.SeniorPct != nil {
			out.DiscountSeniorPct = *pricingDefaults.Discounts.SeniorPct
		}
	}
	if pricingDefaults.Liability != nil {
		if pricingDefaults.Liability.Type != nil {
			out.LiabilityType = string(*pricingDefaults.Liability.Type)
		}
		if pricingDefaults.Liability.ValuationChargeCents != nil {
			out.LiabilityValuationChargeCents = *pricingDefaults.Liability.ValuationChargeCents
		}
	}

	return out
}

func normalizeChargesInput(req oapi.ReplaceEstimateChargesRequest) (normalizedChargesInput, error) {
	mode := string(req.Mode)
	if mode != string(oapi.EstimateChargesModeLocal) && mode != string(oapi.EstimateChargesModeLongDistance) {
		return normalizedChargesInput{}, errors.New("mode must be local or long_distance")
	}

	out := normalizedChargesInput{
		Mode:                          mode,
		CfLbsRatio:                    defaultCfLbsRatio,
		OtherLineItems:                []oapi.EstimateChargesLineItem{},
		LiabilityType:                 string(oapi.Release),
		DiscountCouponPct:             0,
		DiscountCouponAmountCents:     0,
		DiscountSeniorPct:             0,
		DiscountSeniorAmountCents:     0,
		FuelSurchargePct:              0,
		LdRatePerCf:                   0,
		LocalTrucks:                   0,
		LocalWorkers:                  0,
		LocalLaborHours:               0,
		LocalLaborRateCents:           0,
		LocalTravelHours:              0,
		LocalTravelRateCents:          0,
		PackingPackers:                0,
		PackingHours:                  0,
		PackingRateCents:              0,
		LiabilityValuationChargeCents: 0,
		TaxRatePct:                    0,
		AmountPaidCents:               0,
	}

	if req.CfLbsRatio != nil {
		if *req.CfLbsRatio < 0 {
			return normalizedChargesInput{}, errors.New("cfLbsRatio must be greater than or equal to 0")
		}
		out.CfLbsRatio = roundTo2(*req.CfLbsRatio)
	}
	if req.FuelSurchargePct != nil {
		if *req.FuelSurchargePct < 0 || *req.FuelSurchargePct > 100 {
			return normalizedChargesInput{}, errors.New("fuelSurchargePct must be between 0 and 100")
		}
		out.FuelSurchargePct = roundTo2(*req.FuelSurchargePct)
	}
	if req.LongDistance != nil {
		if req.LongDistance.RatePerCf != nil {
			if *req.LongDistance.RatePerCf < 0 {
				return normalizedChargesInput{}, errors.New("longDistance.ratePerCf must be greater than or equal to 0")
			}
			out.LdRatePerCf = roundTo2(*req.LongDistance.RatePerCf)
		}
		if req.LongDistance.FixedBaseAmountCents != nil {
			if *req.LongDistance.FixedBaseAmountCents < 0 {
				return normalizedChargesInput{}, errors.New("longDistance.fixedBaseAmountCents must be greater than or equal to 0")
			}
			out.LdFixedBaseAmountCents = req.LongDistance.FixedBaseAmountCents
		}
	}
	if req.Local != nil {
		if req.Local.Trucks != nil {
			if *req.Local.Trucks < 0 {
				return normalizedChargesInput{}, errors.New("local.trucks must be greater than or equal to 0")
			}
			out.LocalTrucks = int32(*req.Local.Trucks)
		}
		if req.Local.Workers != nil {
			if *req.Local.Workers < 0 {
				return normalizedChargesInput{}, errors.New("local.workers must be greater than or equal to 0")
			}
			out.LocalWorkers = int32(*req.Local.Workers)
		}
		if req.Local.LaborHours != nil {
			if *req.Local.LaborHours < 0 {
				return normalizedChargesInput{}, errors.New("local.laborHours must be greater than or equal to 0")
			}
			out.LocalLaborHours = roundTo2(*req.Local.LaborHours)
		}
		if req.Local.LaborRateCents != nil {
			if *req.Local.LaborRateCents < 0 {
				return normalizedChargesInput{}, errors.New("local.laborRateCents must be greater than or equal to 0")
			}
			out.LocalLaborRateCents = *req.Local.LaborRateCents
		}
		if req.Local.TravelHours != nil {
			if *req.Local.TravelHours < 0 {
				return normalizedChargesInput{}, errors.New("local.travelHours must be greater than or equal to 0")
			}
			out.LocalTravelHours = roundTo2(*req.Local.TravelHours)
		}
		if req.Local.TravelRateCents != nil {
			if *req.Local.TravelRateCents < 0 {
				return normalizedChargesInput{}, errors.New("local.travelRateCents must be greater than or equal to 0")
			}
			out.LocalTravelRateCents = *req.Local.TravelRateCents
		}
	}

	if req.OtherLineItems != nil {
		if len(*req.OtherLineItems) > maxChargeLineItems {
			return normalizedChargesInput{}, fmt.Errorf("otherLineItems must be %d or fewer", maxChargeLineItems)
		}
		out.OtherLineItems = make([]oapi.EstimateChargesLineItem, 0, len(*req.OtherLineItems))
		for _, line := range *req.OtherLineItems {
			label := strings.TrimSpace(line.Label)
			if label == "" {
				return normalizedChargesInput{}, errors.New("otherLineItems.label is required")
			}
			out.OtherLineItems = append(out.OtherLineItems, oapi.EstimateChargesLineItem{
				Label:       label,
				AmountCents: line.AmountCents,
			})
		}
	}

	if req.Discounts != nil {
		if req.Discounts.CouponPct != nil {
			if *req.Discounts.CouponPct < 0 || *req.Discounts.CouponPct > 100 {
				return normalizedChargesInput{}, errors.New("discounts.couponPct must be between 0 and 100")
			}
			out.DiscountCouponPct = roundTo2(*req.Discounts.CouponPct)
		}
		if req.Discounts.CouponAmountCents != nil {
			if *req.Discounts.CouponAmountCents < 0 {
				return normalizedChargesInput{}, errors.New("discounts.couponAmountCents must be greater than or equal to 0")
			}
			out.DiscountCouponAmountCents = *req.Discounts.CouponAmountCents
		}
		if req.Discounts.SeniorPct != nil {
			if *req.Discounts.SeniorPct < 0 || *req.Discounts.SeniorPct > 100 {
				return normalizedChargesInput{}, errors.New("discounts.seniorPct must be between 0 and 100")
			}
			out.DiscountSeniorPct = roundTo2(*req.Discounts.SeniorPct)
		}
		if req.Discounts.SeniorAmountCents != nil {
			if *req.Discounts.SeniorAmountCents < 0 {
				return normalizedChargesInput{}, errors.New("discounts.seniorAmountCents must be greater than or equal to 0")
			}
			out.DiscountSeniorAmountCents = *req.Discounts.SeniorAmountCents
		}
	}

	if req.Packing != nil {
		if req.Packing.Packers != nil {
			if *req.Packing.Packers < 0 {
				return normalizedChargesInput{}, errors.New("packing.packers must be greater than or equal to 0")
			}
			out.PackingPackers = int32(*req.Packing.Packers)
		}
		if req.Packing.Hours != nil {
			if *req.Packing.Hours < 0 {
				return normalizedChargesInput{}, errors.New("packing.hours must be greater than or equal to 0")
			}
			out.PackingHours = roundTo2(*req.Packing.Hours)
		}
		if req.Packing.RateCents != nil {
			if *req.Packing.RateCents < 0 {
				return normalizedChargesInput{}, errors.New("packing.rateCents must be greater than or equal to 0")
			}
			out.PackingRateCents = *req.Packing.RateCents
		}
	}

	if req.Liability != nil {
		if req.Liability.Type != nil {
			switch *req.Liability.Type {
			case oapi.FullValue, oapi.Release:
				out.LiabilityType = string(*req.Liability.Type)
			default:
				return normalizedChargesInput{}, errors.New("liability.type must be release or full_value")
			}
		}
		if req.Liability.ValuationChargeCents != nil {
			if *req.Liability.ValuationChargeCents < 0 {
				return normalizedChargesInput{}, errors.New("liability.valuationChargeCents must be greater than or equal to 0")
			}
			out.LiabilityValuationChargeCents = *req.Liability.ValuationChargeCents
		}
	}

	if req.TaxRatePct != nil {
		if *req.TaxRatePct < 0 || *req.TaxRatePct > 100 {
			return normalizedChargesInput{}, errors.New("taxRatePct must be between 0 and 100")
		}
		out.TaxRatePct = roundTo2(*req.TaxRatePct)
	}

	if req.DepositRequiredCents != nil {
		if *req.DepositRequiredCents < 0 {
			return normalizedChargesInput{}, errors.New("depositRequiredCents must be greater than or equal to 0")
		}
		out.DepositRequiredCents = req.DepositRequiredCents
	}

	if req.AmountPaidCents != nil {
		if *req.AmountPaidCents < 0 {
			return normalizedChargesInput{}, errors.New("amountPaidCents must be greater than or equal to 0")
		}
		out.AmountPaidCents = *req.AmountPaidCents
	}

	return out, nil
}

func calculateCharges(totalCf float64, in normalizedChargesInput) chargesComputation {
	totalCf = roundCF(math.Max(0, totalCf))
	cfLbsRatio := in.CfLbsRatio
	if cfLbsRatio < 0 {
		cfLbsRatio = defaultCfLbsRatio
	}
	totalLbs := roundTo2(totalCf * cfLbsRatio)

	baseCents := int64(0)
	if in.Mode == string(oapi.EstimateChargesModeLongDistance) {
		if in.LdFixedBaseAmountCents != nil {
			baseCents = *in.LdFixedBaseAmountCents
		} else {
			baseCents = int64(math.Round(totalCf * in.LdRatePerCf * 100))
		}
	} else {
		workersMultiplier := float64(in.LocalWorkers)
		if workersMultiplier <= 0 {
			workersMultiplier = 1
		}
		laborCents := in.LocalLaborHours * workersMultiplier * float64(in.LocalLaborRateCents)
		travelCents := in.LocalTravelHours * float64(in.LocalTravelRateCents)
		baseCents = int64(math.Round(laborCents + travelCents))
	}

	fuelSurchargeCents := int64(math.Round(float64(baseCents) * in.FuelSurchargePct / 100))

	otherItemsCents := int64(0)
	for _, line := range in.OtherLineItems {
		otherItemsCents += line.AmountCents
	}

	packingCents := int64(math.Round(float64(in.PackingPackers) * in.PackingHours * float64(in.PackingRateCents)))
	liabilityCents := in.LiabilityValuationChargeCents

	subtotalCents := baseCents + fuelSurchargeCents + otherItemsCents + packingCents + liabilityCents
	discountBase := math.Max(0, float64(subtotalCents))
	discountsCents := int64(math.Round(discountBase*in.DiscountCouponPct/100)) +
		int64(math.Round(discountBase*in.DiscountSeniorPct/100)) +
		in.DiscountCouponAmountCents +
		in.DiscountSeniorAmountCents
	if discountsCents < 0 {
		discountsCents = 0
	}
	if discountsCents > int64(discountBase) {
		discountsCents = int64(discountBase)
	}

	taxableCents := subtotalCents - discountsCents
	if taxableCents < 0 {
		taxableCents = 0
	}
	taxCents := int64(math.Round(float64(taxableCents) * in.TaxRatePct / 100))

	totalCents := subtotalCents - discountsCents + taxCents
	if totalCents < 0 {
		totalCents = 0
	}

	return chargesComputation{
		TotalCf:            totalCf,
		TotalLbs:           totalLbs,
		BaseCents:          baseCents,
		FuelSurchargeCents: fuelSurchargeCents,
		OtherItemsCents:    otherItemsCents,
		PackingCents:       packingCents,
		LiabilityCents:     liabilityCents,
		SubtotalCents:      subtotalCents,
		DiscountsCents:     discountsCents,
		TaxCents:           taxCents,
		TotalCents:         totalCents,
	}
}

func mapDefaultEstimateCharges(estimateID uuid.UUID, in normalizedChargesInput, calc chargesComputation, updatedAt time.Time) oapi.EstimateCharges {
	mode := oapi.EstimateChargesMode(in.Mode)
	liability := oapi.EstimateLiabilityType(in.LiabilityType)

	return oapi.EstimateCharges{
		EstimateId:         estimateID,
		Mode:               mode,
		CalculationVersion: chargesCalculationVersion,
		CfLbsRatio:         roundTo2(in.CfLbsRatio),
		FuelSurchargePct:   roundTo2(in.FuelSurchargePct),
		LongDistance: oapi.EstimateChargesLongDistanceInput{
			RatePerCf:            ptrFloat64(roundTo2(in.LdRatePerCf)),
			FixedBaseAmountCents: in.LdFixedBaseAmountCents,
		},
		Local: oapi.EstimateChargesLocalInput{
			Trucks:          ptrInt(int(in.LocalTrucks)),
			Workers:         ptrInt(int(in.LocalWorkers)),
			LaborHours:      ptrFloat64(roundTo2(in.LocalLaborHours)),
			LaborRateCents:  ptrInt64(in.LocalLaborRateCents),
			TravelHours:     ptrFloat64(roundTo2(in.LocalTravelHours)),
			TravelRateCents: ptrInt64(in.LocalTravelRateCents),
		},
		OtherLineItems: in.OtherLineItems,
		Discounts: oapi.EstimateChargesDiscountInput{
			CouponPct:         ptrFloat64(roundTo2(in.DiscountCouponPct)),
			CouponAmountCents: ptrInt64(in.DiscountCouponAmountCents),
			SeniorPct:         ptrFloat64(roundTo2(in.DiscountSeniorPct)),
			SeniorAmountCents: ptrInt64(in.DiscountSeniorAmountCents),
		},
		Packing: oapi.EstimateChargesPackingInput{
			Packers:   ptrInt(int(in.PackingPackers)),
			Hours:     ptrFloat64(roundTo2(in.PackingHours)),
			RateCents: ptrInt64(in.PackingRateCents),
		},
		Liability: oapi.EstimateChargesLiabilityInput{
			Type:                 &liability,
			ValuationChargeCents: ptrInt64(in.LiabilityValuationChargeCents),
		},
		TaxRatePct:           roundTo2(in.TaxRatePct),
		DepositRequiredCents: in.DepositRequiredCents,
		AmountPaidCents:      in.AmountPaidCents,
		Computed: oapi.EstimateChargesComputed{
			BaseCents:            calc.BaseCents,
			FuelSurchargeCents:   calc.FuelSurchargeCents,
			OtherItemsTotalCents: calc.OtherItemsCents,
			PackingTotalCents:    calc.PackingCents,
			LiabilityTotalCents:  calc.LiabilityCents,
			SubtotalCents:        calc.SubtotalCents,
			DiscountsTotalCents:  calc.DiscountsCents,
			TaxTotalCents:        calc.TaxCents,
			TotalCents:           calc.TotalCents,
			TotalCf:              calc.TotalCf,
			TotalLbs:             calc.TotalLbs,
		},
		UpdatedAt: updatedAt.UTC(),
	}
}

func mapEstimateCharges(row gen.EstimateCharge) (oapi.EstimateCharges, error) {
	lineItems := []oapi.EstimateChargesLineItem{}
	if len(row.OtherLineItemsJson) > 0 {
		if err := json.Unmarshal(row.OtherLineItemsJson, &lineItems); err != nil {
			return oapi.EstimateCharges{}, err
		}
	}
	input := normalizedChargesInput{
		Mode:                          row.Mode,
		CfLbsRatio:                    row.CfLbsRatio,
		FuelSurchargePct:              row.FuelSurchargePct,
		LdRatePerCf:                   row.LdRatePerCf,
		LdFixedBaseAmountCents:        row.LdFixedBaseAmountCents,
		LocalTrucks:                   row.LocalTrucks,
		LocalWorkers:                  row.LocalWorkers,
		LocalLaborHours:               row.LocalLaborHours,
		LocalLaborRateCents:           row.LocalLaborRateCents,
		LocalTravelHours:              row.LocalTravelHours,
		LocalTravelRateCents:          row.LocalTravelRateCents,
		OtherLineItems:                lineItems,
		DiscountCouponPct:             row.DiscountCouponPct,
		DiscountCouponAmountCents:     row.DiscountCouponAmountCents,
		DiscountSeniorPct:             row.DiscountSeniorPct,
		DiscountSeniorAmountCents:     row.DiscountSeniorAmountCents,
		PackingPackers:                row.PackingPackers,
		PackingHours:                  row.PackingHours,
		PackingRateCents:              row.PackingRateCents,
		LiabilityType:                 row.LiabilityType,
		LiabilityValuationChargeCents: row.LiabilityValuationChargeCents,
		TaxRatePct:                    row.TaxRatePct,
		DepositRequiredCents:          row.DepositRequiredCents,
		AmountPaidCents:               row.AmountPaidCents,
	}
	computed := chargesComputation{
		TotalCf:            roundCF(row.ComputedTotalCf),
		TotalLbs:           roundTo2(row.ComputedTotalLbs),
		BaseCents:          row.ComputedBaseCents,
		FuelSurchargeCents: row.ComputedFuelSurchargeCents,
		OtherItemsCents:    row.ComputedOtherItemsCents,
		PackingCents:       row.ComputedPackingCents,
		LiabilityCents:     row.ComputedLiabilityCents,
		SubtotalCents:      row.ComputedSubtotalCents,
		DiscountsCents:     row.ComputedDiscountsCents,
		TaxCents:           row.ComputedTaxCents,
		TotalCents:         row.ComputedTotalCents,
	}
	out := mapDefaultEstimateCharges(row.EstimateID, input, computed, row.UpdatedAt.UTC())
	out.CalculationVersion = row.CalculationVersion
	return out, nil
}

func roundTo2(value float64) float64 {
	return math.Round(value*100) / 100
}

func ptrFloat64(value float64) *float64 {
	v := value
	return &v
}

func ptrInt64(value int64) *int64 {
	v := value
	return &v
}

func ptrInt(value int) *int {
	v := value
	return &v
}
