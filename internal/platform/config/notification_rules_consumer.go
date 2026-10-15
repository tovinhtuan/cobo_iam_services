package config

import "strings"

// ParseNotificationRulesConsumerEnabled reads NOTIFICATION_RULES_CONSUMER_ENABLED.
// True when value is 1, true, or yes (case-insensitive).
func ParseNotificationRulesConsumerEnabled(v string) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	return s == "1" || s == "true" || s == "yes"
}

// ParseNotificationRulesShadowEnabled reads NOTIFICATION_RULES_SHADOW_ENABLED (same truthy values).
// Shadow mode evaluates company alert prefs next to the real dispatch and records the difference;
// it never changes delivery and is ignored while the consumer is enabled.
func ParseNotificationRulesShadowEnabled(v string) bool {
	return ParseNotificationRulesConsumerEnabled(v)
}
