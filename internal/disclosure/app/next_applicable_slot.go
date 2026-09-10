package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Periodic cycle future pre-generation lead-day bounds (CMS write contract).
const (
	PeriodicCycleGenerationLeadDaysMin = 0
	PeriodicCycleGenerationLeadDaysMax = 90
)

// ValidatePeriodicCycleGenerationLeadDays enforces 0..90 calendar days.
// nil means absent/legacy (no future pregen) and is allowed.
func ValidatePeriodicCycleGenerationLeadDays(v *int) error {
	if v == nil {
		return nil
	}
	if *v < PeriodicCycleGenerationLeadDaysMin || *v > PeriodicCycleGenerationLeadDaysMax {
		return perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			fmt.Sprintf(
				"periodic_cycle_generation_lead_days must be between %d and %d (got %d)",
				PeriodicCycleGenerationLeadDaysMin, PeriodicCycleGenerationLeadDaysMax, *v,
			),
			nil,
		)
	}
	return nil
}

// GenerateAtDate returns the calendar date on which future pregen may begin:
// T − leadDays (date-only in t's location). leadDays <= 0 yields T's date.
func GenerateAtDate(t time.Time, leadDays int) time.Time {
	if leadDays <= 0 {
		return stripTime(t)
	}
	return stripTime(t.AddDate(0, 0, -leadDays))
}

func maxNextApplicableIterations(frequencyUnit string) int {
	switch NormalizeFrequencyUnit(frequencyUnit) {
	case PeriodicityDaily:
		return 366
	case PeriodicityWeekly:
		return 60
	case PeriodicityMonthly:
		return 120
	case PeriodicityQuarterly:
		return 40
	case PeriodicityYearly:
		return 15
	default:
		return 120
	}
}

// ResolveNextApplicableLogicalSlot returns ONE future applicable cycle_label after currentSlot,
// or empty when none exists within the bounded forward walk.
//
// Contract: NEXT_APPLICABLE_LOGICAL_SLOT_ONLY — start at NextLogicalSlot(current); never walk
// historical. For SPECIFIC_SLOT ApplicableFrom, if next < boundary and boundary > current,
// jump to boundary. Then apply AF → ResolveOccurrenceT(anchor) → AT; on AF miss advance
// forward only; on AT miss stop (later T cannot re-open).
func ResolveNextApplicableLogicalSlot(
	frequencyUnit, currentSlot string,
	applicableFromMode, applicableFromSlot, applicableTo string,
	anchor AnchorConfig,
	loc *time.Location,
) (string, error) {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	freq := NormalizeFrequencyUnit(frequencyUnit)
	current := strings.TrimSpace(currentSlot)
	if current == "" {
		return "", nil
	}

	cand, err := NextLogicalSlot(freq, current)
	if err != nil {
		return "", err
	}

	mode := NormalizeApplicableFromMode(applicableFromMode)
	boundary := strings.TrimSpace(applicableFromSlot)
	if mode == ApplicableFromModeSpecific && boundary != "" {
		cmpNext, errNext := CompareLogicalSlots(freq, cand, boundary)
		cmpBound, errBound := CompareLogicalSlots(freq, boundary, current)
		if errNext == nil && errBound == nil && cmpNext < 0 && cmpBound > 0 {
			norm, nerr := NormalizeLogicalSlot(freq, boundary)
			if nerr != nil {
				return "", nerr
			}
			cand = norm
		}
	}

	maxIter := maxNextApplicableIterations(freq)
	for i := 0; i < maxIter; i++ {
		afOK, _, afErr := EvaluateApplicableFromEligibility(freq, cand, applicableFromMode, applicableFromSlot)
		if afErr != nil {
			return "", afErr
		}
		if !afOK {
			next, nerr := NextLogicalSlot(freq, cand)
			if nerr != nil {
				return "", nerr
			}
			cand = next
			continue
		}

		occT, terr := ResolveOccurrenceT(freq, cand, anchor, loc)
		if terr != nil {
			return "", terr
		}
		toOK, _, toErr := EvaluateApplicableToEligibility(occT, applicableTo, loc)
		if toErr != nil {
			return "", toErr
		}
		if !toOK {
			// Later slots have later T → also fail AT. Stop.
			return "", nil
		}
		return cand, nil
	}
	return "", nil
}
