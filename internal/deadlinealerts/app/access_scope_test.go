package app

import (
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

func TestResolveDeadlineAlertAccessScope_viewAllForRBACManage(t *testing.T) {
	eff := &authapp.EffectiveAccessSummary{
		CompanyID:    "c_001",
		MembershipID: "m_102",
		Permissions:  []string{"deadline.view", "rbac.manage"},
	}
	scope := ResolveDeadlineAlertAccessScope(eff)
	if !scope.CanViewAll {
		t.Fatal("expected view all for rbac.manage")
	}
}

func TestResolveDeadlineAlertAccessScope_scopedUserCollectsDepartmentsAndAssignments(t *testing.T) {
	eff := &authapp.EffectiveAccessSummary{
		CompanyID:    "c_001",
		MembershipID: "m_105",
		Permissions:  []string{"deadline.view"},
		DataScope: authapp.EffectiveDataScope{
			Departments: []authapp.DepartmentScope{{DepartmentID: "d_legal"}},
			RecordAssignments: []authapp.ResourceAssignment{
				{ResourceType: "disclosure_record", ResourceID: "rec_assigned"},
			},
		},
	}
	scope := ResolveDeadlineAlertAccessScope(eff)
	if scope.CanViewAll {
		t.Fatal("expected scoped user")
	}
	if len(scope.DepartmentIDs) != 1 || scope.DepartmentIDs[0] != "d_legal" {
		t.Fatalf("departments %+v", scope.DepartmentIDs)
	}
	if _, ok := scope.AssignedRecordIDs["rec_assigned"]; !ok {
		t.Fatal("expected assignment")
	}
}

func TestAllowsRow_departmentRecordMatch(t *testing.T) {
	scope := DeadlineAlertAccessScope{
		DepartmentIDs: []string{"d_legal"},
	}
	if !scope.AllowsRow(AlertRow{RecordID: "r1", RecordDepartmentID: "d_legal"}) {
		t.Fatal("expected department match")
	}
	if scope.AllowsRow(AlertRow{RecordID: "r2", RecordDepartmentID: "d_ir"}) {
		t.Fatal("expected deny for other department")
	}
}

func TestAllowsRow_taskAssigneeMatch(t *testing.T) {
	scope := DeadlineAlertAccessScope{DepartmentIDs: []string{"d_legal"}}
	if !scope.AllowsRow(AlertRow{RecordID: "r1", RecordDepartmentID: "d_ir", HasTaskAssignee: true}) {
		t.Fatal("expected task assignee match")
	}
}

func TestAllowsRow_assignmentMatch(t *testing.T) {
	scope := DeadlineAlertAccessScope{
		AssignedRecordIDs: map[string]struct{}{"r1": {}},
	}
	if !scope.AllowsRow(AlertRow{RecordID: "r1", RecordDepartmentID: "d_ir"}) {
		t.Fatal("expected assignment match")
	}
}

func TestAllowsRow_currentStepDepartmentMatch(t *testing.T) {
	scope := DeadlineAlertAccessScope{OrgUnitIDs: []string{"ou_dept_legal"}}
	if !scope.AllowsRow(AlertRow{RecordID: "r1", CurrentStepDepartment: "ou_dept_legal"}) {
		t.Fatal("expected current step department match")
	}
}

func TestAllowsRow_unrelatedUserDenied(t *testing.T) {
	scope := DeadlineAlertAccessScope{DepartmentIDs: []string{"d_legal"}}
	if scope.AllowsRow(AlertRow{RecordID: "r9", RecordDepartmentID: "general"}) {
		t.Fatal("expected deny for general without assignee")
	}
}
