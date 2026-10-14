package app

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Step additions: proposer-owned content layered on top of the template step content
// (inherited, captured at submit). The template part is never editable in a proposal;
// additions only add a note and extra required documents. For steps without template
// content (self-added, missing in the template, capture disabled) the note is the step
// description.
const (
	MaxProposalStepNoteLength          = 2000
	MaxProposalStepAdditionalDocuments = 20
	MaxProposalStepDocumentNameLength  = 255
	// Whole-proposal caps keep proposed_workflow_json small (list reads decode it for every row).
	MaxProposalAdditionsNoteTotal       = 20000
	MaxProposalAdditionalDocumentsTotal = 100
	// ProposalAdditionalDocIDPrefix keeps addition doc ids apart from template doc ids
	// (0135 unique key instance+step_code+source_doc_id).
	ProposalAdditionalDocIDPrefix = "prop_"
)

var proposalAdditionalDocIDPattern = regexp.MustCompile(`^prop_[A-Za-z0-9_-]{1,64}$`)

// newProposalAdditionalDocID is a var so tests can make generated ids deterministic.
var newProposalAdditionalDocID = func() string {
	return ProposalAdditionalDocIDPrefix + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// ProposalStepAdditionalDocument is one extra document required by the proposal.
// It is always required and never has a template file.
type ProposalStepAdditionalDocument struct {
	DocID string `json:"doc_id,omitempty"`
	Name  string `json:"name"`
}

// ProposalStepAdditions is the proposer-owned part of a step. A non-nil value (even empty)
// marks a draft saved by an additions-aware client (submit then requires a description on
// self-added steps).
type ProposalStepAdditions struct {
	Note      string                           `json:"note,omitempty"`
	Documents []ProposalStepAdditionalDocument `json:"documents,omitempty"`
}

// hasDisallowedRunes reports control characters (allowNewlines keeps \n, \r, \t for notes) and
// invisible / bidi-override characters that could disguise text as something else.
func hasDisallowedRunes(s string, allowNewlines bool) bool {
	for _, r := range s {
		switch {
		case allowNewlines && (r == '\n' || r == '\r' || r == '\t'):
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			return true
		case (r >= 0x200b && r <= 0x200f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) || r == 0xfeff:
			return true
		}
	}
	return false
}

// normalizeProposalStepAdditions trims, NFC-normalizes and validates one step's additions (create/PATCH).
// Invalid, duplicate or missing doc ids are replaced by generated prop_ ids.
func normalizeProposalStepAdditions(stepIndex int, in *ProposalStepAdditions) (*ProposalStepAdditions, error) {
	if in == nil {
		return nil, nil
	}
	field := fmt.Sprintf("workflow_steps[%d].additions", stepIndex)
	note := norm.NFC.String(strings.TrimSpace(in.Note))
	if hasDisallowedRunes(note, true) {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, field+".note", "note contains invalid characters")
	}
	if utf8.RuneCountInString(note) > MaxProposalStepNoteLength {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, field+".note",
			fmt.Sprintf("note exceeds %d characters", MaxProposalStepNoteLength))
	}
	if len(in.Documents) > MaxProposalStepAdditionalDocuments {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, field+".documents",
			fmt.Sprintf("documents exceeds %d items", MaxProposalStepAdditionalDocuments))
	}
	out := &ProposalStepAdditions{Note: note}
	if len(in.Documents) == 0 {
		return out, nil
	}
	out.Documents = make([]ProposalStepAdditionalDocument, 0, len(in.Documents))
	names := make(map[string]struct{}, len(in.Documents))
	ids := make(map[string]struct{}, len(in.Documents))
	for j, doc := range in.Documents {
		docField := fmt.Sprintf("%s.documents[%d]", field, j)
		name := norm.NFC.String(strings.TrimSpace(doc.Name))
		if name == "" {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, docField+".name", "name is required")
		}
		if hasDisallowedRunes(name, false) {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, docField+".name", "name contains invalid characters")
		}
		if utf8.RuneCountInString(name) > MaxProposalStepDocumentNameLength {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, docField+".name",
				fmt.Sprintf("name exceeds %d characters", MaxProposalStepDocumentNameLength))
		}
		nameKey := strings.ToLower(name)
		if _, dup := names[nameKey]; dup {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, docField+".name", "duplicate document name in this step")
		}
		names[nameKey] = struct{}{}
		id := strings.TrimSpace(doc.DocID)
		if !proposalAdditionalDocIDPattern.MatchString(id) {
			id = ""
		}
		if _, dup := ids[strings.ToLower(id)]; id == "" || dup {
			id = newProposalAdditionalDocID()
		}
		ids[strings.ToLower(id)] = struct{}{}
		out.Documents = append(out.Documents, ProposalStepAdditionalDocument{DocID: id, Name: name})
	}
	return out, nil
}

// validateProposalAdditionsTotals enforces the whole-proposal caps on normalized steps.
func validateProposalAdditionsTotals(steps []ProposalWorkflowStep) error {
	noteRunes, docs := 0, 0
	for _, st := range steps {
		if st.Additions == nil {
			continue
		}
		noteRunes += utf8.RuneCountInString(st.Additions.Note)
		docs += len(st.Additions.Documents)
	}
	if noteRunes > MaxProposalAdditionsNoteTotal {
		return newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_steps",
			fmt.Sprintf("additions_total_exceeded: step notes exceed %d characters in total", MaxProposalAdditionsNoteTotal))
	}
	if docs > MaxProposalAdditionalDocumentsTotal {
		return newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_steps",
			fmt.Sprintf("additions_total_exceeded: more than %d additional documents in total", MaxProposalAdditionalDocumentsTotal))
	}
	return nil
}

// keepAdditionsFromOlderClients restores the saved additions of steps that an older client sent
// without `additions` (it does not know the field, so a whole-workflow PATCH would erase them).
// Additions-aware clients always send an object, so a nil input only comes from older clients.
// inputs[i] maps to snap.Steps[i]; prev is the stored workflow (same type).
func keepAdditionsFromOlderClients(inputs []ProposalWorkflowStepInput, snap, prev *ProposalWorkflowSnapshot) error {
	if snap == nil || prev == nil || len(inputs) != len(snap.Steps) {
		return nil
	}
	saved := make(map[string]*ProposalStepAdditions, len(prev.Steps))
	for _, st := range prev.Steps {
		if st.Additions != nil {
			saved[st.ID] = st.Additions
		}
	}
	if len(saved) == 0 {
		return nil
	}
	for i := range snap.Steps {
		if inputs[i].Additions != nil || strings.TrimSpace(inputs[i].ID) != snap.Steps[i].ID {
			continue
		}
		if add, ok := saved[snap.Steps[i].ID]; ok {
			snap.Steps[i].Additions = add
		}
	}
	return validateProposalAdditionsTotals(snap.Steps)
}

// ValidateStepAdditionsForSubmit requires a description (additions.note) on self-added steps
// (no source_step_id) saved by an additions-aware client. Steps without additions (older
// clients / drafts) are not checked, so older clients keep the previous submit behaviour.
func ValidateStepAdditionsForSubmit(snap *ProposalWorkflowSnapshot) error {
	if snap == nil {
		return nil
	}
	for i, st := range snap.Steps {
		if strings.TrimSpace(st.SourceStepID) != "" || st.Additions == nil {
			continue
		}
		if strings.TrimSpace(st.Additions.Note) == "" {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest,
				fmt.Sprintf("workflow_steps[%d].additions.note", i),
				"description_required: a description is required for steps added in the proposal")
		}
	}
	return nil
}
