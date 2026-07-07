package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func normalizeOrgIDList(legacySingle string, multi []string) []string {
	out := normalizeFocalDepartmentIDs(multi)
	single := strings.TrimSpace(legacySingle)
	if single != "" {
		found := false
		for _, id := range out {
			if id == single {
				found = true
				break
			}
		}
		if !found {
			out = append([]string{single}, out...)
		}
	}
	return out
}

func mergeDepartmentIDsWithFocal(departmentIDs, focalDepartmentIDs []string) []string {
	merged := normalizeOrgIDList("", departmentIDs)
	for _, id := range normalizeFocalDepartmentIDs(focalDepartmentIDs) {
		found := false
		for _, existing := range merged {
			if existing == id {
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, id)
		}
	}
	return merged
}

func (s *adminService) validateOrgAssignmentsForCompany(ctx context.Context, companyID string, departmentIDs, titleIDs, focalDepartmentIDs []string) error {
	depts, err := s.repo.ListCompanyDepartments(ctx, companyID)
	if err != nil {
		return err
	}
	deptSet := make(map[string]struct{}, len(depts))
	for _, d := range depts {
		if strings.TrimSpace(d.Status) == "inactive" {
			continue
		}
		deptSet[d.DepartmentID] = struct{}{}
	}
	for _, deptID := range departmentIDs {
		if _, ok := deptSet[deptID]; !ok {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department_id not found in company: "+deptID, nil)
		}
	}
	for _, deptID := range focalDepartmentIDs {
		if _, ok := deptSet[deptID]; !ok {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "focal_department_id not found in company: "+deptID, nil)
		}
	}

	titles, err := s.repo.ListCompanyTitles(ctx, companyID)
	if err != nil {
		return err
	}
	titleSet := make(map[string]struct{}, len(titles))
	for _, t := range titles {
		if strings.TrimSpace(t.Status) == "inactive" {
			continue
		}
		titleSet[t.TitleID] = struct{}{}
	}
	for _, titleID := range titleIDs {
		if _, ok := titleSet[titleID]; !ok {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title_id not found in company: "+titleID, nil)
		}
	}
	return nil
}

func (s *adminService) assertDepartmentsInInviteScope(scope inviteScope, departmentIDs []string) error {
	if scope.Kind != inviteScopeDepartment {
		return nil
	}
	allowed := make(map[string]struct{}, len(scope.DepartmentIDs))
	for _, id := range scope.DepartmentIDs {
		allowed[id] = struct{}{}
	}
	for _, deptID := range departmentIDs {
		if _, ok := allowed[deptID]; !ok {
			return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "department_id not in invite scope", nil)
		}
	}
	return nil
}

func (s *adminService) assignInviteOrg(
	ctx context.Context,
	membershipID, companyID string,
	departmentIDs, titleIDs []string,
	isDepartmentFocal bool,
	focalDepartmentIDs []string,
	scope inviteScope,
) error {
	departmentIDs = normalizeOrgIDList("", departmentIDs)
	titleIDs = normalizeOrgIDList("", titleIDs)
	focalDepartmentIDs = normalizeFocalDepartmentIDs(focalDepartmentIDs)

	if isDepartmentFocal {
		if err := s.validateInviteOrgFields(true, focalDepartmentIDs); err != nil {
			return err
		}
		departmentIDs = mergeDepartmentIDsWithFocal(departmentIDs, focalDepartmentIDs)
	}

	if err := s.validateOrgAssignmentsForCompany(ctx, companyID, departmentIDs, titleIDs, focalDepartmentIDs); err != nil {
		return err
	}
	if err := s.assertDepartmentsInInviteScope(scope, departmentIDs); err != nil {
		return err
	}
	if err := s.assertFocalDepartmentsInInviteScope(scope, focalDepartmentIDs); err != nil {
		return err
	}

	focalSet := make(map[string]struct{}, len(focalDepartmentIDs))
	if isDepartmentFocal {
		for _, id := range focalDepartmentIDs {
			focalSet[id] = struct{}{}
		}
	}

	for _, deptID := range departmentIDs {
		_, isFocal := focalSet[deptID]
		if err := s.repo.UpsertDepartmentMembership(ctx, membershipID, deptID, isFocal); err != nil {
			return err
		}
	}
	for _, titleID := range titleIDs {
		if err := s.repo.AddTitle(ctx, membershipID, titleID); err != nil {
			return err
		}
	}
	return nil
}

func (s *adminService) UpdateMembershipOrgAssignments(ctx context.Context, req UpdateMembershipOrgRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.update", req.MembershipID); err != nil {
		return err
	}
	if err := s.authorizeScopedMembershipMutation(ctx, req.Subject, "admin.membership.update", req.MembershipID); err != nil {
		return err
	}

	member, err := s.repo.GetMembershipByID(ctx, req.MembershipID)
	if err != nil {
		return err
	}

	departmentIDs := normalizeOrgIDList("", req.DepartmentIDs)
	titleIDs := normalizeOrgIDList("", req.TitleIDs)
	focalDepartmentIDs := normalizeFocalDepartmentIDs(req.FocalDepartmentIDs)

	if len(focalDepartmentIDs) > 0 {
		departmentIDs = mergeDepartmentIDsWithFocal(departmentIDs, focalDepartmentIDs)
	}

	if err := s.validateOrgAssignmentsForCompany(ctx, member.CompanyID, departmentIDs, titleIDs, focalDepartmentIDs); err != nil {
		return err
	}

	currentDeptIDs, err := s.repo.ListActiveMembershipDepartmentIDs(ctx, req.MembershipID)
	if err != nil {
		return err
	}
	currentTitleIDs, err := s.repo.ListActiveMembershipTitleIDs(ctx, req.MembershipID)
	if err != nil {
		return err
	}

	desiredDept := make(map[string]struct{}, len(departmentIDs))
	for _, id := range departmentIDs {
		desiredDept[id] = struct{}{}
	}
	focalSet := make(map[string]struct{}, len(focalDepartmentIDs))
	for _, id := range focalDepartmentIDs {
		focalSet[id] = struct{}{}
	}

	for _, deptID := range currentDeptIDs {
		if _, keep := desiredDept[deptID]; !keep {
			if err := s.authorizeScopedDepartmentMutation(ctx, req.Subject, "admin.membership.update", deptID); err != nil {
				return err
			}
			if err := s.repo.RemoveDepartment(ctx, req.MembershipID, deptID); err != nil {
				return err
			}
		}
	}
	for _, deptID := range departmentIDs {
		if err := s.authorizeScopedDepartmentMutation(ctx, req.Subject, "admin.membership.update", deptID); err != nil {
			return err
		}
		_, isFocal := focalSet[deptID]
		if err := s.repo.UpsertDepartmentMembership(ctx, req.MembershipID, deptID, isFocal); err != nil {
			return err
		}
	}
	for _, deptID := range departmentIDs {
		_, isFocal := focalSet[deptID]
		if err := s.repo.SetDepartmentMembershipFocal(ctx, req.MembershipID, deptID, isFocal); err != nil {
			return err
		}
	}

	desiredTitle := make(map[string]struct{}, len(titleIDs))
	for _, id := range titleIDs {
		desiredTitle[id] = struct{}{}
	}
	for _, titleID := range currentTitleIDs {
		if _, keep := desiredTitle[titleID]; !keep {
			if err := s.repo.RemoveTitle(ctx, req.MembershipID, titleID); err != nil {
				return err
			}
		}
	}
	for _, titleID := range titleIDs {
		if err := s.repo.AddTitle(ctx, req.MembershipID, titleID); err != nil {
			return err
		}
	}
	return nil
}

func prepareInviteOrgFromRequest(
	departmentID string,
	departmentIDs []string,
	titleID string,
	titleIDs []string,
	isDepartmentFocal bool,
	focalDepartmentIDs []string,
) (normalizedDeptIDs, normalizedTitleIDs []string, pickDepartmentID string, focal bool, normalizedFocalIDs []string) {
	normalizedDeptIDs = normalizeOrgIDList(departmentID, departmentIDs)
	normalizedTitleIDs = normalizeOrgIDList(titleID, titleIDs)
	normalizedFocalIDs = normalizeFocalDepartmentIDs(focalDepartmentIDs)
	focal = isDepartmentFocal
	pickDepartmentID = strings.TrimSpace(departmentID)
	if pickDepartmentID == "" && len(normalizedDeptIDs) > 0 {
		pickDepartmentID = normalizedDeptIDs[0]
	}
	if focal && pickDepartmentID == "" && len(normalizedFocalIDs) > 0 {
		pickDepartmentID = normalizedFocalIDs[0]
	}
	return normalizedDeptIDs, normalizedTitleIDs, pickDepartmentID, focal, normalizedFocalIDs
}
