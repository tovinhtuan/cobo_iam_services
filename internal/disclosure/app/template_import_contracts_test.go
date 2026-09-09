package app_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// TestTemplateImportContracts_ExampleFileParses verifies the canonical example JSON file
// parses cleanly into the TemplateImportEnvelopeV1 DTO without errors.
func TestTemplateImportContracts_ExampleFileParses(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "..", "docs", "schema", "template-import-v1.example.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read example schema file at %s: %v", schemaPath, err)
	}

	var env disclosureapp.TemplateImportEnvelopeV1
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("failed to decode example template with DisallowUnknownFields: %v", err)
	}

	if env.SchemaVersion != disclosureapp.TemplateImportSchemaVersion {
		t.Errorf("schema_version = %q, want %q", env.SchemaVersion, disclosureapp.TemplateImportSchemaVersion)
	}
	if env.Template.Name == "" {
		t.Error("template name is empty")
	}
	if env.Template.TemplateCategory != disclosureapp.TemplateCategoryPeriodic {
		t.Errorf("template_category = %q, want %q", env.Template.TemplateCategory, disclosureapp.TemplateCategoryPeriodic)
	}
	if env.Template.DeadlineConfig == nil {
		t.Fatal("deadline_config is nil")
	}
	if env.Template.DeadlineConfig.FrequencyUnit != "QUARTERLY" {
		t.Errorf("frequency_unit = %q, want QUARTERLY", env.Template.DeadlineConfig.FrequencyUnit)
	}
	if env.Template.DeadlineConfig.ApplicableFromMode != disclosureapp.ApplicableFromModeNext {
		t.Errorf("applicable_from_mode = %q, want NEXT_SLOT", env.Template.DeadlineConfig.ApplicableFromMode)
	}
	if env.Template.DeadlineConfig.ApplicableTo != "" {
		t.Errorf("applicable_to = %q, want open-ended (empty) for durable example", env.Template.DeadlineConfig.ApplicableTo)
	}
	if env.Template.ApplicabilityRules == nil || len(env.Template.ApplicabilityRules.ApplicableSectors) == 0 {
		t.Fatal("expected explicit non-default applicability_rules in canonical example")
	}
	if env.Template.Workflow == nil || len(env.Template.Workflow.Steps) != 3 {
		t.Fatalf("expected 3 workflow steps, got %v", env.Template.Workflow)
	}

	// Verify step 1 documents / portable department shape
	s1 := env.Template.Workflow.Steps[0]
	if s1.Department == nil || s1.Department.Code == "" || s1.Department.Name == "" {
		t.Fatalf("expected department{code,name} on step 1, got %#v", s1.Department)
	}
	if len(s1.AssigneeRoles) == 0 {
		t.Fatal("expected assignee_roles on step 1")
	}
	if len(s1.Documents) != 2 {
		t.Errorf("step 1 document count = %d, want 2", len(s1.Documents))
	}
	if s1.Documents[0].TemplateFileName == "" {
		t.Error("document 1 template_file_name is empty")
	}
	if s1.Documents[0].TemplateFileID != "" {
		t.Error("document 1 template_file_id must be empty in example file")
	}
}

// TestTemplateImportContracts_StrictUnknownFieldRejection verifies that
// unknown business fields at every layer (top-level, template, deadline_config, workflow, document)
// are strictly rejected by the Go DTO model with DisallowUnknownFields().
func TestTemplateImportContracts_StrictUnknownFieldRejection(t *testing.T) {
	tests := []struct {
		name          string
		jsonInput     string
		errSubstrings []string
	}{
		{
			name: "unknown top-level field",
			jsonInput: `{
				"schema_version": "1.0",
				"unknown_top_field": "hack",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5"
				}
			}`,
			errSubstrings: []string{"unknown field", "unknown_top_field"},
		},
		{
			name: "unknown template business field",
			jsonInput: `{
				"schema_version": "1.0",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5",
					"arbitrary_custom_field": "unsupported"
				}
			}`,
			errSubstrings: []string{"unknown field", "arbitrary_custom_field"},
		},
		{
			name: "forbidden runtime field: active_version_no in template",
			jsonInput: `{
				"schema_version": "1.0",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5",
					"active_version_no": 2
				}
			}`,
			errSubstrings: []string{"unknown field", "active_version_no"},
		},
		{
			name: "forbidden runtime field: is_released in template",
			jsonInput: `{
				"schema_version": "1.0",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5",
					"is_released": true
				}
			}`,
			errSubstrings: []string{"unknown field", "is_released"},
		},
		{
			name: "forbidden runtime field: resolved_due_at in template",
			jsonInput: `{
				"schema_version": "1.0",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5",
					"resolved_due_at": "2026-09-08T00:00:00Z"
				}
			}`,
			errSubstrings: []string{"unknown field", "resolved_due_at"},
		},
		{
			name: "unknown deadline_config field",
			jsonInput: `{
				"schema_version": "1.0",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5",
					"deadline_config": {
						"frequency_unit": "MONTHLY",
						"unexpected_deadline_prop": 123
					}
				}
			}`,
			errSubstrings: []string{"unknown field", "unexpected_deadline_prop"},
		},
		{
			name: "unknown workflow step field",
			jsonInput: `{
				"schema_version": "1.0",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5",
					"workflow": {
						"steps": [
							{
								"stage": "Review",
								"unknown_step_attribute": true
							}
						]
					}
				}
			}`,
			errSubstrings: []string{"unknown field", "unknown_step_attribute"},
		},
		{
			name: "unknown workflow document field",
			jsonInput: `{
				"schema_version": "1.0",
				"template": {
					"name": "T1",
					"template_category": "periodic",
					"deadline_rule": "T+5",
					"workflow": {
						"steps": [
							{
								"stage": "Review",
								"documents": [
									{
										"name": "Doc 1",
										"unexpected_doc_attribute": "value"
									}
								]
							}
						]
					}
				}
			}`,
			errSubstrings: []string{"unknown field", "unexpected_doc_attribute"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var env disclosureapp.TemplateImportEnvelopeV1
			dec := json.NewDecoder(strings.NewReader(tc.jsonInput))
			dec.DisallowUnknownFields()
			err := dec.Decode(&env)
			if err == nil {
				t.Fatalf("expected error containing %v, got nil", tc.errSubstrings)
			}
			for _, sub := range tc.errSubstrings {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q does not contain %q", err.Error(), sub)
				}
			}
		})
	}
}

// TestTemplateImportContracts_PeriodicityV2Variants proves that all Periodicity V2
// variants (DAILY, WEEKLY, MONTHLY, QUARTERLY, YEARLY, CALENDAR_DAYS, WORKING_DAYS, IRREGULAR)
// can be faithfully represented by the import contracts.
func TestTemplateImportContracts_PeriodicityV2Variants(t *testing.T) {
	cases := []struct {
		name         string
		freq         string
		anchorDay    *int
		anchorWkday  string
		monthInQtr   *int
		durType      string
		deadDurType  string
		days         *int
	}{
		{
			name:        "DAILY CALENDAR_DAYS",
			freq:        "DAILY",
			durType:     "CALENDAR_DAYS",
			deadDurType: "CALENDAR_DAYS",
		},
		{
			name:        "DAILY WORKING_DAYS",
			freq:        "DAILY",
			durType:     "WORKING_DAYS",
			deadDurType: "WORKING_DAYS",
		},
		{
			name:        "WEEKLY Monday CALENDAR_DAYS",
			freq:        "WEEKLY",
			anchorWkday: "monday",
			durType:     "CALENDAR_DAYS",
			deadDurType: "CALENDAR_DAYS",
		},
		{
			name:        "MONTHLY Day 15 WORKING_DAYS",
			freq:        "MONTHLY",
			anchorDay:   ptr(15),
			durType:     "WORKING_DAYS",
			deadDurType: "WORKING_DAYS",
		},
		{
			name:        "QUARTERLY Month 3 Day 31",
			freq:        "QUARTERLY",
			anchorDay:   ptr(31),
			monthInQtr:  ptr(3),
			durType:     "CALENDAR_DAYS",
			deadDurType: "CALENDAR_DAYS",
			days:        ptr(20),
		},
		{
			name:        "YEARLY Day 31 CALENDAR_DAYS",
			freq:        "YEARLY",
			anchorDay:   ptr(31),
			durType:     "CALENDAR_DAYS",
			deadDurType: "CALENDAR_DAYS",
			days:        ptr(90),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := disclosureapp.TemplateImportDeadlineConfigV1{
				FrequencyUnit:        c.freq,
				CycleAnchorDay:       c.anchorDay,
				CycleAnchorWeekday:   c.anchorWkday,
				MonthInQuarter:       c.monthInQtr,
				DurationType:         c.durType,
				DeadlineDurationType: c.deadDurType,
				DeadlineDays:         c.days,
			}
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			var decoded disclosureapp.TemplateImportDeadlineConfigV1
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if decoded.FrequencyUnit != c.freq {
				t.Errorf("FrequencyUnit = %q, want %q", decoded.FrequencyUnit, c.freq)
			}
			if decoded.DurationType != c.durType {
				t.Errorf("DurationType = %q, want %q", decoded.DurationType, c.durType)
			}
		})
	}
}

// TestTemplateImportContracts_ApplicabilitySemantics verifies representation of
// NEXT_SLOT, CURRENT_SLOT, SPECIFIC_SLOT and ApplicableTo (future, past, open-ended).
func TestTemplateImportContracts_ApplicabilitySemantics(t *testing.T) {
	cases := []struct {
		name        string
		mode        string
		slot        string
		to          string
		description string
	}{
		{
			name:        "NEXT_SLOT relative",
			mode:        "NEXT_SLOT",
			slot:        "",
			to:          "",
			description: "OPEN_ENDED with NEXT_SLOT",
		},
		{
			name:        "CURRENT_SLOT relative",
			mode:        "CURRENT_SLOT",
			slot:        "",
			to:          "2030-01-01",
			description: "Future ApplicableTo with CURRENT_SLOT",
		},
		{
			name:        "SPECIFIC_SLOT with cycle label",
			mode:        "SPECIFIC_SLOT",
			slot:        "2026-Q1",
			to:          "2025-12-31",
			description: "Past ApplicableTo preserved in Draft",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := disclosureapp.TemplateImportDeadlineConfigV1{
				ApplicableFromMode: tc.mode,
				ApplicableFromSlot: tc.slot,
				ApplicableTo:       tc.to,
			}
			b, err := json.Marshal(cfg)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			var out disclosureapp.TemplateImportDeadlineConfigV1
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if out.ApplicableFromMode != tc.mode {
				t.Errorf("mode = %q, want %q", out.ApplicableFromMode, tc.mode)
			}
			if out.ApplicableFromSlot != tc.slot {
				t.Errorf("slot = %q, want %q", out.ApplicableFromSlot, tc.slot)
			}
			if out.ApplicableTo != tc.to {
				t.Errorf("to = %q, want %q", out.ApplicableTo, tc.to)
			}
		})
	}
}

// TestTemplateImportContracts_ConfirmResponseLifecycle verifies that the Confirm response
// contract faithfully represents a new draft root without a fake "status": "draft" field.
func TestTemplateImportContracts_ConfirmResponseLifecycle(t *testing.T) {
	resp := disclosureapp.ConfirmTemplateImportResponse{
		TypeID:      "dt-imported-test",
		VersionNo:   1,
		IsActive:    false,
		IsReleased:  false,
		PortalState: "not_active",
		RootStatus:  "active",
		Name:        "Test Imported Template",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	if m["version_no"] != float64(1) {
		t.Errorf("version_no = %v, want 1", m["version_no"])
	}
	if m["is_active"] != false {
		t.Errorf("is_active = %v, want false", m["is_active"])
	}
	if m["is_released"] != false {
		t.Errorf("is_released = %v, want false", m["is_released"])
	}
	if m["portal_state"] != "not_active" {
		t.Errorf("portal_state = %v, want not_active", m["portal_state"])
	}
	if m["root_status"] != "active" {
		t.Errorf("root_status = %v, want active", m["root_status"])
	}
	if _, exists := m["status"]; exists {
		t.Error("unexpected 'status' field: must not return fake status='draft'")
	}
}

// TestTemplateImportContracts_DefaultTaxonomyConstants verifies the canonical
// taxonomy defaults established during source reconciliation.
func TestTemplateImportContracts_DefaultTaxonomyConstants(t *testing.T) {
	if disclosureapp.DefaultPeriodicGroupID != "group-001" {
		t.Errorf("DefaultPeriodicGroupID = %q, want group-001", disclosureapp.DefaultPeriodicGroupID)
	}
	if disclosureapp.DefaultIrregularGroupID != "group-002" {
		t.Errorf("DefaultIrregularGroupID = %q, want group-002", disclosureapp.DefaultIrregularGroupID)
	}
	if disclosureapp.DefaultPeriodicDisplayGroupCode != "display_groups_003" {
		t.Errorf("DefaultPeriodicDisplayGroupCode = %q, want display_groups_003", disclosureapp.DefaultPeriodicDisplayGroupCode)
	}
	if disclosureapp.DefaultIrregularDisplayGroupCode != "display_groups_001" {
		t.Errorf("DefaultIrregularDisplayGroupCode = %q, want display_groups_001", disclosureapp.DefaultIrregularDisplayGroupCode)
	}
}

func ptr[T any](v T) *T {
	return &v
}
