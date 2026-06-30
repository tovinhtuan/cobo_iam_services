package validation

import (
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
)

// Run executes all validation stages in locked order (read-only).
func Run(in Input) Result {
	suites := make([]SuiteResult, 0, len(StageOrder))
	var allChecks []Check

	for _, suiteName := range StageOrder {
		var checks []Check
		var skipped string
		switch suiteName {
		case SuiteSchema:
			checks = runSchemaStage(in)
		case SuiteBusiness:
			checks = runBusinessStage(in)
		case SuiteDependency:
			checks = runDependencyStage(in)
		case SuiteConflict:
			checks = runConflictStage(in)
		case SuiteRuntime:
			checks = runRuntimeStage(in)
		case SuitePersistence:
			checks = runPersistenceStage(in)
		case SuiteAudit:
			skipped = "not_implemented_in_workspace"
		case SuiteDispatch:
			skipped = "not_implemented_in_workspace"
		}
		for i := range checks {
			checks[i].Suite = suiteName
		}
		if checks == nil {
			checks = []Check{}
		}
		suitePassed := suitePassed(checks)
		suites = append(suites, SuiteResult{
			Suite:         suiteName,
			Passed:        suitePassed,
			Checks:        checks,
			SkippedReason: skipped,
		})
		allChecks = append(allChecks, checks...)
	}

	summary := summarize(allChecks)
	validatedAt := in.ValidatedAt
	if validatedAt.IsZero() {
		validatedAt = in.Snapshot.EvaluatedAt
	}

	return Result{
		Passed:      summary.Blocking == 0,
		ValidatedAt: validatedAt,
		CompanyID:   in.CompanyID,
		Suites:      suites,
		Summary:     summary,
	}
}

func runSchemaStage(in Input) []Check {
	var checks []Check
	snap := in.Snapshot
	if snap == nil {
		return checks
	}
	if !snap.AlertChannelPrefsExists {
		checks = append(checks, Check{
			Code:       "notification.storage_not_configured",
			Severity:   SeverityInfo,
			Message:    "Chưa có cấu hình kênh cảnh báo (alert channel prefs).",
			ActionLink: "/app/admin?tab=notifications",
			Evidence:   map[string]any{"rule_code": conflict.AlertChannelPrefsRuleCode},
		})
	} else if validators.ValidatePrefs != nil {
		valid, issues := validators.ValidatePrefs(snap.AlertChannelPrefsPayload)
		if !valid {
			checks = append(checks, Check{
				Code:       "schema.notification_prefs_invalid",
				Severity:   SeverityBlocking,
				Message:    strings.Join(issues, "; "),
				ActionLink: "/app/admin?tab=notifications",
				Evidence:   map[string]any{"issues": issues, "rule_id": snap.AlertChannelPrefsRuleID},
			})
		}
	}
	if validators.ValidateDepartmentName != nil {
		for _, d := range in.Departments {
			name := strings.TrimSpace(d.Name)
			if name == "" {
				continue
			}
			if _, err := validators.ValidateDepartmentName(name); err != nil {
				checks = append(checks, Check{
					Code:       "schema.department_name_invalid",
					Severity:   SeverityWarning,
					Message:    err.Error(),
					ActionLink: "/app/admin?tab=org",
					Evidence:   map[string]any{"department_id": d.DepartmentID, "name": name},
				})
			}
		}
	}
	return checks
}

func runBusinessStage(in Input) []Check {
	var checks []Check
	snap := in.Snapshot
	if snap == nil {
		return checks
	}
	for _, row := range snap.NonGrantableDirectPermissions {
		checks = append(checks, Check{
			Code:       "business.rbac.non_grantable_direct",
			Severity:   SeverityWarning,
			Message:    "Direct grant cho permission không grantable.",
			ActionLink: "/app/admin?tab=rbac",
			Evidence: map[string]any{
				"membership_id":   row.MembershipID,
				"permission_code": row.PermissionCode,
			},
		})
	}
	for _, role := range snap.Roles {
		if role.Status != "" && role.Status != "active" {
			continue
		}
		if role.MemberCount > 0 {
			continue
		}
		perms := snap.RolePermissionCodes[role.RoleID]
		critical := false
		for _, p := range perms {
			if conflictValidatorsPermissionRisk(p) == "critical" {
				critical = true
				break
			}
		}
		if critical {
			checks = append(checks, Check{
				Code:       "business.rbac.critical_role_empty",
				Severity:   SeverityWarning,
				Message:    "Vai trò critical chưa có thành viên.",
				ActionLink: "/app/admin?tab=rbac",
				Evidence: map[string]any{
					"role_id": role.RoleID, "role_code": role.RoleCode, "member_count": 0,
				},
			})
		}
	}
	if in.CompanyAdminCount == 0 {
		checks = append(checks, Check{
			Code:       "business.admin.no_primary",
			Severity:   SeverityWarning,
			Message:    "Công ty chưa có company admin được gán.",
			ActionLink: "/app/admin?tab=users",
			Evidence:   map[string]any{"company_admin_count": 0},
		})
	}
	return checks
}

func runDependencyStage(in Input) []Check {
	var checks []Check
	snap := in.Snapshot
	if snap == nil {
		return checks
	}
	roleIDs := map[string]struct{}{}
	for _, r := range snap.Roles {
		roleIDs[r.RoleID] = struct{}{}
	}
	for _, rule := range snap.WorkflowAssigneeRules {
		for _, roleID := range extractRoleIDs(rule.Payload) {
			if roleID == "" {
				continue
			}
			if _, ok := roleIDs[roleID]; !ok {
				checks = append(checks, Check{
					Code:       "dependency.role.not_found",
					Severity:   SeverityWarning,
					Message:    "Workflow assignee rule tham chiếu role không tồn tại.",
					ActionLink: "/app/admin?tab=rbac",
					Evidence: map[string]any{
						"workflow_assignee_rule_id": rule.RuleID,
						"role_id":                   roleID,
					},
				})
			}
		}
	}
	return checks
}

func runConflictStage(in Input) []Check {
	checks := make([]Check, 0, len(in.ConflictOutput.Results))
	for _, r := range in.ConflictOutput.Results {
		msg := r.Description
		if msg == "" {
			msg = r.Title
		}
		checks = append(checks, Check{
			Code:       r.Code,
			Severity:   r.Severity,
			Message:    msg,
			ActionLink: r.ActionLink,
			Evidence:   r.Evidence,
		})
	}
	return checks
}

func runRuntimeStage(in Input) []Check {
	var checks []Check
	snap := in.Snapshot
	if snap == nil || !snap.AlertChannelPrefsExists {
		return checks
	}
	if !in.RuntimeConsumerEnabled {
		checks = append(checks, Check{
			Code:       "runtime.consumer.misconfigured",
			Severity:   SeverityInfo,
			Message:    "Runtime consumer chưa bật — prefs đã lưu nhưng dispatch runtime chưa đọc.",
			ActionLink: "/app/admin?tab=notifications",
			Evidence:   map[string]any{"runtime_consumer_enabled": false},
		})
	}
	return checks
}

func runPersistenceStage(in Input) []Check {
	if in.CanonicalAlertPrefsRuleCount > 1 {
		return []Check{{
			Code:       "persistence.notification.duplicate_canonical_rule",
			Severity:   SeverityWarning,
			Message:    "Nhiều hơn một notification rule cho mã canonical alert channel prefs.",
			ActionLink: "/app/admin?tab=notifications",
			Evidence:   map[string]any{"count": in.CanonicalAlertPrefsRuleCount},
		}}
	}
	return nil
}

func conflictValidatorsPermissionRisk(code string) string {
	if conflictValidators.PermissionRiskLevel != nil {
		return conflictValidators.PermissionRiskLevel(code)
	}
	return ""
}

// conflictValidators reuses conflict package registered validators.
var conflictValidators = struct {
	PermissionRiskLevel func(string) string
}{}

func init() {
	// wired from app init after conflict.RegisterValidators
}

func extractRoleIDs(payload map[string]any) []string {
	if payload == nil {
		return nil
	}
	var ids []string
	if s, ok := payload["role_id"].(string); ok && s != "" {
		ids = append(ids, s)
	}
	if s, ok := payload["assignee_role_id"].(string); ok && s != "" {
		ids = append(ids, s)
	}
	if arr, ok := payload["assignee_role_ids"].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				ids = append(ids, s)
			}
		}
	}
	return ids
}

func suitePassed(checks []Check) bool {
	for _, c := range checks {
		if c.Severity == SeverityBlocking {
			return false
		}
	}
	return true
}

func summarize(checks []Check) Summary {
	s := Summary{Total: len(checks)}
	for _, c := range checks {
		switch c.Severity {
		case SeverityBlocking:
			s.Blocking++
			s.Failed++
		case SeverityWarning:
			s.Warning++
			s.Failed++
		case SeverityInfo:
			s.Info++
		}
	}
	return s
}

// WireConflictValidators connects permission risk helper from app init.
func WireConflictValidators(riskFn func(string) string) {
	conflictValidators.PermissionRiskLevel = riskFn
}
