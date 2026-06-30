package config

import "strings"

// ParseNotificationRulesConsumerEnabled reads NOTIFICATION_RULES_CONSUMER_ENABLED.
// True when value is 1, true, or yes (case-insensitive).
func ParseNotificationRulesConsumerEnabled(v string) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	return s == "1" || s == "true" || s == "yes"
}
