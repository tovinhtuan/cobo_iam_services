package app

import "strings"

const (
	WorkflowStepDescriptionFormatPlainText = "plain_text"
	WorkflowStepDescriptionFormatSafeHTML  = "safe_html"
)

// NormalizeWorkflowStepDescriptionFormat maps missing/legacy values to plain_text.
// Unknown non-empty values are NOT normalized here — reject on write via Validate*.
func NormalizeWorkflowStepDescriptionFormat(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" {
		return WorkflowStepDescriptionFormatPlainText
	}
	if v == WorkflowStepDescriptionFormatSafeHTML {
		return WorkflowStepDescriptionFormatSafeHTML
	}
	if v == WorkflowStepDescriptionFormatPlainText {
		return WorkflowStepDescriptionFormatPlainText
	}
	return WorkflowStepDescriptionFormatPlainText
}

// ValidateWorkflowStepDescriptionFormatForPersist accepts empty (→ plain) or known enums.
func ValidateWorkflowStepDescriptionFormatForPersist(raw string) error {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" || v == WorkflowStepDescriptionFormatPlainText || v == WorkflowStepDescriptionFormatSafeHTML {
		return nil
	}
	return errInvalidWorkflowStepDescriptionFormat(raw)
}

func errInvalidWorkflowStepDescriptionFormat(raw string) error {
	return &invalidDescriptionFormatError{raw: raw}
}

type invalidDescriptionFormatError struct{ raw string }

func (e *invalidDescriptionFormatError) Error() string {
	return "workflow step description_format must be plain_text or safe_html"
}
