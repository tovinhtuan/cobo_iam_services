package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// OrgDirectory validates department and membership tenant boundaries for workflow steps.
type OrgDirectory interface {
	IsActiveDepartmentInCompany(ctx context.Context, companyID, departmentID string) (bool, error)
	IsActiveMembershipInCompany(ctx context.Context, companyID, membershipID string) (bool, error)
	MemberBelongsToDepartment(ctx context.Context, membershipID, departmentID string) (bool, error)
	// ResolveDepartmentHeadMembership returns the active head membership for a department.
	// Errors use field messages department_head_not_configured / department_head_invalid.
	ResolveDepartmentHeadMembership(ctx context.Context, companyID, departmentID string) (string, error)
}

// ValidateWorkflowStepOrgRefs batch-validates department/assignee refs on a normalized snapshot.
// Unique IDs are resolved once, then checked per step index for field-level errors (no N+1).
// Supports v2 singular and v3 AssigneeMembershipIDs.
func ValidateWorkflowStepOrgRefs(ctx context.Context, org OrgDirectory, companyID string, steps []ProposalWorkflowStep) error {
	if org == nil {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "organization directory is unavailable", nil)
	}
	companyID = strings.TrimSpace(companyID)
	deptOK := map[string]bool{}
	memberOK := map[string]bool{}

	collectMember := func(m string) error {
		m = strings.TrimSpace(m)
		if m == "" {
			return nil
		}
		if _, seen := memberOK[m]; seen {
			return nil
		}
		ok, err := org.IsActiveMembershipInCompany(ctx, companyID, m)
		if err != nil {
			return mapRepositoryError(fmt.Errorf("validate membership: %w", err))
		}
		memberOK[m] = ok
		return nil
	}

	for _, s := range steps {
		if d := strings.TrimSpace(s.DepartmentID); d != "" {
			if _, seen := deptOK[d]; !seen {
				ok, err := org.IsActiveDepartmentInCompany(ctx, companyID, d)
				if err != nil {
					return mapRepositoryError(fmt.Errorf("validate department: %w", err))
				}
				deptOK[d] = ok
			}
		}
		if err := collectMember(s.AssigneeMembershipID); err != nil {
			return err
		}
		for _, m := range s.AssigneeMembershipIDs {
			if err := collectMember(m); err != nil {
				return err
			}
		}
	}

	belongCache := map[string]bool{}
	checkBelong := func(assignee, dept, field string) error {
		assignee = strings.TrimSpace(assignee)
		dept = strings.TrimSpace(dept)
		if assignee == "" {
			return nil
		}
		if dept == "" {
			return newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, field, field+" requires department_id")
		}
		if !memberOK[assignee] {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, field, field+" is invalid, inactive, or not in company")
		}
		key := assignee + "\x00" + dept
		ok, cached := belongCache[key]
		if !cached {
			var err error
			ok, err = org.MemberBelongsToDepartment(ctx, assignee, dept)
			if err != nil {
				return mapRepositoryError(fmt.Errorf("validate membership department: %w", err))
			}
			belongCache[key] = ok
		}
		if !ok {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, field, field+" does not belong to department_id")
		}
		return nil
	}

	for i, s := range steps {
		dept := strings.TrimSpace(s.DepartmentID)
		if dept != "" && !deptOK[dept] {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].department_id", i), "department_id is invalid, inactive, not a department, or not in company")
		}
		if err := checkBelong(s.AssigneeMembershipID, dept, fmt.Sprintf("workflow_steps[%d].assignee_membership_id", i)); err != nil {
			return err
		}
		for j, m := range s.AssigneeMembershipIDs {
			field := fmt.Sprintf("workflow_steps[%d].assignee_membership_ids[%d]", i, j)
			if err := checkBelong(m, dept, field); err != nil {
				return err
			}
		}
	}
	return nil
}
