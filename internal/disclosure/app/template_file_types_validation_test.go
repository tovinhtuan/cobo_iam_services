package app

import (
	"strings"
	"testing"
)

func TestValidateFreeTextFileTypes_acceptsCustomTypes(t *testing.T) {
	fieldErrors := map[string]string{}
	validateFreeTextFileTypes([]any{"PDF", "DOCX", "XLSX", "XML"}, "file_types", fieldErrors)
	if len(fieldErrors) != 0 {
		t.Fatalf("expected free-text file types to pass, got %v", fieldErrors)
	}
}

func TestValidateFreeTextFileTypes_rejectsEmptyAndTooLong(t *testing.T) {
	fieldErrors := map[string]string{}
	validateFreeTextFileTypes([]any{"PDF", "  "}, "file_types", fieldErrors)
	if fieldErrors["file_types"] == "" {
		t.Fatal("expected empty item rejection")
	}

	fieldErrors = map[string]string{}
	tooLong := strings.Repeat("A", maxChannelFileTypeLen+1)
	validateFreeTextFileTypes([]any{tooLong}, "file_types", fieldErrors)
	if fieldErrors["file_types"] == "" {
		t.Fatal("expected too-long rejection")
	}
}

func TestValidateChannelsAndFormatBusinessRules_acceptsDOCX(t *testing.T) {
	fieldErrors := map[string]string{}
	config := map[string]any{
		"channels": []any{
			map[string]any{
				"name":                   "Web",
				"disclosure_method":      "ELECTRONIC",
				"file_types":             []any{"PDF", "DOCX", "XLSX", "XML"},
				"attachment_requirement": "REQUIRED",
			},
		},
		"file_types": []any{"PDF", "DOCX", "XLSX", "XML"},
	}
	validateChannelsAndFormatBusinessRules("blocks.0", config, fieldErrors)
	if len(fieldErrors) != 0 {
		t.Fatalf("expected DOCX/XLSX free-text to pass, got %v", fieldErrors)
	}
}

func TestValidateOptionalDisclosureMethodLabels_acceptsCustomAndLegacyAbsent(t *testing.T) {
	fieldErrors := map[string]string{}
	config := map[string]any{
		"channels": []any{
			map[string]any{
				"name":                      "Web",
				"disclosure_methods":        []any{"EMAIL", "ONLINE"},
				"disclosure_method":         "ONLINE",
				"disclosure_method_labels":  []any{"Email", "Cổng thông tin điện tử", "Trực tuyến"},
				"file_types":                []any{"PDF"},
				"attachment_requirement":    "REQUIRED",
			},
			map[string]any{
				"name":                   "Legacy",
				"disclosure_method":      "EMAIL",
				"file_types":             []any{"PDF"},
				"attachment_requirement": "REQUIRED",
			},
		},
		"file_types": []any{"PDF"},
	}
	validateChannelsAndFormatBusinessRules("blocks.0", config, fieldErrors)
	if len(fieldErrors) != 0 {
		t.Fatalf("expected disclosure_method_labels additive payload to pass, got %v", fieldErrors)
	}
}

func TestValidateOptionalDisclosureMethodLabels_rejectsEmptyOrTooLong(t *testing.T) {
	fieldErrors := map[string]string{}
	validateOptionalDisclosureMethodLabels([]any{"Email", "  "}, "labels", fieldErrors)
	if fieldErrors["labels"] == "" {
		t.Fatal("expected empty label rejection")
	}
	fieldErrors = map[string]string{}
	tooLong := strings.Repeat("A", maxChannelFileTypeLen+1)
	validateOptionalDisclosureMethodLabels([]any{tooLong}, "labels", fieldErrors)
	if fieldErrors["labels"] == "" {
		t.Fatal("expected too-long rejection")
	}
}
