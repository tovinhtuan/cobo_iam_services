package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ApplicableToDecision codes for observability (mirror ApplicableFromDecision style).
const (
	ApplicableToDecisionOpenEnded = "open_ended"
	ApplicableToDecisionEligible  = "eligible"
	ApplicableToDecisionSkipAfter = "skip_after_applicable_to"
	ApplicableToDecisionInvalid   = "applicable_to_invalid"
)

func atErr(status int, message string, details map[string]any) error {
	return &perr.HTTPError{
		Code:       perr.CodeInvalidRequest,
		Message:    message,
		HTTPStatus: status,
		Details:    details,
	}
}

// IsOpenEndedApplicableTo reports empty / omitted ApplicableTo (OPEN_ENDED).
func IsOpenEndedApplicableTo(raw string) bool {
	return strings.TrimSpace(raw) == ""
}

// NormalizeApplicableTo validates and canonicalizes a YYYY-MM-DD business date.
// Empty / whitespace → "" (OPEN_ENDED). Rejects timestamps and non-ISO forms.
// Does not interpret "past relative to today" (Phase B activation).
func NormalizeApplicableTo(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	loc := asiaHoChiMinh()
	t, err := time.ParseInLocation("2006-01-02", raw, loc)
	if err != nil {
		return "", atErr(http.StatusBadRequest, "applicable_to must be YYYY-MM-DD", map[string]any{
			"code": ApplicableToDecisionInvalid, "value": raw,
		})
	}
	canon := t.Format("2006-01-02")
	if canon != raw {
		return "", atErr(http.StatusBadRequest, "applicable_to must be YYYY-MM-DD", map[string]any{
			"code": ApplicableToDecisionInvalid, "value": raw, "normalized": canon,
		})
	}
	return canon, nil
}

// ValidateApplicableToFormat accepts OPEN_ENDED or a strict calendar YYYY-MM-DD.
func ValidateApplicableToFormat(raw string) error {
	_, err := NormalizeApplicableTo(raw)
	return err
}

// PrepareApplicableToForDraftWrite normalizes cfg.ApplicableTo for draft persistence.
// Empty stays empty (OPEN_ENDED). Does not compare against ApplicableFrom or today.
func PrepareApplicableToForDraftWrite(cfg *TemplateDeadlineConfig) error {
	if cfg == nil {
		return nil
	}
	canon, err := NormalizeApplicableTo(cfg.ApplicableTo)
	if err != nil {
		return err
	}
	cfg.ApplicableTo = canon
	return nil
}

// UnmarshalJSON detects whether "applicable_to" was present so Upsert can preserve
// vs explicit-clear under full-replace deadline_config semantics.
func (c *TemplateDeadlineConfig) UnmarshalJSON(data []byte) error {
	type alias TemplateDeadlineConfig
	var raw alias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = TemplateDeadlineConfig(raw)
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	_, c.ApplicableToProvided = keys["applicable_to"]
	return nil
}

// ShouldPreserveApplicableTo reports omit/empty write that must not erase a stored value.
// Explicit clear: ApplicableToProvided && empty. Explicit set: non-empty value.
func ShouldPreserveApplicableTo(cfg *TemplateDeadlineConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.ApplicableToProvided {
		return false
	}
	return strings.TrimSpace(cfg.ApplicableTo) == ""
}

// ApplyCloneApplicableToDefaults clears ApplicableTo for a new template root (CLONE_APPLICABLE_TO=CLEAR).
func ApplyCloneApplicableToDefaults(cfg *TemplateDeadlineConfig) {
	if cfg == nil {
		return
	}
	cfg.ApplicableTo = ""
	cfg.ApplicableToProvided = true
}

// EvaluateApplicableToEligibility is the Phase A upper-bound filter primitive.
// Authority is resolved occurrence T (not today, not DueAt, not slot start).
// ApplicableTo empty → OPEN_ENDED (eligible). Inclusive: T_HCM_date <= ApplicableTo.
// Caller supplies occurrenceT; helper never reads time.Now().
func EvaluateApplicableToEligibility(occurrenceT time.Time, applicableTo string, loc *time.Location) (eligible bool, decision string, err error) {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	boundary := strings.TrimSpace(applicableTo)
	if boundary == "" {
		return true, ApplicableToDecisionOpenEnded, nil
	}
	canon, nerr := NormalizeApplicableTo(boundary)
	if nerr != nil {
		return false, ApplicableToDecisionInvalid, nerr
	}
	tDay := stripTime(occurrenceT.In(loc)).Format("2006-01-02")
	if tDay > canon {
		return false, ApplicableToDecisionSkipAfter, nil
	}
	return true, ApplicableToDecisionEligible, nil
}

// Activation readiness / Activate blocker codes for ApplicableTo (API-stable).
const (
	ActivationBlockerApplicableToInvalid       = "TEMPLATE_APPLICABLE_TO_INVALID"
	ActivationBlockerApplicableToPast          = "TEMPLATE_APPLICABLE_TO_PAST"
	ActivationBlockerApplicabilityRangeInvalid = "TEMPLATE_APPLICABILITY_RANGE_INVALID"
)

// CollectApplicableToActivationBlockers is the single authority for readiness + Activate.
// Side-effect free: does not freeze ApplicableFrom, persist, or create occurrences.
// Precedence: format → past (ApplicableTo < TodayHCM) → range (first T > ApplicableTo).
// OPEN_ENDED (empty) → no blockers. Non-periodic: format+past only (V1 range is PERIODIC_ONLY).
func CollectApplicableToActivationBlockers(cfg *TemplateDeadlineConfig, evalAt time.Time) []ActivationBlockerDTO {
	if cfg == nil || IsOpenEndedApplicableTo(cfg.ApplicableTo) {
		return nil
	}
	loc := asiaHoChiMinh()
	canon, err := NormalizeApplicableTo(cfg.ApplicableTo)
	if err != nil {
		return []ActivationBlockerDTO{{
			Code:    ActivationBlockerApplicableToInvalid,
			Message: "Ngày kết thúc áp dụng không hợp lệ. Định dạng phải là YYYY-MM-DD.",
		}}
	}
	todayHCM := stripTime(evalAt.In(loc)).Format("2006-01-02")
	if canon < todayHCM {
		return []ActivationBlockerDTO{{
			Code:    ActivationBlockerApplicableToPast,
			Message: "Ngày kết thúc áp dụng đã qua. Không thể kích hoạt template đã hết thời gian áp dụng.",
		}}
	}
	if !IsPeriodicFrequencyUnit(cfg.FrequencyUnit) {
		return nil
	}
	firstT, ok := resolveFirstApplicableOccurrenceT(cfg, evalAt, loc)
	if !ok {
		// Schedule/ApplicableFrom not resolvable — existing blockers own the failure.
		return nil
	}
	eligible, decision, eligErr := EvaluateApplicableToEligibility(firstT, canon, loc)
	if eligErr != nil {
		return []ActivationBlockerDTO{{
			Code:    ActivationBlockerApplicableToInvalid,
			Message: "Ngày kết thúc áp dụng không hợp lệ. Định dạng phải là YYYY-MM-DD.",
		}}
	}
	if !eligible && decision == ApplicableToDecisionSkipAfter {
		return []ActivationBlockerDTO{{
			Code:    ActivationBlockerApplicabilityRangeInvalid,
			Message: "Kỳ nghĩa vụ đầu tiên (mốc T) nằm sau ngày kết thúc áp dụng. Điều chỉnh Bắt đầu áp dụng, Kết thúc áp dụng hoặc mốc T.",
		}}
	}
	return nil
}

// resolveFirstApplicableOccurrenceT mirrors BuildFirstOccurrencePreview slot→T path without persistence.
// ok=false when frequency/slot/T cannot be resolved (caller skips range blocker).
func resolveFirstApplicableOccurrenceT(cfg *TemplateDeadlineConfig, evalAt time.Time, loc *time.Location) (time.Time, bool) {
	if cfg == nil || loc == nil {
		return time.Time{}, false
	}
	freq := NormalizeFrequencyUnit(cfg.FrequencyUnit)
	if !IsPeriodicFrequencyUnit(freq) {
		return time.Time{}, false
	}
	current := ResolveLogicalSlot(freq, evalAt, loc)
	var firstSlot string
	if IsLegacyApplicableFrom(cfg.ApplicableFromMode, cfg.ApplicableFromSlot) {
		firstSlot = current
	} else {
		if err := ValidateApplicableFromForActivate(cfg); err != nil {
			return time.Time{}, false
		}
		_, frozen, err := FreezeApplicableFromAtActivate(cfg, evalAt)
		if err != nil {
			return time.Time{}, false
		}
		boundary := strings.TrimSpace(frozen)
		first, err := ResolveFirstMaterializableSlot(freq, current, boundary, false)
		if err != nil || strings.TrimSpace(first) == "" {
			return time.Time{}, false
		}
		firstSlot = first
	}
	tEff, err := ResolveOccurrenceT(freq, firstSlot, cmsAnchorFromDeadlineConfig(cfg), loc)
	if err != nil {
		return time.Time{}, false
	}
	return tEff, true
}

// applicableToActivationHTTPError maps the first ApplicableTo blocker to Activate rejection (422).
func applicableToActivationHTTPError(b ActivationBlockerDTO) error {
	return &perr.HTTPError{
		Code:       perr.Code(b.Code),
		Message:    b.Message,
		HTTPStatus: http.StatusUnprocessableEntity,
		Details:    map[string]any{"code": b.Code},
	}
}
