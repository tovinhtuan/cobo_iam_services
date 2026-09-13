package app

import (
	"strings"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func validPinnedStep(id, stage, desc string) WorkflowPublicationStep {
	return WorkflowPublicationStep{WorkflowStepDTO: WorkflowStepDTO{
		StepID: id, Stage: stage, DepartmentID: "d1", AssigneeRoleIds: []string{"r1"},
		ProcessingDays: 1, DueRule: "T+1", Description: desc,
	}}
}

func TestApplyActivationReadiness_DescriptionRequired(t *testing.T) {
	item := &DisclosureTypeDTO{
		TypeID:                "dt-desc",
		VersionNo:             1,
		WorkflowAuthorityMode: WorkflowAuthorityTemplatePinned,
		WorkflowManifest: &WorkflowPublicationManifest{
			SchemaVersion: WorkflowManifestSchemaVersion,
			Steps: []WorkflowPublicationStep{
				validPinnedStep("s1", "Thu thập", ""),
				validPinnedStep("s2", "Rà soát", "Có mô tả"),
				{WorkflowStepDTO: WorkflowStepDTO{
					StepID: "s3", Stage: "Phê duyệt", DepartmentID: "d1",
					AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1",
					Description: "<p></p>", DescriptionFormat: "safe_html",
				}},
			},
		},
	}
	applyActivationReadiness(item, time.Now().UTC(), nil)
	if item.ActivationReady {
		t.Fatal("want not ready")
	}
	n := 0
	for _, b := range item.ActivationBlockers {
		if b.Code == ActivationBlockerWorkflowStepDescriptionRequired {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("want 2 description blockers, got %v", item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_MixedBlockersPreserveDeptAndDescription(t *testing.T) {
	item := &DisclosureTypeDTO{
		TypeID:                "dt-mixed",
		VersionNo:             1,
		WorkflowAuthorityMode: WorkflowAuthorityTemplatePinned,
		WorkflowManifest: &WorkflowPublicationManifest{
			SchemaVersion: WorkflowManifestSchemaVersion,
			Steps: []WorkflowPublicationStep{
				{WorkflowStepDTO: WorkflowStepDTO{
					StepID: "s1", Stage: "Thu thập", DepartmentID: "",
					AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, Description: "",
				}},
			},
		},
	}
	applyActivationReadiness(item, time.Now().UTC(), nil)
	hasDept, hasDesc := false, false
	for _, b := range item.ActivationBlockers {
		if b.Code == "WORKFLOW_STEP_DEPARTMENT_REQUIRED" {
			hasDept = true
		}
		if b.Code == ActivationBlockerWorkflowStepDescriptionRequired {
			hasDesc = true
		}
	}
	if !hasDept || !hasDesc {
		t.Fatalf("want both dept+desc blockers, got %v", item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_EmptyDocumentsNotBlocked(t *testing.T) {
	item := &DisclosureTypeDTO{
		TypeID:                "dt-docs",
		VersionNo:             1,
		WorkflowAuthorityMode: WorkflowAuthorityTemplatePinned,
		WorkflowManifest: &WorkflowPublicationManifest{
			SchemaVersion: WorkflowManifestSchemaVersion,
			Steps: []WorkflowPublicationStep{
				{WorkflowStepDTO: WorkflowStepDTO{
					StepID: "s1", Stage: "A", DepartmentID: "d1",
					AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1",
					Description: "Mô tả", Documents: []WorkflowDocumentDTO{},
				}},
			},
		},
	}
	applyActivationReadiness(item, time.Now().UTC(), nil)
	if !item.ActivationReady {
		t.Fatalf("documents=[] must not block, blockers=%v", item.ActivationBlockers)
	}
	for _, b := range item.ActivationBlockers {
		if strings.Contains(strings.ToLower(b.Code), "document") {
			t.Fatalf("unexpected document blocker %v", b)
		}
	}
}

func TestValidateCompanyWorkflowOverrideStepDescriptionsForActivation(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideStepDescriptionsForActivation([]WorkflowStepDTO{
		{StepID: "s1", Stage: "Review", Description: "OK"},
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	err = ValidateCompanyWorkflowOverrideStepDescriptionsForActivation([]WorkflowStepDTO{
		{StepID: "s1", Stage: "Review", Description: ""},
		{StepID: "s2", Stage: "Approve", Description: "<p><br></p>", DescriptionFormat: "safe_html"},
	})
	if err == nil {
		t.Fatal("want error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || string(he.Code) != ActivationBlockerWorkflowStepDescriptionRequired {
		t.Fatalf("err=%v", err)
	}
	fe, _ := he.Details["field_errors"].(map[string]string)
	if len(fe) != 2 {
		t.Fatalf("field_errors=%v", he.Details["field_errors"])
	}
}

func TestValidateCompanyWorkflowOverrideSteps_DraftAllowsEmptyDescription(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{StepID: "s1", Stage: "Review", ProcessingDays: 1, Description: ""},
	})
	if err != nil {
		t.Fatalf("draft must allow empty description, got %v", err)
	}
}

func TestTemplateOverrideEmptySemanticParity(t *testing.T) {
	cases := []string{"", "  ", "<p></p>", "<p><br></p>", "&nbsp;"}
	for _, desc := range cases {
		format := "plain_text"
		if strings.Contains(desc, "<") || strings.Contains(desc, "&") {
			format = "safe_html"
		}
		blockers := CollectWorkflowStepDescriptionActivationBlockers([]WorkflowStepDTO{
			{StepID: "s1", Stage: "A", Description: desc, DescriptionFormat: format},
		})
		err := ValidateCompanyWorkflowOverrideStepDescriptionsForActivation([]WorkflowStepDTO{
			{StepID: "s1", Stage: "A", Description: desc, DescriptionFormat: format},
		})
		if len(blockers) != 1 {
			t.Fatalf("template blockers for %q: %v", desc, blockers)
		}
		if err == nil {
			t.Fatalf("override must reject %q", desc)
		}
	}
}
