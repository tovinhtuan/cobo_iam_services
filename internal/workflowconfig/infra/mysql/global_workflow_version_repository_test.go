package mysql

import (
	"os"
	"strings"
	"testing"
)

func TestActivateGlobalWorkflowIsHistoryOnlyAndCannotPublishTemplate(t *testing.T) {
	raw, err := os.ReadFile("global_workflow_version_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (r *VersionRepository) Activate(")
	if start < 0 {
		t.Fatal("Activate implementation not found")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}

	if strings.Contains(body, "UPDATE disclosure_type_versions") ||
		strings.Contains(body, "UPDATE disclosure_types") {
		t.Fatal("global workflow compatibility activate must not release or activate a template")
	}
	if !strings.Contains(body, "UPDATE global_workflow_versions") {
		t.Fatal("compatibility activate must retain global history state transition")
	}
}
