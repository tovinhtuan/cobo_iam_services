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
