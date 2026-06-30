package conflict

// AlertChannelPrefsRuleCode mirrors app.AlertChannelPrefsRuleCode (avoid import cycle).
const AlertChannelPrefsRuleCode = "company.alert_channel_prefs.v1"

// NotificationRuleSnapshot is a read-only notification rule row for conflict loading.
type NotificationRuleSnapshot struct {
	RuleID  string
	Payload map[string]any
}

// ValidatorDeps wires app-layer validators without importing app from conflict.
type ValidatorDeps struct {
	ValidatePrefs         func(map[string]any) (bool, []string)
	PermissionRiskLevel   func(string) string
	IsGrantablePermission func(string) bool
}

var validators ValidatorDeps

// RegisterValidators sets validator callbacks (call once from app init).
func RegisterValidators(v ValidatorDeps) {
	validators = v
}
