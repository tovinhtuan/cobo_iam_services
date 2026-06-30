package config

import "strings"

// ParseSubscriptionTierEnforcementEnabled reads SUBSCRIPTION_TIER_ENFORCEMENT_ENABLED.
// True when value is 1, true, or yes (case-insensitive). Default false.
func ParseSubscriptionTierEnforcementEnabled(v string) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	return s == "1" || s == "true" || s == "yes"
}
