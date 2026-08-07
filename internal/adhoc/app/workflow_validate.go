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
}

// ValidateWorkflowStepOrgRefs batch-validates department/assignee refs on a normalized snapshot.
// Unique IDs are resolved once, then checked per step index for field-level errors (no N+1).
func ValidateWorkflowStepOrgRefs(ctx context.Context, org OrgDirectory, companyID string, steps []ProposalWorkflowStep) error {
	if org == nil {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "organization directory is unavailable", nil)
	}
	companyID = strings.TrimSpace(companyID)
	deptOK := map[string]bool{}
	memberOK := map[string]bool{}
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
		if m := strings.TrimSpace(s.AssigneeMembershipID); m != "" {
			if _, seen := memberOK[m]; !seen {
				ok, err := org.IsActiveMembershipInCompany(ctx, companyID, m)
				if err != nil {
					return mapRepositoryError(fmt.Errorf("validate membership: %w", err))
				}
				memberOK[m] = ok
			}
		}
	}

	belongCache := map[string]bool{}
	for i, s := range steps {
		dept := strings.TrimSpace(s.DepartmentID)
		assignee := strings.TrimSpace(s.AssigneeMembershipID)
		if dept != "" && !deptOK[dept] {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].department_id", i), "department_id is invalid, inactive, not a department, or not in company")
		}
		if assignee != "" && !memberOK[assignee] {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_id", i), "assignee_membership_id is invalid, inactive, or not in company")
		}
		if assignee == "" {
			continue
		}
		if dept == "" {
			return newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_id", i), "assignee_membership_id requires department_id")
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
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_id", i), "assignee_membership_id does not belong to department_id")
		}
	}
	return nil
}
