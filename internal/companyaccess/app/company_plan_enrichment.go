package app

import (
	"context"
	"net/http"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

// MapCompanyPlanReadError applies the locked STRICT enrichment policy:
// reader/database errors become HTTP 500; never silent plan:null.
// Same policy for GetOwnCompany and GET /api/v1/me/companies.
func MapCompanyPlanReadError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := perr.AsHTTPError(err); ok {
		return err
	}
	return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "company plan lookup failed", err)
}

func (s *adminService) resolveCompanyPlanAt() time.Time {
	if s.companyPlanNow != nil {
		return s.companyPlanNow().UTC()
	}
	return time.Now().UTC()
}

// resolveOwnCompanyPlanDTO loads the commercial plan DTO for response enrichment.
// Callers that mutate company state MUST invoke this BEFORE the mutation so a
// STRICT reader failure cannot turn a committed update into HTTP 500.
func (s *adminService) resolveOwnCompanyPlanDTO(ctx context.Context, companyID string) (*companyplan.PlanDTO, error) {
	if s.companyPlan == nil {
		return nil, nil
	}
	plan, err := s.companyPlan.GetEffectivePlan(ctx, companyID, s.resolveCompanyPlanAt())
	if err != nil {
		return nil, MapCompanyPlanReadError(err)
	}
	return companyplan.ToPlanDTO(plan), nil
}

func (s *adminService) attachOwnCompanyPlan(ctx context.Context, companyID string, detail *PlatformCompanyDetail) error {
	if detail == nil {
		return nil
	}
	dto, err := s.resolveOwnCompanyPlanDTO(ctx, companyID)
	if err != nil {
		return err
	}
	detail.Plan = dto
	return nil
}
