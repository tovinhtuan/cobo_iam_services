package app

import (
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// MaxTemplateDescriptionLength is the CMS template summary (Mô tả) UTF-8 byte limit enforced on FE+BE.
// DEV MySQL upsert fails above ~3320 bytes on disclosure_type_versions.description (ASCII probe).
const MaxTemplateDescriptionLength = 3200

func templateDescriptionByteLength(description string) int {
	return len([]byte(strings.TrimSpace(description)))
}

func validateTemplateDescription(description string) error {
	if templateDescriptionByteLength(description) <= MaxTemplateDescriptionLength {
		return nil
	}
	return newValidationError(map[string]string{
		"description": fmt.Sprintf("description must be at most %d UTF-8 bytes", MaxTemplateDescriptionLength),
	})
}

func validateTemplateBlockDescriptionLengths(blocks []TemplateBlockDTO, fieldErrors map[string]string) {
	for idx := range blocks {
		block := &blocks[idx]
		maxLen, ok := blockConfigMaxLength(block.Config)
		if !ok {
			continue
		}
		descBytes := len([]byte(block.Description))
		if descBytes > maxLen {
			prefix := "blocks." + fmt.Sprint(idx)
			fieldErrors[prefix+".description"] = fmt.Sprintf("description must be at most %d UTF-8 bytes", maxLen)
		}
	}
}

func blockConfigMaxLength(config map[string]any) (int, bool) {
	if config == nil {
		return 0, false
	}
	raw, ok := config["max_length"]
	if !ok || raw == nil {
		return 0, false
	}
	n, ok := positiveIntegerFromAny(raw)
	if !ok {
		return 0, false
	}
	return int(n), true
}

func mapRepositoryUpsertError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "data too long") || strings.Contains(msg, "1406") {
		return &perr.HTTPError{
			Code:       perr.CodeInvalidRequest,
			Message:    "template field exceeds maximum allowed length",
			HTTPStatus: http.StatusBadRequest,
			Details: map[string]any{
				"field_errors": map[string]string{
					"description": fmt.Sprintf("description must be at most %d UTF-8 bytes", MaxTemplateDescriptionLength),
				},
			},
		}
	}
	return err
}
