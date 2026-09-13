package app

import (
	"fmt"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// DocumentRequirementSnapshot is one frozen document requirement for a workflow instance step.
// Runtime authority after materialization — not live authoring documents[].
type DocumentRequirementSnapshot struct {
	ID                 string    `json:"id"`
	CompanyID          string    `json:"company_id"`
	DisclosureRecordID string    `json:"disclosure_record_id"`
	WorkflowInstanceID string    `json:"workflow_instance_id"`
	StepCode           string    `json:"step_code"`
	SourceDocID        string    `json:"source_doc_id"`
	RequirementKey     string    `json:"requirement_key"`
	Name               string    `json:"name"`
	Required           bool      `json:"required"`
	TemplateFileID     string    `json:"template_file_id,omitempty"`
	TemplateFileName   string    `json:"template_file_name,omitempty"`
	Ordinal            int       `json:"ordinal"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
}

// ProjectDocumentRequirementSnapshots maps effective workflow step documents[] into
// snapshot rows (ids / company / instance filled by create path).
// Preserves authoring order. Empty documents → nil/empty slice (valid).
// Does not copy template binaries.
func ProjectDocumentRequirementSnapshots(steps []disclosureapp.WorkflowStepDTO) []DocumentRequirementSnapshot {
	if len(steps) == 0 {
		return nil
	}
	out := make([]DocumentRequirementSnapshot, 0)
	for _, step := range steps {
		stepCode := strings.TrimSpace(step.StepID)
		if stepCode == "" {
			continue
		}
		for i, doc := range step.Documents {
			sourceDocID := strings.TrimSpace(doc.DocID)
			if sourceDocID == "" {
				// Authoring validation requires doc_id; defensive deterministic key for legacy blobs.
				sourceDocID = fmt.Sprintf("auto:%s:%d", stepCode, i)
			}
			name := strings.TrimSpace(doc.Name)
			if name == "" {
				name = sourceDocID
			}
			tplID := strings.TrimSpace(doc.TemplateFileID)
			tplName := strings.TrimSpace(doc.TemplateFileName)
			if tplID == "" {
				tplName = ""
			}
			out = append(out, DocumentRequirementSnapshot{
				StepCode:         stepCode,
				SourceDocID:      sourceDocID,
				RequirementKey:   sourceDocID,
				Name:             name,
				Required:         doc.Required,
				TemplateFileID:   tplID,
				TemplateFileName: tplName,
				Ordinal:          i,
			})
		}
	}
	return out
}
