package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *adminService) ListDepartments(ctx context.Context, req ListDepartmentsRequest) ([]DepartmentView, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.ListCompanyDepartments(ctx, req.Subject.CompanyID)
}

func (s *adminService) CreateDepartment(ctx context.Context, req CreateDepartmentRequest) (*DepartmentView, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil)
	}
	if req.HeadMembershipID != nil {
		if err := s.requireMembershipInCompany(ctx, *req.HeadMembershipID, req.Subject.CompanyID); err != nil {
			return nil, err
		}
	}
	deptID := s.idg.NewUUID()
	deptCode := s.idg.NewUUID() // opaque, unique-enough; not exposed in API
	return s.repo.CreateDepartmentRow(ctx, req.Subject.CompanyID, deptID, deptCode, name, req.HeadMembershipID, req.SortOrder)
}

func (s *adminService) UpdateDepartment(ctx context.Context, req UpdateDepartmentRequest) (*DepartmentView, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.DepartmentID = strings.TrimSpace(req.DepartmentID)
	if req.DepartmentID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department_id is required", nil)
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name cannot be empty", nil)
		}
		req.Name = &trimmed
	}
	var clearHead bool
	if req.HeadMembershipID != nil {
		if *req.HeadMembershipID == "" {
			clearHead = true
		} else {
			if err := s.requireMembershipInCompany(ctx, *req.HeadMembershipID, req.Subject.CompanyID); err != nil {
				return nil, err
			}
		}
	}
	return s.repo.PatchDepartmentRow(ctx, req.Subject.CompanyID, req.DepartmentID, req.Name, req.HeadMembershipID, clearHead, req.SortOrder, req.Status)
}

func (s *adminService) DeleteDepartment(ctx context.Context, req DeleteDepartmentRequest) error {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	req.DepartmentID = strings.TrimSpace(req.DepartmentID)
	if req.DepartmentID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department_id is required", nil)
	}
	n, err := s.repo.CountDepartmentMembers(ctx, req.DepartmentID)
	if err != nil {
		return err
	}
	if n > 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "DEPARTMENT_HAS_MEMBERS", nil)
	}
	return s.repo.SoftDeleteDepartment(ctx, req.DepartmentID, req.Subject.CompanyID)
}

func (s *adminService) AddDeptMember(ctx context.Context, req AddDeptMemberRequest) error {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	if err := s.requireMembershipInCompany(ctx, req.MembershipID, req.Subject.CompanyID); err != nil {
		return err
	}
	// AddDepartment validates department belongs to membership's company.
	return s.repo.AddDepartment(ctx, req.MembershipID, req.DepartmentID)
}

func (s *adminService) RemoveDeptMember(ctx context.Context, req RemoveDeptMemberRequest) error {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	return s.repo.RemoveDepartment(ctx, req.MembershipID, req.DepartmentID)
}

// requireRbacManage checks that the subject has the rbac.manage permission.
func (s *adminService) requireRbacManage(ctx context.Context, sub AdminSubject) error {
	ok, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "rbac.manage required", nil)
	}
	return nil
}

// requireMembershipInCompany checks that a membership exists and belongs to companyID.
func (s *adminService) requireMembershipInCompany(ctx context.Context, membershipID, companyID string) error {
	membershipID = strings.TrimSpace(membershipID)
	if membershipID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "membership_id is required", nil)
	}
	ok, err := s.repo.MembershipBelongsToCompany(ctx, membershipID, companyID)
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found in company", nil)
	}
	return nil
}
