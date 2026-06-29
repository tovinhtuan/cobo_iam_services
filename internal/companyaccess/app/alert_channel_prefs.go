package app

import (
	"strconv"
	"strings"
	"time"
)

// AlertChannelPrefsRuleCode is the canonical notification_rules.rule_code for enterprise alert prefs.
const AlertChannelPrefsRuleCode = "company.alert_channel_prefs.v1"

// NotificationRuleStatusView is returned by GET /api/v1/admin/notification-rules/status.
type NotificationRuleStatusView struct {
	RuleCode                     string   `json:"rule_code"`
	StorageConfigured            bool     `json:"storage_configured"`
	PayloadValid                 bool     `json:"payload_valid"`
	PreviewAvailable             bool     `json:"preview_available"`
	RuntimeConsumerEnabled       bool     `json:"runtime_consumer_enabled"`
	DispatchEnforcementEnabled   bool     `json:"dispatch_enforcement_enabled"`
	SubscriptionTierEnforced     bool     `json:"subscription_tier_enforced"`
	ChannelsActive               []string `json:"channels_active"`
	LastUpdatedAt                string   `json:"last_updated_at,omitempty"`
	LastUpdatedBy                string   `json:"last_updated_by,omitempty"`
	UIState                      string   `json:"ui_state"`
	Warnings                     []string `json:"warnings"`
}

type GetNotificationRuleStatusRequest struct {
	Subject AdminSubject
}

// DefaultAlertChannelPrefsPayload returns the bootstrap payload for alert channel prefs.
func DefaultAlertChannelPrefsPayload(membershipID string) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	return map[string]any{
		"version": 1,
		"event_scope": []any{
			"deadline",
			"workflow",
		},
		"channels": map[string]any{
			"email":  map[string]any{"enabled": true},
			"in_app": map[string]any{"enabled": true},
			"zalo":   map[string]any{"enabled": false},
			"sms":    map[string]any{"enabled": false},
		},
		"schedules": []any{
			map[string]any{"offset_days": 30, "enabled": true, "premium_only": false},
			map[string]any{"offset_days": 7, "enabled": true, "premium_only": false},
			map[string]any{"offset_days": 3, "enabled": true, "premium_only": false},
			map[string]any{"offset_days": 0, "enabled": true, "premium_only": false},
			map[string]any{"kind": "escalation", "enabled": false, "premium_only": true},
		},
		"recipient_policies": []any{
			"department_focal",
			"assignee",
			"company_admin",
		},
		"updated_by": membershipID,
		"updated_at": now,
	}
}

// ValidateAlertChannelPrefsPayload checks the alert channel prefs document shape.
func ValidateAlertChannelPrefsPayload(payload map[string]any) (valid bool, issues []string) {
	if payload == nil {
		return false, []string{"payload is required"}
	}
	if v, ok := payload["version"]; ok {
		switch n := v.(type) {
		case float64:
			if int(n) != 1 {
				issues = append(issues, "version must be 1")
			}
		case int:
			if n != 1 {
				issues = append(issues, "version must be 1")
			}
		default:
			issues = append(issues, "version must be a number")
		}
	}
	channels, ok := payload["channels"].(map[string]any)
	if !ok || len(channels) == 0 {
		issues = append(issues, "channels object is required")
	} else {
		for _, key := range []string{"email", "in_app", "zalo", "sms"} {
			raw, exists := channels[key]
			if !exists {
				continue
			}
			chMap, ok := raw.(map[string]any)
			if !ok {
				issues = append(issues, "channels."+key+" must be an object")
				continue
			}
			if _, ok := chMap["enabled"]; !ok {
				issues = append(issues, "channels."+key+".enabled is required")
			}
		}
	}
	if schedules, ok := payload["schedules"].([]any); ok {
		for i, item := range schedules {
			m, ok := item.(map[string]any)
			if !ok {
				issues = append(issues, "schedules["+strconv.Itoa(i)+"] must be an object")
			} else if _, ok := m["enabled"]; !ok {
				issues = append(issues, "schedules["+strconv.Itoa(i)+"].enabled is required")
			}
		}
	}
	return len(issues) == 0, issues
}

// ChannelsActiveFromPayload returns enabled channel keys from payload.
func ChannelsActiveFromPayload(payload map[string]any) []string {
	channels, ok := payload["channels"].(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, 4)
	for _, key := range []string{"email", "in_app", "zalo", "sms"} {
		raw, ok := channels[key].(map[string]any)
		if !ok {
			continue
		}
		enabled, _ := raw["enabled"].(bool)
		if enabled {
			out = append(out, key)
		}
	}
	return out
}

func stringFromPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
