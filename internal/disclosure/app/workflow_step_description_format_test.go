package app

import "testing"

func TestNormalizeWorkflowStepDescriptionFormat(t *testing.T) {
	if got := NormalizeWorkflowStepDescriptionFormat(""); got != WorkflowStepDescriptionFormatPlainText {
		t.Fatalf("empty → plain, got %q", got)
	}
	if got := NormalizeWorkflowStepDescriptionFormat("SAFE_HTML"); got != WorkflowStepDescriptionFormatSafeHTML {
		t.Fatalf("safe_html, got %q", got)
	}
	if got := NormalizeWorkflowStepDescriptionFormat("raw_html"); got != WorkflowStepDescriptionFormatPlainText {
		t.Fatalf("unknown on read falls back plain, got %q", got)
	}
}

func TestValidateWorkflowStepDescriptionFormatForPersist(t *testing.T) {
	if err := ValidateWorkflowStepDescriptionFormatForPersist(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorkflowStepDescriptionFormatForPersist("plain_text"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorkflowStepDescriptionFormatForPersist("safe_html"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorkflowStepDescriptionFormatForPersist("raw_html"); err == nil {
		t.Fatal("expected reject unknown format")
	}
}
