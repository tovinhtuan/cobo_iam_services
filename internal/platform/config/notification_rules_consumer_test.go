package config

import "testing"

func TestParseNotificationRulesConsumerEnabled(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"false", false},
		{"0", false},
		{"no", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{" yes ", true},
	}
	for _, tc := range tests {
		if got := ParseNotificationRulesConsumerEnabled(tc.in); got != tc.want {
			t.Fatalf("ParseNotificationRulesConsumerEnabled(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestLoad_NotificationRulesConsumerEnabledDefaultFalse(t *testing.T) {
	t.Setenv("NOTIFICATION_RULES_CONSUMER_ENABLED", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NotificationRulesConsumerEnabled {
		t.Fatal("NotificationRulesConsumerEnabled must default to false")
	}
}

func TestLoad_NotificationRulesConsumerEnabledTrue(t *testing.T) {
	t.Setenv("NOTIFICATION_RULES_CONSUMER_ENABLED", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.NotificationRulesConsumerEnabled {
		t.Fatal("NotificationRulesConsumerEnabled want true")
	}
}

// H7 T12: NOTIFICATION_RULES_SHADOW_ENABLED is read with the same truthy values, default false.
func TestLoad_NotificationRulesShadowEnabled(t *testing.T) {
	t.Setenv("NOTIFICATION_RULES_SHADOW_ENABLED", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NotificationRulesShadowEnabled {
		t.Fatal("shadow must default to false")
	}
	t.Setenv("NOTIFICATION_RULES_SHADOW_ENABLED", "true")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.NotificationRulesShadowEnabled {
		t.Fatal("NOTIFICATION_RULES_SHADOW_ENABLED=true must enable shadow")
	}
	if !ParseNotificationRulesShadowEnabled(" YES ") || ParseNotificationRulesShadowEnabled("no") {
		t.Fatal("shadow flag parsing must follow the consumer flag")
	}
}
