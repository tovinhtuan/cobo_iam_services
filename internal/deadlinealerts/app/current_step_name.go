package app

import (
	"encoding/json"
	"strings"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// CurrentStepNameFromRow returns the user-facing workflow step name (Option C).
func CurrentStepNameFromRow(currentStepCode, currentStepName string, snapshotJSON []byte) string {
	if name := strings.TrimSpace(currentStepName); name != "" {
		return name
	}
	return CurrentStepNameFromSnapshot(currentStepCode, snapshotJSON)
}

// CurrentStepNameFromSnapshot resolves display name from frozen workflow snapshot (stage label).
func CurrentStepNameFromSnapshot(currentStepCode string, snapshotJSON []byte) string {
	code := strings.TrimSpace(currentStepCode)
	if code == "" || len(snapshotJSON) == 0 {
		return ""
	}
	var steps []workflowapp.StepSnapshot
	if err := json.Unmarshal(snapshotJSON, &steps); err != nil {
		return ""
	}
	for _, step := range steps {
		stepCode := strings.TrimSpace(step.StepCode)
		if stepCode == "" {
			stepCode = strings.TrimSpace(step.StepID)
		}
		if stepCode != code {
			continue
		}
		return strings.TrimSpace(step.Stage)
	}
	return ""
}
