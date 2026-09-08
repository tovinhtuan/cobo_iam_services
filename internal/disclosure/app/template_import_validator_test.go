package app_test

import (
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestTemplateImportValidator_PeriodicityV2Variants(t *testing.T) {
	validDisplayGroups := map[string]struct{}{
		"display_groups_003": {},
		"display_groups_001": {},
	}
	departmentsByCode := map[string]disclosureapp.TemplateDepartmentDTO{
		"dept-001": {DepartmentCode: "dept-001", DepartmentName: "Phòng Kế toán"},
	}
	departmentsByName := map[string]disclosureapp.TemplateDepartmentDTO{
		"phòng kế toán": {DepartmentCode: "dept-001", DepartmentName: "Phòng Kế toán"},
	}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	t.Run("valid QUARTERLY with Working Days", func(t *testing.T) {
		day := 31
		month := 3
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo Quý",
			TemplateCategory:  "periodic",
			Periodicity:       "quarterly",
			DeadlineRule:      "Trong vòng 20 ngày kể từ khi kết thúc quý.",
			DisplayGroupCodes: []string{"display_groups_003"},
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:        "QUARTERLY",
				CycleAnchorDay:       &day,
				MonthInQuarter:       &month,
				DurationType:         "WORKING_DAYS",
				DeadlineDurationType: "WORKING_DAYS",
			},
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:           "Bước 1",
						DepartmentID:    "dept-001",
						AssigneeRoleIDs: []string{"reviewer", "approver"},
						ProcessingDays:  3,
					},
				},
			},
		}

		errs, warns, mappings, actBlockers := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)

		if len(errs) != 0 {
			t.Fatalf("unexpected validation errors: %v", errs)
		}
		if len(warns) != 0 {
			t.Errorf("unexpected warnings: %v", warns)
		}
		if len(mappings) != 0 {
			t.Errorf("unexpected mappings: %v", mappings)
		}
		if len(actBlockers) != 0 {
			t.Errorf("unexpected activation blockers: %v", actBlockers)
		}
	})

	t.Run("valid WEEKLY with Weekday Anchor", func(t *testing.T) {
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo Tuần",
			TemplateCategory:  "periodic",
			Periodicity:       "weekly",
			DeadlineRule:      "Thứ Hai hàng tuần",
			DisplayGroupCodes: []string{"display_groups_003"},
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:      "WEEKLY",
				CycleAnchorWeekday: "monday",
				DurationType:       "CALENDAR_DAYS",
			},
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:           "Giai đoạn duyệt",
						DepartmentID:    "dept-001",
						AssigneeRoleIDs: []string{"approver"},
						ProcessingDays:  1,
					},
				},
			},
		}

		errs, _, _, _ := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)
		if len(errs) != 0 {
			t.Fatalf("unexpected validation errors: %v", errs)
		}
	})

	t.Run("negative anchor day out of bounds", func(t *testing.T) {
		badDay := 32
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo",
			TemplateCategory:  "periodic",
			Periodicity:       "monthly",
			DeadlineRule:      "T+5",
			DisplayGroupCodes: []string{"display_groups_003"},
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:  "MONTHLY",
				CycleAnchorDay: &badDay,
			},
		}

		errs, _, _, _ := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)
		if len(errs) == 0 {
			t.Fatal("expected validation error for anchor day 32, got none")
		}
		found := false
		for _, e := range errs {
			if e.Code == "INVALID_CYCLE_ANCHOR_DAY" && e.FieldPath == "template.deadline_config.cycle_anchor_day" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected INVALID_CYCLE_ANCHOR_DAY error, got %v", errs)
		}
	})

	t.Run("negative month in quarter out of bounds", func(t *testing.T) {
		badMonth := 4
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo",
			TemplateCategory:  "periodic",
			Periodicity:       "quarterly",
			DeadlineRule:      "T+5",
			DisplayGroupCodes: []string{"display_groups_003"},
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:  "QUARTERLY",
				MonthInQuarter: &badMonth,
			},
		}

		errs, _, _, _ := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)
		found := false
		for _, e := range errs {
			if e.Code == "INVALID_MONTH_IN_QUARTER" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected INVALID_MONTH_IN_QUARTER, got %v", errs)
		}
	})

	t.Run("negative irregular carrying periodic anchors", func(t *testing.T) {
		day := 15
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:             "Công bố bất thường",
			TemplateCategory: "irregular",
			DeadlineRule:     "Trong vòng 24 giờ",
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:  "MONTHLY",
				CycleAnchorDay: &day,
			},
		}

		errs, _, _, _ := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)
		found := false
		for _, e := range errs {
			if e.Code == "IRREGULAR_HAS_PERIODIC_ANCHORS" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected IRREGULAR_HAS_PERIODIC_ANCHORS, got %v", errs)
		}
	})
}

func TestTemplateImportValidator_SeparationOfDraftValidityAndActivationReadiness(t *testing.T) {
	validDisplayGroups := map[string]struct{}{
		"display_groups_003": {},
	}
	departmentsByCode := map[string]disclosureapp.TemplateDepartmentDTO{
		"dept-001": {DepartmentCode: "dept-001", DepartmentName: "Phòng Kế toán"},
	}
	departmentsByName := map[string]disclosureapp.TemplateDepartmentDTO{
		"phòng kế toán": {DepartmentCode: "dept-001", DepartmentName: "Phòng Kế toán"},
	}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	t.Run("valid past ApplicableTo emits Draft Warning and Activation Blocker without blocking Draft Import", func(t *testing.T) {
		day := 15
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo lịch sử",
			TemplateCategory:  "periodic",
			Periodicity:       "monthly",
			DeadlineRule:      "T+5",
			DisplayGroupCodes: []string{"display_groups_003"},
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:  "MONTHLY",
				CycleAnchorDay: &day,
				ApplicableTo:   "2025-12-31", // Valid past date
			},
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:           "Phê duyệt",
						DepartmentID:    "dept-001",
						AssigneeRoleIDs: []string{"approver"},
						ProcessingDays:  2,
					},
				},
			},
		}

		errs, warns, mappings, actBlockers := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)

		// Must NOT produce domain blockers!
		if len(errs) != 0 {
			t.Fatalf("expected 0 domain blockers for past ApplicableTo, got %v", errs)
		}
		if len(mappings) != 0 {
			t.Errorf("expected 0 mappings, got %v", mappings)
		}

		// Must produce Draft warning
		foundWarning := false
		for _, w := range warns {
			if w.Code == "APPLICABLE_TO_IN_PAST" {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Errorf("expected APPLICABLE_TO_IN_PAST warning, got %v", warns)
		}

		// Must produce Activation blocker
		foundActBlocker := false
		for _, ab := range actBlockers {
			if ab.Code == "APPLICABLE_TO_EXPIRED" {
				foundActBlocker = true
				break
			}
		}
		if !foundActBlocker {
			t.Errorf("expected APPLICABLE_TO_EXPIRED activation blocker, got %v", actBlockers)
		}
	})

	t.Run("unmatched department emits required mapping and warning, leaving domain valid", func(t *testing.T) {
		day := 15
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo phòng ban mới",
			TemplateCategory:  "periodic",
			Periodicity:       "monthly",
			DeadlineRule:      "T+5",
			DisplayGroupCodes: []string{"display_groups_003"},
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:  "MONTHLY",
				CycleAnchorDay: &day,
			},
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:           "Bước rà soát",
						DepartmentID:    "dept-unmatched-999",
						DepartmentName:  "Ban Quản lý Rủi ro Nguồn",
						AssigneeRoleIDs: []string{"approver"},
						ProcessingDays:  2,
					},
				},
			},
		}

		errs, warns, mappings, _ := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)

		if len(errs) != 0 {
			t.Fatalf("expected 0 domain blockers, got %v", errs)
		}
		if len(mappings) != 1 {
			t.Fatalf("expected 1 required mapping, got %d", len(mappings))
		}
		if mappings[0].SourceID != "dept-unmatched-999" || mappings[0].IsAutoMatched != false {
			t.Errorf("mapping = %+v, want un-matched source", mappings[0])
		}

		foundWarning := false
		for _, w := range warns {
			if w.Code == "UNRESOLVED_DEPARTMENT_MAPPING" {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Errorf("expected UNRESOLVED_DEPARTMENT_MAPPING warning, got %v", warns)
		}
	})

	t.Run("unknown workflow role emits domain blocker", func(t *testing.T) {
		day := 15
		tpl := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo lỗi vai trò",
			TemplateCategory:  "periodic",
			Periodicity:       "monthly",
			DeadlineRule:      "T+5",
			DisplayGroupCodes: []string{"display_groups_003"},
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:  "MONTHLY",
				CycleAnchorDay: &day,
			},
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:           "Bước rà soát",
						DepartmentID:    "dept-001",
						AssigneeRoleIDs: []string{"nonexistent_role_xyz"},
						ProcessingDays:  2,
					},
				},
			},
		}

		errs, _, _, _ := disclosureapp.ValidateImportTemplate(
			&tpl, now, validDisplayGroups, departmentsByCode, departmentsByName,
		)

		if len(errs) == 0 {
			t.Fatal("expected domain blocker for unknown workflow role, got none")
		}
		found := false
		for _, e := range errs {
			if e.Code == "UNKNOWN_WORKFLOW_ROLE" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected UNKNOWN_WORKFLOW_ROLE error, got %v", errs)
		}
	})
}
