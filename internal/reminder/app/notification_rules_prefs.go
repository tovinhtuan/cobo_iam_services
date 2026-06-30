package app

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseAlertChannelPrefsDocument maps storage payload into a typed document.
func ParseAlertChannelPrefsDocument(companyID, ruleCode, status string, payload map[string]any) (*AlertChannelPrefsDocument, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	if payload == nil {
		return nil, fmt.Errorf("payload is required")
	}
	valid, issues := validateAlertChannelPrefsPayload(payload)
	if !valid {
		return nil, fmt.Errorf("invalid prefs: %s", strings.Join(issues, "; "))
	}
	doc := &AlertChannelPrefsDocument{
		RuleCode:          strings.TrimSpace(ruleCode),
		CompanyID:         companyID,
		Status:            strings.TrimSpace(status),
		Channels:          map[string]ChannelPref{},
		RecipientPolicies: []string{},
		RawPayload:        payload,
	}
	if v, ok := payload["version"]; ok {
		switch n := v.(type) {
		case float64:
			doc.Version = int(n)
		case int:
			doc.Version = n
		}
	}
	doc.EventScope = stringSliceFromAny(payload["event_scope"])
	channels, _ := payload["channels"].(map[string]any)
	for _, key := range channelKeys() {
		raw, ok := channels[key].(map[string]any)
		if !ok {
			continue
		}
		enabled, _ := raw["enabled"].(bool)
		doc.Channels[key] = ChannelPref{Enabled: enabled}
	}
	if schedules, ok := payload["schedules"].([]any); ok {
		for _, item := range schedules {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			sp := SchedulePref{
				Kind:        stringFromAny(m["kind"]),
				Enabled:     boolFromAny(m["enabled"]),
				PremiumOnly: boolFromAny(m["premium_only"]),
			}
			if od, ok := m["offset_days"]; ok {
				switch n := od.(type) {
				case float64:
					v := int(n)
					sp.OffsetDays = &v
				case int:
					v := n
					sp.OffsetDays = &v
				}
			}
			doc.Schedules = append(doc.Schedules, sp)
		}
	}
	doc.RecipientPolicies = stringSliceFromAny(payload["recipient_policies"])
	doc.UpdatedBy = stringFromAny(payload["updated_by"])
	doc.UpdatedAt = stringFromAny(payload["updated_at"])
	return doc, nil
}

func channelKeys() []string {
	return []string{"email", "in_app", "zalo", "sms"}
}

func validateAlertChannelPrefsPayload(payload map[string]any) (valid bool, issues []string) {
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
		for _, key := range channelKeys() {
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

func activeChannelsFromDoc(doc *AlertChannelPrefsDocument) []string {
	if doc == nil {
		return nil
	}
	out := make([]string, 0, len(doc.Channels))
	for _, key := range channelKeys() {
		if ch, ok := doc.Channels[key]; ok && ch.Enabled {
			out = append(out, key)
		}
	}
	return out
}

func disabledChannelsFromDoc(doc *AlertChannelPrefsDocument) []string {
	if doc == nil {
		return nil
	}
	out := make([]string, 0, len(doc.Channels))
	for _, key := range channelKeys() {
		if ch, ok := doc.Channels[key]; ok && !ch.Enabled {
			out = append(out, key)
		}
	}
	return out
}

func stringSliceFromAny(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return strings.TrimSpace(s)
}

func boolFromAny(v any) bool {
	b, ok := v.(bool)
	if ok {
		return b
	}
	return false
}

func eventScopeAllows(doc *AlertChannelPrefsDocument, eventType string) bool {
	if doc == nil || len(doc.EventScope) == 0 {
		return true
	}
	eventType = strings.TrimSpace(strings.ToLower(eventType))
	for _, scope := range doc.EventScope {
		if strings.EqualFold(strings.TrimSpace(scope), eventType) {
			return true
		}
	}
	return false
}
