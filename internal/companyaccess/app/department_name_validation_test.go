package app

import (
	"net/http"
	"strings"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestValidateDepartmentName_acceptsPlainText(t *testing.T) {
	cases := []string{
		"Phòng Kế toán",
		"Phòng QA 日本語",
		"Team 🎯",
		"  Leading trimmed  ",
	}
	for _, raw := range cases {
		got, err := validateDepartmentName(raw)
		if err != nil {
			t.Fatalf("validateDepartmentName(%q) unexpected error: %v", raw, err)
		}
		if got != strings.TrimSpace(raw) {
			t.Fatalf("validateDepartmentName(%q) = %q, want trimmed %q", raw, got, strings.TrimSpace(raw))
		}
	}
}

func TestValidateDepartmentName_rejectsHTMLAndScriptPatterns(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"<script>alert(1)</script>",
		"<img src=x onerror=alert(1)>",
		"javascript:alert(1)",
		"Dept <b>bold</b>",
	}
	for _, raw := range cases {
		_, err := validateDepartmentName(raw)
		if err == nil {
			t.Fatalf("validateDepartmentName(%q) expected error", raw)
		}
		he, ok := perr.AsHTTPError(err)
		if !ok {
			t.Fatalf("validateDepartmentName(%q) want HTTPError, got %T", raw, err)
		}
		if he.HTTPStatus != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", he.HTTPStatus)
		}
		fieldErrors, ok := he.Details["field_errors"].(map[string]string)
		if !ok || fieldErrors["department_name"] == "" {
			t.Fatalf("expected field_errors.department_name, got %#v", he.Details)
		}
	}
}

func TestValidateDepartmentName_rejectsTooLong(t *testing.T) {
	long := strings.Repeat("a", maxDepartmentNameBytes+1)
	if _, err := validateDepartmentName(long); err == nil {
		t.Fatal("expected error for long name")
	}
}
