package inmemory

import (
	"context"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

type Resolver struct {
	repo authapp.Repository
}

func NewResolver(repo authapp.Repository) *Resolver {
	return &Resolver{repo: repo}
}

func (r *Resolver) Resolve(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	permissions, err := r.repo.ListPermissionCodes(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	deps, err := r.repo.ListDepartmentScopes(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	assigns, err := r.repo.ListAssignments(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	resp, err := r.repo.ListResponsibilities(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	positions, err := r.repo.ListPositionCodes(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	orgUnitIDs, err := r.repo.ListOrgUnitIDs(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	orgSubtreeIDs, err := r.repo.ListOrgSubtreeUnitIDs(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	scopeType := "none"
	hasCompanyWide := has(permissions, "company_wide_access")
	if hasCompanyWide {
		scopeType = "company_wide"
	} else if len(deps) > 0 || len(assigns) > 0 {
		scopeType = "mixed"
	}
	return &authapp.EffectiveAccessSummary{
		CompanyID:    companyID,
		MembershipID: membershipID,
		Permissions:  permissions,
		DataScope: authapp.EffectiveDataScope{
			ScopeType:            scopeType,
			Departments:          deps,
			RecordAssignments:    assigns,
			HasCompanyWideAccess: hasCompanyWide,
			OrgUnitIDs:           orgUnitIDs,
			OrgSubtreeUnitIDs:    orgSubtreeIDs,
			PositionCodes:        positions,
		},
		Responsibilities: resp,
	}, nil
}

func has(items []string, target string) bool {
	for _, it := range items {
		if strings.EqualFold(it, target) {
			return true
		}
	}
	return false
}
