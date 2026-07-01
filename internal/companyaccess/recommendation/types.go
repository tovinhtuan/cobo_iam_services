package recommendation

import "time"

const SourceConfigurationHealth = "configuration_health"

// CheckInput is minimal health-check input for the pure formatter (no I/O).
type CheckInput struct {
	Code        string
	Severity    string
	Domain      string
	Title       string
	Description string
	ActionLink  string
	Evidence    map[string]any
}

// Item is one rule recommendation (Batch 7A contract).
type Item struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	Source           string         `json:"source"`
	SourceCode       string         `json:"source_code"`
	Severity         string         `json:"severity"`
	Priority         string         `json:"priority"`
	Reason           string         `json:"reason"`
	ActionLabel      string         `json:"action_label"`
	ActionLink       string         `json:"action_link"`
	Evidence         map[string]any `json:"evidence"`
	GeneratedAt      time.Time      `json:"generated_at"`
	Category         string         `json:"category,omitempty"`
	AffectedResource map[string]any `json:"affected_resource,omitempty"`
	RelatedCheckID   string         `json:"related_check_id,omitempty"`
}
