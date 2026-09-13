package app

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Activation blocker / override error code for semantic-empty step description.
const ActivationBlockerWorkflowStepDescriptionRequired = "WORKFLOW_STEP_DESCRIPTION_REQUIRED"

var (
	htmlBreakTagRe   = regexp.MustCompile(`(?i)<\s*br\s*/?\s*>`)
	htmlCloseBlockRe = regexp.MustCompile(`(?i)</\s*(p|div|li|tr|h[1-6])\s*>`)
	htmlTagRe        = regexp.MustCompile(`(?i)<[^>]*>`)
	htmlNbspEntityRe = regexp.MustCompile(`(?i)&nbsp;|&#160;|&#x0*a0;`)
)

// StripHTMLToPlainText extracts readable text from HTML/safe_html for emptiness checks.
// Mirrors FE stripHtmlToPlainText semantics (tags → space/newline, common entities decoded).
func StripHTMLToPlainText(raw string) string {
	if raw == "" {
		return ""
	}
	withBreaks := htmlBreakTagRe.ReplaceAllString(raw, "\n")
	withBreaks = htmlCloseBlockRe.ReplaceAllString(withBreaks, "\n")
	noTags := htmlTagRe.ReplaceAllString(withBreaks, " ")
	noTags = htmlNbspEntityRe.ReplaceAllString(noTags, " ")
	// Decode remaining entities (&amp;, &lt;, numeric, …) after nbsp normalization.
	noTags = html.UnescapeString(noTags)
	return collapseWhitespaceKeepNewlines(noTags)
}

func collapseWhitespaceKeepNewlines(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == '\n' || r == '\r' {
			b.WriteRune('\n')
			prevSpace = false
			continue
		}
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

// WorkflowStepDescriptionMeaningfulPlain returns the plain textual content used to
// decide whether a workflow step description is semantically non-empty.
//
// Rules:
//   - plain_text (or missing format): TrimSpace on raw; also treat as empty when the
//     trimmed value has no letters/digits after HTML strip (covers visually empty markup
//     accidentally stored as plain_text).
//   - safe_html: strip tags/entities then TrimSpace.
func WorkflowStepDescriptionMeaningfulPlain(description, descriptionFormat string) string {
	raw := strings.TrimSpace(description)
	if raw == "" {
		return ""
	}
	format := NormalizeWorkflowStepDescriptionFormat(descriptionFormat)
	if format == WorkflowStepDescriptionFormatSafeHTML {
		return strings.TrimSpace(StripHTMLToPlainText(raw))
	}
	// plain_text: trim first; if content still looks like markup-only, strip.
	if looksLikeHTMLMarkup(raw) {
		return strings.TrimSpace(StripHTMLToPlainText(raw))
	}
	return raw
}

func looksLikeHTMLMarkup(s string) bool {
	return strings.Contains(s, "<") && strings.Contains(s, ">")
}

// IsWorkflowStepDescriptionSemanticEmpty reports whether description has no meaningful content.
// Shared by template Activate readiness and company override approve/apply.
// No minimum character length — any non-whitespace textual content is enough.
func IsWorkflowStepDescriptionSemanticEmpty(description, descriptionFormat string) bool {
	return WorkflowStepDescriptionMeaningfulPlain(description, descriptionFormat) == ""
}

// HasMeaningfulWorkflowStepDescription is the inverse of IsWorkflowStepDescriptionSemanticEmpty.
func HasMeaningfulWorkflowStepDescription(description, descriptionFormat string) bool {
	return !IsWorkflowStepDescriptionSemanticEmpty(description, descriptionFormat)
}

func workflowStepDescriptionBlockerMessage(stage string, index int) string {
	label := strings.TrimSpace(stage)
	if label == "" {
		label = formatWorkflowStepFallbackLabel(index)
	}
	return "Bước '" + label + "': chưa có mô tả. Vui lòng bổ sung mô tả trước khi đăng lên Portal."
}

func formatWorkflowStepFallbackLabel(index int) string {
	if index < 0 {
		return "Bước"
	}
	return "Bước " + strconv.Itoa(index+1)
}

// CollectWorkflowStepDescriptionActivationBlockers returns one blocker per semantic-empty step.
func CollectWorkflowStepDescriptionActivationBlockers(steps []WorkflowStepDTO) []ActivationBlockerDTO {
	out := make([]ActivationBlockerDTO, 0)
	for i, step := range steps {
		if !IsWorkflowStepDescriptionSemanticEmpty(step.Description, step.DescriptionFormat) {
			continue
		}
		stepID := strings.TrimSpace(step.StepID)
		out = append(out, ActivationBlockerDTO{
			Code:    ActivationBlockerWorkflowStepDescriptionRequired,
			Message: workflowStepDescriptionBlockerMessage(step.Stage, i),
			StepKey: stepID,
			StepID:  stepID,
		})
	}
	return out
}
