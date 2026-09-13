package workflowfulfillment

import (
	"net/http"
	"sort"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// MissingRequirement is one required snapshot lacking ACTIVE fulfillment.
type MissingRequirement struct {
	RequirementSnapshotID string `json:"requirement_snapshot_id"`
	SourceDocID           string `json:"source_doc_id"`
	Name                  string `json:"name"`
}

// EvaluateMissingRequiredDocuments returns required snapshots with ACTIVE count < 1.
// Zero snapshots → nil (legacy preserve). Optional requirements ignored.
// Order: ordinal ASC, then source_doc_id ASC, then id ASC.
// Template file refs never count; only activeCounts (ACTIVE lifecycle) do.
func EvaluateMissingRequiredDocuments(
	snaps []workflowapp.DocumentRequirementSnapshot,
	activeCounts map[string]int,
) []MissingRequirement {
	if len(snaps) == 0 {
		return nil
	}
	ordered := append([]workflowapp.DocumentRequirementSnapshot(nil), snaps...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Ordinal != ordered[j].Ordinal {
			return ordered[i].Ordinal < ordered[j].Ordinal
		}
		if ordered[i].SourceDocID != ordered[j].SourceDocID {
			return ordered[i].SourceDocID < ordered[j].SourceDocID
		}
		return ordered[i].ID < ordered[j].ID
	})
	missing := make([]MissingRequirement, 0)
	for _, snap := range ordered {
		if !snap.Required {
			continue
		}
		if activeCounts[snap.ID] >= 1 {
			continue
		}
		missing = append(missing, MissingRequirement{
			RequirementSnapshotID: snap.ID,
			SourceDocID:           snap.SourceDocID,
			Name:                  snap.Name,
		})
	}
	return missing
}

// NewRequiredDocumentMissingError builds the stable B3 Complete Step blocker.
func NewRequiredDocumentMissingError(missing []MissingRequirement) error {
	payload := make([]map[string]any, 0, len(missing))
	for _, m := range missing {
		payload = append(payload, map[string]any{
			"requirement_snapshot_id": m.RequirementSnapshotID,
			"source_doc_id":           m.SourceDocID,
			"name":                    m.Name,
		})
	}
	he := perr.NewHTTPError(
		http.StatusUnprocessableEntity,
		perr.CodeWorkflowStepRequiredDocumentMissing,
		"required document fulfillment missing for workflow step",
		nil,
	)
	he.Details = map[string]any{
		"missing_requirements": payload,
	}
	return he
}
