package app_test

import (
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestTemplateImportNormalizer_DefaultsAndDerivations(t *testing.T) {
	t.Run("periodic category defaults group-001 and display_groups_003", func(t *testing.T) {
		raw := disclosureapp.TemplateImportDefinitionV1{
			Name:             "  Báo cáo định kỳ  ",
			TemplateCategory: "periodic",
			DeadlineRule:     "T+5",
		}
		norm := disclosureapp.NormalizeTemplateImportV1(raw)

		if norm.Name != "Báo cáo định kỳ" {
			t.Errorf("Name = %q, want 'Báo cáo định kỳ'", norm.Name)
		}
		if norm.GroupID != disclosureapp.DefaultPeriodicGroupID {
			t.Errorf("GroupID = %q, want %q", norm.GroupID, disclosureapp.DefaultPeriodicGroupID)
		}
		if len(norm.DisplayGroupCodes) != 1 || norm.DisplayGroupCodes[0] != disclosureapp.DefaultPeriodicDisplayGroupCode {
			t.Errorf("DisplayGroupCodes = %v, want [%s]", norm.DisplayGroupCodes, disclosureapp.DefaultPeriodicDisplayGroupCode)
		}
		if norm.DeadlineStrategy != disclosureapp.DeadlineStrategyFixedCycleDays {
			t.Errorf("DeadlineStrategy = %q, want %q", norm.DeadlineStrategy, disclosureapp.DeadlineStrategyFixedCycleDays)
		}
	})

	t.Run("irregular category defaults group-002 and display_groups_001", func(t *testing.T) {
		raw := disclosureapp.TemplateImportDefinitionV1{
			Name:             "Công bố bất thường",
			TemplateCategory: "irregular",
			DeadlineRule:     "Trong vòng 24 giờ",
		}
		norm := disclosureapp.NormalizeTemplateImportV1(raw)

		if norm.GroupID != disclosureapp.DefaultIrregularGroupID {
			t.Errorf("GroupID = %q, want %q", norm.GroupID, disclosureapp.DefaultIrregularGroupID)
		}
		if len(norm.DisplayGroupCodes) != 1 || norm.DisplayGroupCodes[0] != disclosureapp.DefaultIrregularDisplayGroupCode {
			t.Errorf("DisplayGroupCodes = %v, want [%s]", norm.DisplayGroupCodes, disclosureapp.DefaultIrregularDisplayGroupCode)
		}
		if norm.DeadlineStrategy != disclosureapp.DeadlineStrategyEventHours {
			t.Errorf("DeadlineStrategy = %q, want %q", norm.DeadlineStrategy, disclosureapp.DeadlineStrategyEventHours)
		}
	})

	t.Run("explicit display group codes are preserved and trimmed", func(t *testing.T) {
		raw := disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo",
			TemplateCategory:  "periodic",
			DeadlineRule:      "T+5",
			DisplayGroupCodes: []string{"  display_groups_002  ", "display_groups_003", "display_groups_002"},
		}
		norm := disclosureapp.NormalizeTemplateImportV1(raw)

		if len(norm.DisplayGroupCodes) != 2 {
			t.Fatalf("expected 2 unique display group codes, got %v", norm.DisplayGroupCodes)
		}
		if norm.DisplayGroupCodes[0] != "display_groups_002" || norm.DisplayGroupCodes[1] != "display_groups_003" {
			t.Errorf("DisplayGroupCodes = %v", norm.DisplayGroupCodes)
		}
	})
}

func TestTemplateImportNormalizer_ApplicableFromStaleSlotStripping(t *testing.T) {
	t.Run("NEXT_SLOT strips stale frozen slot", func(t *testing.T) {
		raw := disclosureapp.TemplateImportDefinitionV1{
			Name:             "Periodic Report",
			TemplateCategory: "periodic",
			DeadlineRule:     "T+5",
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				ApplicableFromMode: "NEXT_SLOT",
				ApplicableFromSlot: "2025-Q1", // Stale slot from source export!
			},
		}
		norm := disclosureapp.NormalizeTemplateImportV1(raw)

		if norm.DeadlineConfig.ApplicableFromMode != disclosureapp.ApplicableFromModeNext {
			t.Errorf("ApplicableFromMode = %q, want NEXT_SLOT", norm.DeadlineConfig.ApplicableFromMode)
		}
		if norm.DeadlineConfig.ApplicableFromSlot != "" {
			t.Errorf("ApplicableFromSlot = %q, want empty string to prevent freezing", norm.DeadlineConfig.ApplicableFromSlot)
		}
	})

	t.Run("CURRENT_SLOT strips stale frozen slot", func(t *testing.T) {
		raw := disclosureapp.TemplateImportDefinitionV1{
			Name:             "Periodic Report",
			TemplateCategory: "periodic",
			DeadlineRule:     "T+5",
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				ApplicableFromMode: "CURRENT_SLOT",
				ApplicableFromSlot: "2024-12",
			},
		}
		norm := disclosureapp.NormalizeTemplateImportV1(raw)

		if norm.DeadlineConfig.ApplicableFromMode != disclosureapp.ApplicableFromModeCurrent {
			t.Errorf("ApplicableFromMode = %q, want CURRENT_SLOT", norm.DeadlineConfig.ApplicableFromMode)
		}
		if norm.DeadlineConfig.ApplicableFromSlot != "" {
			t.Errorf("ApplicableFromSlot = %q, want empty string to prevent freezing", norm.DeadlineConfig.ApplicableFromSlot)
		}
	})

	t.Run("SPECIFIC_SLOT preserves valid specific slot", func(t *testing.T) {
		raw := disclosureapp.TemplateImportDefinitionV1{
			Name:             "Periodic Report",
			TemplateCategory: "periodic",
			DeadlineRule:     "T+5",
			DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
				ApplicableFromMode: "SPECIFIC_SLOT",
				ApplicableFromSlot: "2026-Q2",
			},
		}
		norm := disclosureapp.NormalizeTemplateImportV1(raw)

		if norm.DeadlineConfig.ApplicableFromMode != disclosureapp.ApplicableFromModeSpecific {
			t.Errorf("ApplicableFromMode = %q, want SPECIFIC_SLOT", norm.DeadlineConfig.ApplicableFromMode)
		}
		if norm.DeadlineConfig.ApplicableFromSlot != "2026-Q2" {
			t.Errorf("ApplicableFromSlot = %q, want '2026-Q2'", norm.DeadlineConfig.ApplicableFromSlot)
		}
	})
}

func TestTemplateImportNormalizer_WorkflowAndDocumentSanitization(t *testing.T) {
	raw := disclosureapp.TemplateImportDefinitionV1{
		Name:             "Template with Workflow",
		TemplateCategory: "periodic",
		DeadlineRule:     "T+5",
		Workflow: &disclosureapp.TemplateImportWorkflowV1{
			Steps: []disclosureapp.TemplateImportWorkflowStepV1{
				{
					StepID:          "source-uuid-12345", // Must be stripped
					Stage:           "  Rà soát pháp lý  ",
					DepartmentID:    "dept-legal",
					DepartmentName:  "Ban Pháp chế",
					AssigneeRoleIDs: []string{"reviewer", "reviewer", " approver "},
					ProcessingDays:  3,
					Documents: []disclosureapp.TemplateImportWorkflowDocumentV1{
						{
							Name:             "  Tài liệu thuyết minh  ",
							Required:         true,
							TemplateFileName: "  Mau_Thuyet_Minh.docx  ",
							TemplateFileID:   "source-storage-id-999", // ACCEPT_AND_CLEAR
						},
					},
				},
			},
		},
	}

	norm := disclosureapp.NormalizeTemplateImportV1(raw)

	if len(norm.Workflow.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(norm.Workflow.Steps))
	}
	step := norm.Workflow.Steps[0]

	if step.StepID != "" {
		t.Errorf("StepID = %q, want empty string (deferred to Phase C)", step.StepID)
	}
	if step.Stage != "Rà soát pháp lý" {
		t.Errorf("Stage = %q, want 'Rà soát pháp lý'", step.Stage)
	}
	if len(step.AssigneeRoleIDs) != 2 {
		t.Errorf("AssigneeRoleIDs = %v, want 2 deduplicated roles", step.AssigneeRoleIDs)
	}
	if step.DisplayOrder != 1 {
		t.Errorf("DisplayOrder = %d, want 1", step.DisplayOrder)
	}

	if len(step.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(step.Documents))
	}
	doc := step.Documents[0]
	if doc.Name != "Tài liệu thuyết minh" {
		t.Errorf("Document Name = %q, want 'Tài liệu thuyết minh'", doc.Name)
	}
	if doc.TemplateFileName != "Mau_Thuyet_Minh.docx" {
		t.Errorf("TemplateFileName = %q, want 'Mau_Thuyet_Minh.docx'", doc.TemplateFileName)
	}
	if doc.TemplateFileID != "" {
		t.Errorf("TemplateFileID = %q, want empty string per ACCEPT_AND_CLEAR contract", doc.TemplateFileID)
	}
}
