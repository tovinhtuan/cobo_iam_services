package app

import (
	"context"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

// DeadlineAlertAccessScope is the server-side data boundary for list deadline alerts.
type DeadlineAlertAccessScope struct {
	CompanyID    string
	MembershipID string
	CanViewAll   bool
	// DepartmentIDs from department_memberships (e.g. d_legal).
	DepartmentIDs []string
	// OrgUnitIDs and OrgSubtreeUnitIDs for matching workflow snapshot step departments.
	OrgUnitIDs        []string
	OrgSubtreeUnitIDs []string
	// AssignedRecordIDs: direct disclosure_record assignments for this membership.
	AssignedRecordIDs map[string]struct{}
}

// ResolveDeadlineAlertAccessScope builds scope from effective access (JWT subject is source of truth).
func ResolveDeadlineAlertAccessScope(eff *authapp.EffectiveAccessSummary) DeadlineAlertAccessScope {
	scope := DeadlineAlertAccessScope{
		CompanyID:    eff.CompanyID,
		MembershipID: eff.MembershipID,
		CanViewAll:   canViewAllDeadlineAlerts(eff),
	}
	if scope.CanViewAll {
		return scope
	}
	for _, d := range eff.DataScope.Departments {
		if id := strings.TrimSpace(d.DepartmentID); id != "" {
			scope.DepartmentIDs = append(scope.DepartmentIDs, id)
		}
	}
	scope.OrgUnitIDs = append(scope.OrgUnitIDs, eff.DataScope.OrgUnitIDs...)
	scope.OrgSubtreeUnitIDs = append(scope.OrgSubtreeUnitIDs, eff.DataScope.OrgSubtreeUnitIDs...)
	scope.AssignedRecordIDs = map[string]struct{}{}
	for _, a := range eff.DataScope.RecordAssignments {
		if a.ResourceType != "disclosure_record" {
			continue
		}
		if id := strings.TrimSpace(a.ResourceID); id != "" {
			scope.AssignedRecordIDs[id] = struct{}{}
		}
	}
	return scope
}

func canViewAllDeadlineAlerts(eff *authapp.EffectiveAccessSummary) bool {
	if eff == nil {
		return false
	}
	if eff.DataScope.HasCompanyWideAccess {
		return true
	}
	for _, p := range eff.Permissions {
		switch p {
		case "rbac.manage", "system.settings", "company_wide_access":
			return true
		}
	}
	return false
}

// DepartmentMatchKeys returns all identifiers usable for department / org-unit matching.
func (s DeadlineAlertAccessScope) DepartmentMatchKeys() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || v == "general" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, id := range s.DepartmentIDs {
		add(id)
	}
	for _, id := range s.OrgUnitIDs {
		add(id)
	}
	for _, id := range s.OrgSubtreeUnitIDs {
		add(id)
	}
	return out
}

// AllowsRow returns true when the alert row is inside the caller's data scope.
func (s DeadlineAlertAccessScope) AllowsRow(row AlertRow) bool {
	if s.CanViewAll {
		return true
	}
	recordID := strings.TrimSpace(row.RecordID)
	if recordID != "" {
		if _, ok := s.AssignedRecordIDs[recordID]; ok {
			return true
		}
	}
	if row.HasTaskAssignee {
		return true
	}
	keys := s.departmentKeySet()
	if len(keys) == 0 {
		return false
	}
	if dept := strings.TrimSpace(row.RecordDepartmentID); dept != "" && dept != "general" {
		if keys.matches(dept) {
			return true
		}
	}
	if dept := strings.TrimSpace(row.CurrentStepDepartment); dept != "" {
		if keys.matches(dept) {
			return true
		}
	}
	return false
}

type departmentKeySet map[string]struct{}

func (s DeadlineAlertAccessScope) departmentKeySet() departmentKeySet {
	out := departmentKeySet{}
	for _, k := range s.DepartmentMatchKeys() {
		out[strings.ToLower(k)] = struct{}{}
	}
	return out
}

func (k departmentKeySet) matches(value string) bool {
	if len(k) == 0 {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return false
	}
	if _, ok := k[v]; ok {
		return true
	}
	return false
}

// AccessScopeResolver loads effective access for a subject.
type AccessScopeResolver interface {
	Resolve(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error)
}

type authAccessScopeResolver struct {
	auth authapp.Service
}

func NewAuthAccessScopeResolver(auth authapp.Service) AccessScopeResolver {
	return &authAccessScopeResolver{auth: auth}
}

func (r *authAccessScopeResolver) Resolve(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	return r.auth.GetEffectiveAccess(ctx, membershipID, companyID)
}
