package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ApplicableFrom authoring modes (Template Version deadline_config_json).
const (
	ApplicableFromModeCurrent  = "CURRENT_SLOT"
	ApplicableFromModeNext     = "NEXT_SLOT"
	ApplicableFromModeSpecific = "SPECIFIC_SLOT"
)

func afErr(status int, message string, details map[string]any) error {
	return &perr.HTTPError{
		Code:       perr.CodeInvalidRequest,
		Message:    message,
		HTTPStatus: status,
		Details:    details,
	}
}

// NormalizeApplicableFromMode maps wire values; empty = legacy absent.
func NormalizeApplicableFromMode(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case ApplicableFromModeCurrent, "CURRENT":
		return ApplicableFromModeCurrent
	case ApplicableFromModeNext, "NEXT":
		return ApplicableFromModeNext
	case ApplicableFromModeSpecific, "SPECIFIC":
		return ApplicableFromModeSpecific
	default:
		return ""
	}
}

// IsLegacyApplicableFrom reports pre-V2 absent config (not NEXT_SLOT).
func IsLegacyApplicableFrom(mode, slot string) bool {
	return NormalizeApplicableFromMode(mode) == "" && strings.TrimSpace(slot) == ""
}

// ValidateLogicalSlot checks frequency-native cycle_label representation (strict canonical).
func ValidateLogicalSlot(frequencyUnit, raw string) error {
	canon, err := NormalizeLogicalSlot(frequencyUnit, raw)
	if err != nil {
		return err
	}
	if strings.TrimSpace(raw) != canon {
		return afErr(http.StatusBadRequest, "applicable_from_slot must be canonical for frequency", map[string]any{
			"slot": raw, "normalized": canon, "code": "APPLICABLE_FROM_SLOT_INVALID_FOR_FREQUENCY",
		})
	}
	return nil
}

// NormalizeLogicalSlot canonicalizes authoring input to cycle_label.
// Weekly: any date in week → Sunday key. Monthly: pad YYYY-MM.
func NormalizeLogicalSlot(frequencyUnit, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", afErr(http.StatusBadRequest, "applicable_from_slot is required", map[string]any{"code": "APPLICABLE_FROM_SPECIFIC_SLOT_REQUIRED"})
	}
	loc := asiaHoChiMinh()
	switch NormalizeFrequencyUnit(frequencyUnit) {
	case PeriodicityDaily:
		t, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			return "", afErr(http.StatusBadRequest, "invalid daily applicable_from_slot", map[string]any{"slot": raw})
		}
		return t.Format("2006-01-02"), nil
	case PeriodicityWeekly:
		t, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			return "", afErr(http.StatusBadRequest, "invalid weekly applicable_from_slot", map[string]any{"slot": raw})
		}
		return weekStartSunday(t).Format("2006-01-02"), nil
	case PeriodicityMonthly:
		var y, m int
		if _, err := fmt.Sscanf(raw, "%d-%d", &y, &m); err != nil || m < 1 || m > 12 {
			return "", afErr(http.StatusBadRequest, "invalid monthly applicable_from_slot", map[string]any{"slot": raw})
		}
		return fmt.Sprintf("%04d-%02d", y, m), nil
	case PeriodicityQuarterly:
		var y, q int
		if _, err := fmt.Sscanf(raw, "%d-Q%d", &y, &q); err != nil || q < 1 || q > 4 {
			return "", afErr(http.StatusBadRequest, "invalid quarterly applicable_from_slot", map[string]any{"slot": raw})
		}
		return fmt.Sprintf("%d-Q%d", y, q), nil
	case PeriodicityYearly:
		var y int
		if _, err := fmt.Sscanf(raw, "%d", &y); err != nil || y < 1 {
			return "", afErr(http.StatusBadRequest, "invalid yearly applicable_from_slot", map[string]any{"slot": raw})
		}
		return fmt.Sprintf("%d", y), nil
	default:
		return "", afErr(http.StatusBadRequest, "frequency_unit required for applicable_from_slot", nil)
	}
}

// NextLogicalSlot returns the immediate next cycle_label after slot.
func NextLogicalSlot(frequencyUnit, slot string) (string, error) {
	canon, err := NormalizeLogicalSlot(frequencyUnit, slot)
	if err != nil {
		return "", err
	}
	loc := asiaHoChiMinh()
	switch NormalizeFrequencyUnit(frequencyUnit) {
	case PeriodicityDaily:
		t, _ := time.ParseInLocation("2006-01-02", canon, loc)
		return t.AddDate(0, 0, 1).Format("2006-01-02"), nil
	case PeriodicityWeekly:
		t, _ := time.ParseInLocation("2006-01-02", canon, loc)
		return weekStartSunday(t).AddDate(0, 0, 7).Format("2006-01-02"), nil
	case PeriodicityMonthly:
		var y, m int
		_, _ = fmt.Sscanf(canon, "%d-%d", &y, &m)
		t := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0)
		return t.Format("2006-01"), nil
	case PeriodicityQuarterly:
		var y, q int
		_, _ = fmt.Sscanf(canon, "%d-Q%d", &y, &q)
		q++
		if q > 4 {
			q = 1
			y++
		}
		return fmt.Sprintf("%d-Q%d", y, q), nil
	case PeriodicityYearly:
		var y int
		_, _ = fmt.Sscanf(canon, "%d", &y)
		return fmt.Sprintf("%d", y+1), nil
	default:
		return "", afErr(http.StatusBadRequest, "unsupported frequency for next slot", nil)
	}
}

// CompareLogicalSlots returns -1, 0, 1 for a vs b under frequency (chronological).
func CompareLogicalSlots(frequencyUnit, a, b string) (int, error) {
	ca, err := NormalizeLogicalSlot(frequencyUnit, a)
	if err != nil {
		return 0, err
	}
	cb, err := NormalizeLogicalSlot(frequencyUnit, b)
	if err != nil {
		return 0, err
	}
	startA, err := slotStartDate(frequencyUnit, ca)
	if err != nil {
		return 0, err
	}
	startB, err := slotStartDate(frequencyUnit, cb)
	if err != nil {
		return 0, err
	}
	if startA.Before(startB) {
		return -1, nil
	}
	if startA.After(startB) {
		return 1, nil
	}
	return 0, nil
}

// ApplicableFromDecision codes for worker observability (not public API).
const (
	ApplicableFromDecisionLegacyAllow   = "legacy_allow"
	ApplicableFromDecisionEligible      = "eligible"
	ApplicableFromDecisionSkipBefore    = "skip_before_applicable_from"
	ApplicableFromDecisionInvalidSlot   = "applicable_from_invalid"
	ApplicableFromDecisionUnfrozenMode  = "applicable_from_unfrozen_relative"
)

// EvaluateApplicableFromEligibility is the Phase 5 lower-bound filter.
// Legacy NULL/empty → allow existing flow. Worker never resolves CURRENT/NEXT dynamically.
// Invalid non-null boundary or relative mode without frozen slot → not eligible (fail closed).
func EvaluateApplicableFromEligibility(frequencyUnit, candidateSlot, mode, boundarySlot string) (eligible bool, decision string, err error) {
	mode = NormalizeApplicableFromMode(mode)
	boundary := strings.TrimSpace(boundarySlot)
	if IsLegacyApplicableFrom(mode, boundary) {
		return true, ApplicableFromDecisionLegacyAllow, nil
	}
	if boundary == "" {
		// V2 relative authoring without freeze — must not dynamic-resolve at worker.
		return false, ApplicableFromDecisionUnfrozenMode, afErr(http.StatusUnprocessableEntity,
			"active applicable_from_mode without frozen applicable_from_slot",
			map[string]any{"code": ApplicableFromDecisionUnfrozenMode, "mode": mode})
	}
	freq := NormalizeFrequencyUnit(frequencyUnit)
	if _, err := NormalizeLogicalSlot(freq, boundary); err != nil {
		return false, ApplicableFromDecisionInvalidSlot, err
	}
	cmp, err := CompareLogicalSlots(freq, candidateSlot, boundary)
	if err != nil {
		return false, ApplicableFromDecisionInvalidSlot, err
	}
	if cmp < 0 {
		return false, ApplicableFromDecisionSkipBefore, nil
	}
	return true, ApplicableFromDecisionEligible, nil
}

func slotStartDate(frequencyUnit, slot string) (time.Time, error) {
	loc := asiaHoChiMinh()
	switch NormalizeFrequencyUnit(frequencyUnit) {
	case PeriodicityDaily, PeriodicityWeekly:
		return time.ParseInLocation("2006-01-02", slot, loc)
	case PeriodicityMonthly:
		var y, m int
		_, err := fmt.Sscanf(slot, "%d-%d", &y, &m)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(y, time.Month(m), 1, 0, 0, 0, 0, loc), nil
	case PeriodicityQuarterly:
		var y, q int
		_, err := fmt.Sscanf(slot, "%d-Q%d", &y, &q)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(y, time.Month((q-1)*3+1), 1, 0, 0, 0, 0, loc), nil
	case PeriodicityYearly:
		var y int
		_, err := fmt.Sscanf(slot, "%d", &y)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(y, 1, 1, 0, 0, 0, 0, loc), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported frequency")
	}
}

// PrepareApplicableFromForDraftWrite normalizes authoring fields for Upsert/Save Draft.
// Does NOT freeze CURRENT/NEXT. Legacy untouched stays empty.
// Same-root safety: relative mode + non-empty slot → SPECIFIC_SLOT (preserve boundary).
func PrepareApplicableFromForDraftWrite(cfg *TemplateDeadlineConfig, priorFreq string) error {
	if cfg == nil {
		return nil
	}
	mode := NormalizeApplicableFromMode(cfg.ApplicableFromMode)
	slot := strings.TrimSpace(cfg.ApplicableFromSlot)
	freq := NormalizeFrequencyUnit(cfg.FrequencyUnit)

	if priorFreq != "" && freq != "" && NormalizeFrequencyUnit(priorFreq) != freq && slot != "" {
		cfg.ApplicableFromMode = ApplicableFromModeNext
		cfg.ApplicableFromSlot = ""
		return nil
	}

	if mode == "" && slot == "" {
		cfg.ApplicableFromMode = ""
		cfg.ApplicableFromSlot = ""
		return nil
	}

	if mode == "" && slot != "" {
		mode = ApplicableFromModeSpecific
	}

	switch mode {
	case ApplicableFromModeCurrent, ApplicableFromModeNext:
		if slot != "" {
			norm, err := NormalizeLogicalSlot(freq, slot)
			if err != nil {
				return err
			}
			cfg.ApplicableFromMode = ApplicableFromModeSpecific
			cfg.ApplicableFromSlot = norm
			return nil
		}
		cfg.ApplicableFromMode = mode
		cfg.ApplicableFromSlot = ""
		return nil
	case ApplicableFromModeSpecific:
		if slot == "" {
			cfg.ApplicableFromMode = ApplicableFromModeSpecific
			cfg.ApplicableFromSlot = ""
			return nil
		}
		norm, err := NormalizeLogicalSlot(freq, slot)
		if err != nil {
			return err
		}
		cfg.ApplicableFromMode = ApplicableFromModeSpecific
		cfg.ApplicableFromSlot = norm
		return nil
	default:
		return afErr(http.StatusBadRequest, "invalid applicable_from_mode", map[string]any{"mode": cfg.ApplicableFromMode, "code": "APPLICABLE_FROM_MODE_INVALID"})
	}
}

// ValidateApplicableFromForActivate enforces activation rules for V2-authored configs.
func ValidateApplicableFromForActivate(cfg *TemplateDeadlineConfig) error {
	if cfg == nil || IsLegacyApplicableFrom(cfg.ApplicableFromMode, cfg.ApplicableFromSlot) {
		return nil
	}
	freq := NormalizeFrequencyUnit(cfg.FrequencyUnit)
	if !IsPeriodicFrequencyUnit(freq) {
		return nil
	}
	mode := NormalizeApplicableFromMode(cfg.ApplicableFromMode)
	slot := strings.TrimSpace(cfg.ApplicableFromSlot)
	switch mode {
	case ApplicableFromModeCurrent, ApplicableFromModeNext:
		return nil
	case ApplicableFromModeSpecific:
		if slot == "" {
			return afErr(http.StatusUnprocessableEntity, "applicable_from_slot is required for SPECIFIC_SLOT", map[string]any{"code": "APPLICABLE_FROM_SPECIFIC_SLOT_REQUIRED"})
		}
		if _, err := NormalizeLogicalSlot(freq, slot); err != nil {
			return err
		}
		return nil
	default:
		return afErr(http.StatusUnprocessableEntity, "invalid applicable_from_mode", map[string]any{"code": "APPLICABLE_FROM_MODE_INVALID"})
	}
}

// FreezeApplicableFromAtActivate resolves concrete cycle_label once.
// If slot already frozen, returns it unchanged (activation retry stability).
func FreezeApplicableFromAtActivate(cfg *TemplateDeadlineConfig, activateAt time.Time) (mode string, frozenSlot string, err error) {
	if cfg == nil || IsLegacyApplicableFrom(cfg.ApplicableFromMode, cfg.ApplicableFromSlot) {
		return "", "", nil
	}
	freq := NormalizeFrequencyUnit(cfg.FrequencyUnit)
	if !IsPeriodicFrequencyUnit(freq) {
		return NormalizeApplicableFromMode(cfg.ApplicableFromMode), strings.TrimSpace(cfg.ApplicableFromSlot), nil
	}
	mode = NormalizeApplicableFromMode(cfg.ApplicableFromMode)
	existing := strings.TrimSpace(cfg.ApplicableFromSlot)
	if existing != "" {
		norm, nerr := NormalizeLogicalSlot(freq, existing)
		if nerr != nil {
			return "", "", nerr
		}
		return mode, norm, nil
	}
	loc := asiaHoChiMinh()
	now := activateAt.In(loc)
	switch mode {
	case ApplicableFromModeCurrent:
		return mode, ResolveLogicalSlot(freq, now, loc), nil
	case ApplicableFromModeNext:
		cur := ResolveLogicalSlot(freq, now, loc)
		next, nerr := NextLogicalSlot(freq, cur)
		if nerr != nil {
			return "", "", nerr
		}
		return mode, next, nil
	case ApplicableFromModeSpecific:
		return "", "", afErr(http.StatusUnprocessableEntity, "applicable_from_slot is required for SPECIFIC_SLOT", map[string]any{"code": "APPLICABLE_FROM_SPECIFIC_SLOT_REQUIRED"})
	default:
		return "", "", afErr(http.StatusUnprocessableEntity, "invalid applicable_from_mode", map[string]any{"code": "APPLICABLE_FROM_MODE_INVALID"})
	}
}

// ApplyCloneApplicableFromDefaults resets applicability for a new template root.
func ApplyCloneApplicableFromDefaults(cfg *TemplateDeadlineConfig) {
	if cfg == nil {
		return
	}
	cfg.ApplicableFromMode = ApplicableFromModeNext
	cfg.ApplicableFromSlot = ""
}
