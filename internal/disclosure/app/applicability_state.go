package app

import (
	"strings"
	"time"
)

// Portal-facing derived applicability states (presentation/read-model only).
// Orthogonal to template lifecycle (active/archived) and record/workflow status.
const (
	TemplateApplicabilityStateUpcoming = "UPCOMING"
	TemplateApplicabilityStateActive   = "ACTIVE"
	TemplateApplicabilityStateEnded    = "ENDED"
)

// ResolveTemplateApplicabilityState derives Portal applicability presentation from
// active-version deadline_config. Pure / deterministic / no DB / no writes.
//
// periodicityFallback is used when cfg.FrequencyUnit is empty (legacy rows).
//
// Precedence (periodic only):
//  1. ApplicableTo non-empty AND ApplicableTo < TodayHCM → ENDED
//  2. CurrentLogicalSlot before ApplicableFrom boundary → UPCOMING
//  3. else → ACTIVE
//
// ok=false → omit field (non-periodic, invalid config, unresolvable lower bound).
// INVALID_APPLICABILITY_READ_POLICY = omit_state_template_still_readable
//
// Worker upper-bound authority remains occurrence T (Phase C).
// Portal ENDED authority is TodayHCM vs ApplicableTo (inclusive end date).
func ResolveTemplateApplicabilityState(cfg *TemplateDeadlineConfig, periodicityFallback string, evalAt time.Time) (state string, ok bool) {
	if cfg == nil {
		return "", false
	}
	freq := NormalizeFrequencyUnit(cfg.FrequencyUnit)
	if !IsPeriodicFrequencyUnit(freq) {
		freq = NormalizeFrequencyUnit(periodicityFallback)
	}
	if !IsPeriodicFrequencyUnit(freq) {
		return "", false
	}
	loc := asiaHoChiMinh()
	todayHCM := stripTime(evalAt.In(loc)).Format("2006-01-02")

	if !IsOpenEndedApplicableTo(cfg.ApplicableTo) {
		canon, err := NormalizeApplicableTo(cfg.ApplicableTo)
		if err != nil {
			return "", false
		}
		if canon != "" && canon < todayHCM {
			return TemplateApplicabilityStateEnded, true
		}
	}

	currentSlot := ResolveLogicalSlot(freq, evalAt, loc)
	if strings.TrimSpace(currentSlot) == "" {
		return "", false
	}
	eligible, decision, err := EvaluateApplicableFromEligibility(
		freq, currentSlot, cfg.ApplicableFromMode, cfg.ApplicableFromSlot,
	)
	if err != nil || decision == ApplicableFromDecisionInvalidSlot || decision == ApplicableFromDecisionUnfrozenMode {
		return "", false
	}
	if !eligible && decision == ApplicableFromDecisionSkipBefore {
		return TemplateApplicabilityStateUpcoming, true
	}
	return TemplateApplicabilityStateActive, true
}

// ApplyDerivedApplicabilityState sets ApplicabilityState when derivation succeeds.
func ApplyDerivedApplicabilityState(dst *string, cfg *TemplateDeadlineConfig, periodicityFallback string, evalAt time.Time) {
	if dst == nil {
		return
	}
	state, ok := ResolveTemplateApplicabilityState(cfg, periodicityFallback, evalAt)
	if !ok {
		*dst = ""
		return
	}
	*dst = state
}
