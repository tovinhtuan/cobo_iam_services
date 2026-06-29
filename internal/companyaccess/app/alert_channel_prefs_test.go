package app

import "testing"

func TestValidateAlertChannelPrefsPayload(t *testing.T) {
	validPayload := DefaultAlertChannelPrefsPayload("m_test")
	ok, issues := ValidateAlertChannelPrefsPayload(validPayload)
	if !ok {
		t.Fatalf("expected valid payload, issues=%v", issues)
	}

	bad := map[string]any{"version": 2}
	ok, issues = ValidateAlertChannelPrefsPayload(bad)
	if ok || len(issues) == 0 {
		t.Fatalf("expected invalid payload")
	}
}

func TestChannelsActiveFromPayload(t *testing.T) {
	payload := DefaultAlertChannelPrefsPayload("m_test")
	active := ChannelsActiveFromPayload(payload)
	if len(active) < 2 {
		t.Fatalf("expected email and in_app active, got %v", active)
	}
}

func TestPermissionRiskLevel(t *testing.T) {
	if PermissionRiskLevel("rbac.manage") != "critical" {
		t.Fatal("rbac.manage should be critical")
	}
	if PermissionRiskLevel("disclosure.view") != "low" {
		t.Fatal("disclosure.view should be low")
	}
}
