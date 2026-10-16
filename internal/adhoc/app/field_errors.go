package app

import (
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func newAdHocFieldError(httpStatus int, code perr.Code, field, message string) error {
	he := perr.NewHTTPError(httpStatus, code, message, nil)
	he.Details = map[string]any{"field": field}
	return he
}

func newAdHocPermissionError(permission, message string) error {
	he := perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, message, nil)
	he.Details = map[string]any{"permission": permission}
	return he
}

// ReasonAllReviewersApproved is details.reason of the 409 returned by a reject or cancel that comes
// after every assigned reviewer approved (v1.2 §3.2).
const ReasonAllReviewersApproved = "all_reviewers_approved"

// AllReviewersApprovedError is that 409.
func AllReviewersApprovedError() error {
	he := perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "every reviewer has approved; the proposal can no longer be rejected or cancelled", nil)
	he.Details = map[string]any{"reason": ReasonAllReviewersApproved}
	return he
}

// ReasonFinalizeInProgress is details.reason of the 409 returned when another request is finalizing
// the proposal.
const ReasonFinalizeInProgress = "finalize_in_progress"

// FinalizeInProgressError is that 409.
func FinalizeInProgressError() error {
	he := perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "the proposal is being finalized; retry shortly", nil)
	he.Details = map[string]any{"reason": ReasonFinalizeInProgress}
	return he
}

// ReasonFinalValuesLocked is details.reason of the 409 returned when a vote tries to change the final
// values after finalize created the record from them.
const ReasonFinalValuesLocked = "final_values_locked"

// FinalValuesLockedError is that 409.
func FinalValuesLockedError() error {
	he := perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "finalize already started from the current final values; they can no longer change", nil)
	he.Details = map[string]any{"reason": ReasonFinalValuesLocked}
	return he
}
