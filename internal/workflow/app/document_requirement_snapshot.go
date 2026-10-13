package app

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
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

// Column limits of workflow_step_document_requirement_snapshots (0135).
const (
	requirementSourceDocIDMaxLen    = 128
	requirementNameMaxLen           = 512
	requirementTemplateFileIDMaxLen = 64
	requirementTemplateNameMaxLen   = 255
)

// ProjectProposalDocumentRequirementSnapshots maps the document requirements captured into a
// frozen proposal workflow (schema v2|v3) at submit into snapshot rows. StepCode is the proposal
// step id (runtime step_code). Steps without captured content produce no rows. Values are fitted to
// the 0135 column limits and doc ids deduplicated case-insensitively within a step (unique key
// instance+step_code+source_doc_id under a _ci collation) so approve can never fail on insert.
func ProjectProposalDocumentRequirementSnapshots(snap *adhocapp.ProposalWorkflowSnapshot) []DocumentRequirementSnapshot {
	if snap == nil {
		return nil
	}
	var out []DocumentRequirementSnapshot
	for _, step := range snap.Steps {
		stepCode := strings.TrimSpace(step.ID)
		if stepCode == "" || step.Inherited == nil {
			continue
		}
		seen := make(map[string]struct{}, len(step.Inherited.Documents))
		for i, doc := range step.Inherited.Documents {
			sourceDocID := strings.TrimSpace(doc.DocID)
			if sourceDocID == "" || utf8.RuneCountInString(sourceDocID) > requirementSourceDocIDMaxLen {
				sourceDocID = fmt.Sprintf("auto:%s:%d", stepCode, i)
			}
			key := strings.ToLower(sourceDocID)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			name := truncateRunes(strings.TrimSpace(doc.Name), requirementNameMaxLen)
			if name == "" {
				name = sourceDocID
			}
			tplID := strings.TrimSpace(doc.TemplateFileID)
			tplName := truncateRunes(strings.TrimSpace(doc.TemplateFileName), requirementTemplateNameMaxLen)
			if tplID == "" || len(tplID) > requirementTemplateFileIDMaxLen {
				tplID, tplName = "", ""
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

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
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
