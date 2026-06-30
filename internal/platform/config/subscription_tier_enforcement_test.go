package config_test

import (
	"testing"

	"github.com/cobo/cobo_iam_services/internal/platform/config"
)

func TestParseSubscriptionTierEnforcementEnabled_DefaultFalse(t *testing.T) {
	if config.ParseSubscriptionTierEnforcementEnabled("") {
		t.Fatal("empty should be false")
	}
	if config.ParseSubscriptionTierEnforcementEnabled("false") {
		t.Fatal("false should be false")
	}
}

func TestParseSubscriptionTierEnforcementEnabled_TrueValues(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "TRUE"} {
		if !config.ParseSubscriptionTierEnforcementEnabled(v) {
			t.Fatalf("%q should be true", v)
		}
	}
}
