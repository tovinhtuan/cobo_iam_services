package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DefaultWorkflowStepReminderDays is the BE runtime fallback when reminder_config is absent.
// CUSTOM replaces this list; never merge. Do not scatter this literal elsewhere.
var DefaultWorkflowStepReminderDays = []int{3, 1}

const (
	WorkflowStepReminderModeDaysBefore   = "days_before"
	WorkflowStepReminderModeSpecificDate = "specific_date"

	MaxWorkflowStepReminderOffset  = 90
	MaxWorkflowStepReminderOffsets = 8

	WorkflowStepReminderKindAbsent       = "absent"
	WorkflowStepReminderKindDefault      = "default"
	WorkflowStepReminderKindCustom       = "custom"
	WorkflowStepReminderKindDisabled     = "disabled"
	WorkflowStepReminderKindSpecificDate = "specific_date"
	WorkflowStepReminderKindInvalid      = "invalid"
)

// ReminderDaysNormalizeFailure names why custom days_before was rejected.
type ReminderDaysNormalizeFailure string

const (
	ReminderDaysNormalizeEmpty          ReminderDaysNormalizeFailure = "empty"
	ReminderDaysNormalizeNotInteger     ReminderDaysNormalizeFailure = "not_integer"
	ReminderDaysNormalizeNotPositive    ReminderDaysNormalizeFailure = "not_positive"
	ReminderDaysNormalizeOffsetTooLarge ReminderDaysNormalizeFailure = "offset_too_large"
	ReminderDaysNormalizeTooMany        ReminderDaysNormalizeFailure = "too_many_offsets"
)

// WorkflowStepReminderResolution is the canonical BE effective-rule result.
type WorkflowStepReminderResolution struct {
	Kind          string
	Configured    *WorkflowStepReminderConfig
	EffectiveDays []int
	NormalizeFail ReminderDaysNormalizeFailure
	Invalid       bool
}

func (r WorkflowStepReminderResolution) Error() error {
	if !r.Invalid {
		return nil
	}
	reason := string(r.NormalizeFail)
	if reason == "" {
		reason = "invalid"
	}
	return fmt.Errorf("invalid workflow step reminder_config: %s", reason)
}

// CloneWorkflowStepReminderConfig returns a deep copy, or nil.
func CloneWorkflowStepReminderConfig(cfg *WorkflowStepReminderConfig) *WorkflowStepReminderConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	if cfg.DaysBefore != nil {
		out.DaysBefore = append([]int(nil), cfg.DaysBefore...)
	}
	return &out
}

// NormalizeWorkflowStepReminderDays validates and canonicalizes custom offsets.
// Example: [1,3,3,7] → [7,3,1]. Invalid input is rejected (not defaulted).
func NormalizeWorkflowStepReminderDays(input []int) (days []int, fail ReminderDaysNormalizeFailure, ok bool) {
	if len(input) == 0 {
		return nil, ReminderDaysNormalizeEmpty, false
	}
	unique := make(map[int]struct{}, len(input))
	for _, item := range input {
		if item <= 0 {
			return nil, ReminderDaysNormalizeNotPositive, false
		}
		if item > MaxWorkflowStepReminderOffset {
			return nil, ReminderDaysNormalizeOffsetTooLarge, false
		}
		unique[item] = struct{}{}
	}
	if len(unique) == 0 {
		return nil, ReminderDaysNormalizeEmpty, false
	}
	if len(unique) > MaxWorkflowStepReminderOffsets {
		return nil, ReminderDaysNormalizeTooMany, false
	}
	out := make([]int, 0, len(unique))
	for day := range unique {
		out = append(out, day)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] > out[j] })
	return out, "", true
}

// ResolveWorkflowStepReminderRule is the BE runtime authority for effective reminder days.
//
//	absent            → DEFAULT [3,1]
//	enabled=false     → []
//	days_before valid → exact normalized custom
//	specific_date     → preserved path (no days_before reinterpret)
//	invalid custom    → Invalid=true, do not fall back to DEFAULT
func ResolveWorkflowStepReminderRule(cfg *WorkflowStepReminderConfig) WorkflowStepReminderResolution {
	if cfg == nil {
		return WorkflowStepReminderResolution{
			Kind:          WorkflowStepReminderKindDefault,
			EffectiveDays: append([]int(nil), DefaultWorkflowStepReminderDays...),
		}
	}
	cloned := CloneWorkflowStepReminderConfig(cfg)
	mode := strings.TrimSpace(strings.ToLower(cfg.Mode))
	if !cfg.Enabled {
		return WorkflowStepReminderResolution{
			Kind:          WorkflowStepReminderKindDisabled,
			Configured:    cloned,
			EffectiveDays: []int{},
		}
	}
	if mode == WorkflowStepReminderModeSpecificDate {
		return WorkflowStepReminderResolution{
			Kind:          WorkflowStepReminderKindSpecificDate,
			Configured:    cloned,
			EffectiveDays: []int{},
		}
	}
	// Missing mode with enabled=true and non-empty days_before → treat as days_before.
	days, fail, ok := NormalizeWorkflowStepReminderDays(cfg.DaysBefore)
	if !ok {
		return WorkflowStepReminderResolution{
			Kind:          WorkflowStepReminderKindInvalid,
			Configured:    cloned,
			EffectiveDays: nil,
			NormalizeFail: fail,
			Invalid:       true,
		}
	}
	return WorkflowStepReminderResolution{
		Kind:          WorkflowStepReminderKindCustom,
		Configured:    cloned,
		EffectiveDays: days,
	}
}

// ValidateWorkflowStepReminderConfigForPersist rejects invalid custom at API write time.
// nil (DEFAULT omit), enabled=false, and specific_date are accepted.
func ValidateWorkflowStepReminderConfigForPersist(cfg *WorkflowStepReminderConfig) error {
	if cfg == nil {
		return nil
	}
	res := ResolveWorkflowStepReminderRule(cfg)
	if res.Invalid {
		return res.Error()
	}
	return nil
}

type globalWorkflowStepDocumentsEnvelope struct {
	ReminderConfig *WorkflowStepReminderConfig `json:"reminder_config,omitempty"`
}

const legacyDocumentsJSONKey = "documents"

// EncodeGlobalWorkflowStepReminderDocumentsJSON stores reminder_config inside the existing
// global_workflow_steps.documents_json JSON column (no migration). DEFAULT omit → nil/NULL.
func EncodeGlobalWorkflowStepReminderDocumentsJSON(cfg *WorkflowStepReminderConfig) ([]byte, error) {
	return MergeGlobalWorkflowStepReminderDocumentsJSON(nil, cfg)
}

// MergeGlobalWorkflowStepReminderDocumentsJSON writes reminder_config without erasing
// leftover document payload in the same JSON column.
// DEFAULT omit (cfg==nil) removes reminder_config only; a legacy array is kept as-is.
func MergeGlobalWorkflowStepReminderDocumentsJSON(existing []byte, cfg *WorkflowStepReminderConfig) ([]byte, error) {
	existing = bytesTrimSpace(existing)
	if cfg == nil {
		return stripReminderConfigFromDocumentsJSON(existing)
	}
	if len(existing) == 0 || string(existing) == "null" {
		return json.Marshal(globalWorkflowStepDocumentsEnvelope{ReminderConfig: CloneWorkflowStepReminderConfig(cfg)})
	}
	if existing[0] == '[' {
		env := map[string]any{
			legacyDocumentsJSONKey: json.RawMessage(append([]byte(nil), existing...)),
			"reminder_config":      CloneWorkflowStepReminderConfig(cfg),
		}
		return json.Marshal(env)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(existing, &obj); err != nil {
		return json.Marshal(globalWorkflowStepDocumentsEnvelope{ReminderConfig: CloneWorkflowStepReminderConfig(cfg)})
	}
	rem, err := json.Marshal(CloneWorkflowStepReminderConfig(cfg))
	if err != nil {
		return nil, err
	}
	obj["reminder_config"] = rem
	return json.Marshal(obj)
}

func stripReminderConfigFromDocumentsJSON(existing []byte) ([]byte, error) {
	if len(existing) == 0 || string(existing) == "null" {
		return nil, nil
	}
	if existing[0] == '[' {
		return append([]byte(nil), existing...), nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(existing, &obj); err != nil {
		return append([]byte(nil), existing...), nil
	}
	delete(obj, "reminder_config")
	if len(obj) == 0 {
		return nil, nil
	}
	return json.Marshal(obj)
}

// DecodeGlobalWorkflowStepReminderDocumentsJSON reads reminder_config from documents_json.
// Legacy document arrays are ignored (reminder absent → DEFAULT at resolve time).
func DecodeGlobalWorkflowStepReminderDocumentsJSON(raw []byte) *WorkflowStepReminderConfig {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '[' {
		return nil
	}
	var envelope globalWorkflowStepDocumentsEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.ReminderConfig != nil {
		return CloneWorkflowStepReminderConfig(envelope.ReminderConfig)
	}
	var cfg WorkflowStepReminderConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	if !cfg.Enabled && strings.TrimSpace(cfg.Mode) == "" && len(cfg.DaysBefore) == 0 {
		return nil
	}
	return CloneWorkflowStepReminderConfig(&cfg)
}

func bytesTrimSpace(raw []byte) []byte {
	i, j := 0, len(raw)
	for i < j && (raw[i] == ' ' || raw[i] == '\n' || raw[i] == '\r' || raw[i] == '\t') {
		i++
	}
	for j > i && (raw[j-1] == ' ' || raw[j-1] == '\n' || raw[j-1] == '\r' || raw[j-1] == '\t') {
		j--
	}
	return raw[i:j]
}

var dueMinusEmbeddedInIDPattern = regexp.MustCompile(`due_minus_[1-9][0-9]*d`)

// DueMinusMilestoneType names an EndDate-minus-N reminder milestone (arbitrary offset).
func DueMinusMilestoneType(offsetDays int) MilestoneType {
	return MilestoneType(fmt.Sprintf("due_minus_%dd", offsetDays))
}

// RecoverDueMinusMilestoneTypeFromID extracts due_minus_Nd when buildMilestoneID embedded it
// and the DB column stored '' (invalid ENUM truncation). ok=false for unknown/legacy IDs.
func RecoverDueMinusMilestoneTypeFromID(milestoneID string) (string, bool) {
	found := dueMinusEmbeddedInIDPattern.FindString(strings.TrimSpace(milestoneID))
	if found == "" {
		return "", false
	}
	if _, ok := ParseDueMinusOffset(found); !ok {
		return "", false
	}
	return found, true
}

// ParseDueMinusOffset extracts N from due_minus_Nd. ok=false for other milestone types.
func ParseDueMinusOffset(milestoneType string) (int, bool) {
	const prefix = "due_minus_"
	const suffix = "d"
	s := strings.TrimSpace(milestoneType)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return 0, false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix)
	if mid == "" {
		return 0, false
	}
	n := 0
	for _, c := range mid {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return 0, false
	}
	return n, true
}

// IsDueMinusReminderMilestone reports whether the type is a configurable due-minus reminder.
func IsDueMinusReminderMilestone(milestoneType string) bool {
	_, ok := ParseDueMinusOffset(milestoneType)
	return ok
}
