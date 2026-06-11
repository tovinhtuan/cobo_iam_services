package app

import (
	"context"
	"net/http"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *service) filterTypesByApplicability(
	ctx context.Context,
	companyID string,
	items []DisclosureTypeSummaryDTO,
) ([]DisclosureTypeSummaryDTO, error) {
	if companyID == "" {
		return items, nil
	}
	profile, err := s.repo.GetCompanyApplicabilityProfile(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]DisclosureTypeSummaryDTO, 0, len(items))
	for _, item := range items {
		if item.Scope == templateScopeCompany {
			out = append(out, item)
			continue
		}
		if !applicability.IsApplicable(item.ApplicabilityRules, profile, s.templateApplicabilityStrictFilter) {
			continue
		}
		if days, ok := applicability.ResolveDeadlineDays(item.ApplicabilityRules, profile); ok {
			item.ResolvedStructureDeadlineDays = &days
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *service) enforceTemplateApplicability(ctx context.Context, companyID, typeID string) error {
	if companyID == "" || typeID == "" {
		return nil
	}
	detail, err := s.repo.GetTypeDetail(ctx, companyID, typeID)
	if err != nil {
		return err
	}
	if detail.Scope == templateScopeCompany {
		return nil
	}
	profile, err := s.repo.GetCompanyApplicabilityProfile(ctx, companyID)
	if err != nil {
		return err
	}
	if !applicability.IsApplicable(detail.ApplicabilityRules, profile, s.templateApplicabilityStrictFilter) {
		return perr.NewHTTPError(http.StatusUnprocessableEntity, "TEMPLATE_NOT_APPLICABLE", "template not applicable to company profile", nil)
	}
	return nil
}

// enforceStructureDeadlineOnCreate (E11) blocks manual create when a global template
// configures deadline_by_structure but lacks the entry for the company's structure.
// Company-scoped templates and templates without structure deadlines are unchanged.
func (s *service) enforceStructureDeadlineOnCreate(ctx context.Context, companyID, typeID string) error {
	if companyID == "" || typeID == "" {
		return nil
	}
	detail, err := s.repo.GetTypeDetail(ctx, companyID, typeID)
	if err != nil {
		return err
	}
	if detail.Scope == templateScopeCompany {
		return nil
	}
	rules := detail.ApplicabilityRules
	if rules == nil || len(rules.DeadlineByStructure) == 0 {
		return nil
	}
	profile, err := s.repo.GetCompanyApplicabilityProfile(ctx, companyID)
	if err != nil {
		return err
	}
	if _, ok := applicability.ResolveDeadlineDays(rules, profile); ok {
		return nil
	}
	return perr.NewHTTPError(
		http.StatusUnprocessableEntity,
		"DEADLINE_RULE_UNRESOLVED",
		"Không xác định được thời hạn nộp theo cơ cấu tổ chức của doanh nghiệp. Liên hệ quản trị nền tảng để cập nhật mẫu công bố.",
		nil,
	)
}
