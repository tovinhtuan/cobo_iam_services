package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// NarrativeFieldKeys are template narrative JSON keys that the CMS editor does not
// author. When a PUT body omits them, Upsert keeps the candidate version's values
// (same omit-vs-clear semantics as legal_bases / applicable_to).
var NarrativeFieldKeys = []string{
	"applicability",
	"implementation_notes",
	"special_cases",
	"beneficiaries",
	"receiving_authorities",
}

// preserveOmittedFieldsFromCandidate copies omitted narrative fields from the current
// candidate (open draft, else active) so a full-replace save cannot silently erase data
// the client never loaded. New templates (CreateOnly) have nothing to preserve.
// "No candidate" (404/409) means there is nothing to keep; any other lookup error aborts
// the write rather than erasing the fields.
func (s *service) preserveOmittedFieldsFromCandidate(ctx context.Context, req *UpsertTypeVersionRequest) error {
	if req == nil || req.CreateOnly || len(req.NarrativeFieldsOmitted) == 0 {
		return nil
	}
	detail, _, err := s.templateWorkflowCandidateDetail(ctx, req.Subject, req.TypeID)
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && (he.HTTPStatus == http.StatusNotFound || he.HTTPStatus == http.StatusConflict) {
			return nil
		}
		return err
	}
	if detail == nil {
		return nil
	}
	for _, key := range req.NarrativeFieldsOmitted {
		switch key {
		case "applicability":
			req.Applicability = detail.Applicability
		case "implementation_notes":
			req.ImplementationNotes = detail.ImplementationNotes
		case "special_cases":
			req.SpecialCases = detail.SpecialCases
		case "beneficiaries":
			req.Beneficiaries = detail.Beneficiaries
		case "receiving_authorities":
			req.ReceivingAuthorities = detail.ReceivingAuthorities
		}
	}
	return nil
}

// rejectArchivedGlobalTemplateWrite blocks draft writes on an archived global template:
// the write would flip status back to 'active' and strand the archive metadata Restore needs.
func (s *service) rejectArchivedGlobalTemplateWrite(ctx context.Context, typeID string) error {
	life, err := s.lookupTypeLifecycle(ctx, typeID)
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
			return nil // new type_id: nothing archived to protect
		}
		return err
	}
	if life == nil {
		return nil
	}
	if strings.TrimSpace(life.CompanyID) == "" && strings.EqualFold(strings.TrimSpace(life.Status), "archived") {
		return errTemplateArchivedWrite(typeID)
	}
	return nil
}

func errTemplateArchivedWrite(typeID string) error {
	return &perr.HTTPError{
		HTTPStatus: http.StatusConflict,
		Code:       "TEMPLATE_ARCHIVED",
		Message:    "template is archived; restore it before editing",
		Details:    map[string]any{"type_id": typeID},
	}
}
