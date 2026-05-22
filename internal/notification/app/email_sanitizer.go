package app

import (
	"encoding/json"
	"sort"
	"strings"
)

// SensitiveVarNames lists template variables whose plain values must never be
// persisted, logged, or echoed to dashboards. The notification service redacts
// these before insert; the renderer still sees the real values in memory so
// the email body itself is correct.
//
// Keep this list in sync with the variables declared sensitive in the SA at
// docs/email-notification-system-proposal.md §11.
var SensitiveVarNames = []string{
	"otp_code",
	"reset_link",
	"setup_link",
	"raw_token",
}

// RedactedVarPlaceholder is the value substituted for sensitive vars in the
// sanitised JSON. Using a constant string instead of a hash or length cue
// avoids leaking any signal about the secret.
const RedactedVarPlaceholder = "[REDACTED]"

// SanitizeVariables returns a copy of vars with every key listed in
// SensitiveVarNames (case-insensitive) replaced by RedactedVarPlaceholder.
// The input map is never mutated.
func SanitizeVariables(vars map[string]any) map[string]any {
	if vars == nil {
		return map[string]any{}
	}
	sensitive := sensitiveVarSet()
	out := make(map[string]any, len(vars))
	for k, v := range vars {
		if _, ok := sensitive[strings.ToLower(strings.TrimSpace(k))]; ok {
			out[k] = RedactedVarPlaceholder
			continue
		}
		out[k] = v
	}
	return out
}

// SanitizeVariablesToJSON returns a JSON string of SanitizeVariables output
// with deterministic key order, suitable for persistence in
// email_notifications.variables_json_sanitized.
func SanitizeVariablesToJSON(vars map[string]any) (string, error) {
	clean := SanitizeVariables(vars)
	keys := make([]string, 0, len(clean))
	for k := range clean {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make([]struct {
		Key   string
		Value any
	}, 0, len(keys))
	for _, k := range keys {
		ordered = append(ordered, struct {
			Key   string
			Value any
		}{Key: k, Value: clean[k]})
	}
	// json.Marshal of a map does NOT guarantee deterministic ordering across
	// Go versions in all paths; encode as a sorted key/value array to keep
	// fixtures stable. Callers that need a JSON object can use the helper
	// below.
	var sb strings.Builder
	sb.WriteByte('{')
	for i, kv := range ordered {
		if i > 0 {
			sb.WriteByte(',')
		}
		kb, err := json.Marshal(kv.Key)
		if err != nil {
			return "", err
		}
		sb.Write(kb)
		sb.WriteByte(':')
		vb, err := json.Marshal(kv.Value)
		if err != nil {
			return "", err
		}
		sb.Write(vb)
	}
	sb.WriteByte('}')
	return sb.String(), nil
}

func sensitiveVarSet() map[string]struct{} {
	m := make(map[string]struct{}, len(SensitiveVarNames))
	for _, name := range SensitiveVarNames {
		m[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	return m
}
