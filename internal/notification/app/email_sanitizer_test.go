package app_test

import (
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

func TestSanitizeVariables_RedactsSensitive(t *testing.T) {
	vars := map[string]any{
		"full_name":      "Nguyen Van A",
		"otp_code":       "123456",
		"reset_link":     "https://app/reset?token=abc",
		"setup_link":     "https://app/setup?token=def",
		"raw_token":      "tok-xyz",
		"expiry_minutes": 15,
	}
	got := notificationapp.SanitizeVariables(vars)

	if got["full_name"] != "Nguyen Van A" {
		t.Errorf("non-sensitive var was modified: full_name = %v", got["full_name"])
	}
	if got["expiry_minutes"] != 15 {
		t.Errorf("non-sensitive var was modified: expiry_minutes = %v", got["expiry_minutes"])
	}
	for _, k := range []string{"otp_code", "reset_link", "setup_link", "raw_token"} {
		if got[k] != notificationapp.RedactedVarPlaceholder {
			t.Errorf("sensitive var %q not redacted, got %v", k, got[k])
		}
	}
}

func TestSanitizeVariables_DoesNotMutateInput(t *testing.T) {
	vars := map[string]any{"otp_code": "123456", "full_name": "A"}
	_ = notificationapp.SanitizeVariables(vars)
	if vars["otp_code"] != "123456" {
		t.Fatalf("input map mutated: otp_code = %v", vars["otp_code"])
	}
}

func TestSanitizeVariables_NilInput(t *testing.T) {
	got := notificationapp.SanitizeVariables(nil)
	if got == nil {
		t.Fatal("expected empty map, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestSanitizeVariables_CaseInsensitiveKey(t *testing.T) {
	got := notificationapp.SanitizeVariables(map[string]any{
		"OTP_Code":   "123",
		"Reset_Link": "x",
	})
	if got["OTP_Code"] != notificationapp.RedactedVarPlaceholder {
		t.Errorf("OTP_Code not redacted: %v", got["OTP_Code"])
	}
	if got["Reset_Link"] != notificationapp.RedactedVarPlaceholder {
		t.Errorf("Reset_Link not redacted: %v", got["Reset_Link"])
	}
}

func TestSanitizeVariablesToJSON_DeterministicOrder(t *testing.T) {
	vars := map[string]any{
		"zeta":      "z",
		"alpha":     "a",
		"otp_code":  "123456",
		"full_name": "Nguyen Van A",
	}
	out1, err := notificationapp.SanitizeVariablesToJSON(vars)
	if err != nil {
		t.Fatalf("SanitizeVariablesToJSON err = %v", err)
	}
	out2, err := notificationapp.SanitizeVariablesToJSON(vars)
	if err != nil {
		t.Fatalf("SanitizeVariablesToJSON err = %v", err)
	}
	if out1 != out2 {
		t.Fatalf("output not deterministic\nrun1: %s\nrun2: %s", out1, out2)
	}
	// Sorted ascending: alpha, full_name, otp_code, zeta
	want := `{"alpha":"a","full_name":"Nguyen Van A","otp_code":"[REDACTED]","zeta":"z"}`
	if out1 != want {
		t.Fatalf("unexpected JSON\nwant: %s\ngot:  %s", want, out1)
	}
	// Belt-and-suspenders: never leak the raw OTP in the JSON.
	if strings.Contains(out1, "123456") {
		t.Fatalf("raw OTP leaked into JSON: %s", out1)
	}
}
