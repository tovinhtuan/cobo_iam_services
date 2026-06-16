package app

import (
	"encoding/json"
	"strings"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// ActiveDepartmentsFromRow returns Option C using a pre-extracted department (list query).
func ActiveDepartmentsFromRow(currentStepCode, currentStepDepartment string, snapshotJSON []byte) []string {
	if dept := strings.TrimSpace(currentStepDepartment); dept != "" {
		return []string{dept}
	}
	return ActiveDepartmentsFromSnapshot(currentStepCode, snapshotJSON)
}

// ActiveDepartmentsFromSnapshot returns Option C: department of the current workflow step.
func ActiveDepartmentsFromSnapshot(currentStepCode string, snapshotJSON []byte) []string {
	code := strings.TrimSpace(currentStepCode)
	if code == "" || len(snapshotJSON) == 0 {
		return nil
	}
	var steps []workflowapp.StepSnapshot
	if err := json.Unmarshal(snapshotJSON, &steps); err != nil {
		return nil
	}
	for _, step := range steps {
		stepCode := strings.TrimSpace(step.StepCode)
		if stepCode == "" {
			stepCode = strings.TrimSpace(step.StepID)
		}
		if stepCode != code {
			continue
		}
		dept := strings.TrimSpace(step.Department)
		if dept == "" {
			return nil
		}
		return []string{dept}
	}
	return nil
}
