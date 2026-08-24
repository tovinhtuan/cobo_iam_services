package app

import (
	"context"
	"strings"
	"time"
)

// First-occurrence preview status (CMS baseline; HCM business dates).
const (
	FirstOccurrenceStatusFuture     = "FUTURE"
	FirstOccurrenceStatusOpen       = "OPEN"
	FirstOccurrenceStatusDueToday   = "DUE_TODAY"
	FirstOccurrenceStatusOverdue    = "OVERDUE"
	FirstOccurrenceStatusUnavailable = "UNAVAILABLE"
)

// Activation warning codes (advisory; never flip activation_ready).
const (
	ActivationWarningFirstOccurrenceOverdue  = "FIRST_OCCURRENCE_OVERDUE"
	ActivationWarningFirstOccurrenceDueToday = "FIRST_OCCURRENCE_DUE_TODAY"
)

const PreviewScopeCMSBaseline = "cms_baseline"

// MaxLogicalSlot returns the chronologically later of a and b (frequency-aware).
func MaxLogicalSlot(frequencyUnit, a, b string) (string, error) {
	cmp, err := CompareLogicalSlots(frequencyUnit, a, b)
	if err != nil {
		return "", err
	}
	if cmp < 0 {
		return b, nil
	}
	return a, nil
}

// ResolveFirstMaterializableSlot mirrors Phase 5 current-slot-only materialization:
// legacy NULL → current; else First = max(current, boundary). Never backfills history.
func ResolveFirstMaterializableSlot(frequencyUnit, currentSlot, prospectiveBoundary string, legacy bool) (string, error) {
	currentSlot = strings.TrimSpace(currentSlot)
	if legacy || strings.TrimSpace(prospectiveBoundary) == "" {
		return currentSlot, nil
	}
	return MaxLogicalSlot(frequencyUnit, currentSlot, prospectiveBoundary)
}

// ClassifyFirstOccurrenceScheduleStatus uses HCM calendar dates for OpenAt/DueAt vs evaluation time.
func ClassifyFirstOccurrenceScheduleStatus(evalAt, openAt, dueAt time.Time, loc *time.Location) string {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	nowDate := stripTime(evalAt.In(loc))
	openDate := stripTime(openAt.In(loc))
	dueDate := stripTime(dueAt.In(loc))
	if nowDate.Before(openDate) {
		return FirstOccurrenceStatusFuture
	}
	if nowDate.After(dueDate) {
		return FirstOccurrenceStatusOverdue
	}
	if nowDate.Equal(dueDate) {
		return FirstOccurrenceStatusDueToday
	}
	return FirstOccurrenceStatusOpen
}

func previewDurationType(cfg *TemplateDeadlineConfig) string {
	if cfg == nil {
		return DurationTypeCalendarDays
	}
	dt := strings.TrimSpace(cfg.DeadlineDurationType)
	if dt != "" {
		return dt
	}
	// Match seedPeriodicCycles when no applicability_rules: working days once DeadlineDays > 0.
	if cfg.DeadlineDays > 0 {
		return DurationTypeWorkingDays
	}
	return DurationTypeCalendarDays
}

func cmsAnchorFromDeadlineConfig(cfg *TemplateDeadlineConfig) AnchorConfig {
	if cfg == nil {
		return AnchorConfig{}
	}
	return AnchorConfig{
		Month:          cfg.CycleAnchorMonth,
		Day:            cfg.CycleAnchorDay,
		Weekday:        cfg.CycleAnchorWeekday,
		MonthInQuarter: cfg.MonthInQuarter,
	}
}

func formatPreviewDate(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02")
}

func formatPreviewRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// BuildFirstOccurrencePreview is a pure CMS-baseline schedule impact preview.
// It may resolve CURRENT/NEXT prospectively for display but never persists/freezes/materializes.
// Reuses FreezeApplicableFromAtActivate for prospective boundary (same math as Activate freeze).
func BuildFirstOccurrencePreview(
	ctx context.Context,
	cfg *TemplateDeadlineConfig,
	evalAt time.Time,
	calc *DeadlineCalculator,
) (*FirstOccurrencePreviewDTO, []ActivationWarningDTO) {
	if cfg == nil || !IsPeriodicFrequencyUnit(cfg.FrequencyUnit) {
		return nil, nil
	}
	freq := NormalizeFrequencyUnit(cfg.FrequencyUnit)
	loc := asiaHoChiMinh()
	current := ResolveLogicalSlot(freq, evalAt, loc)

	out := &FirstOccurrencePreviewDTO{
		Scope:              PreviewScopeCMSBaseline,
		EvaluatedAt:        formatPreviewRFC3339(evalAt),
		FrequencyUnit:      freq,
		CurrentLogicalSlot: current,
		CompanyNote:        "Ngày thực tế tại từng doanh nghiệp có thể khác nếu doanh nghiệp đang có cấu hình mốc T tùy chỉnh.",
	}

	legacy := IsLegacyApplicableFrom(cfg.ApplicableFromMode, cfg.ApplicableFromSlot)
	mode := NormalizeApplicableFromMode(cfg.ApplicableFromMode)
	out.ApplicableFromMode = mode

	var boundary string
	if legacy {
		out.ProspectiveApplicableFromSlot = nil
		out.FirstOccurrenceSlot = current
		out.FirstOccurrenceIsCurrentCandidate = true
	} else {
		// Prospective resolve only — identical algorithm to Activate freeze; no DB write.
		_, frozen, err := FreezeApplicableFromAtActivate(cfg, evalAt)
		if err != nil {
			out.Status = FirstOccurrenceStatusUnavailable
			out.UnavailableReason = err.Error()
			return out, nil
		}
		boundary = strings.TrimSpace(frozen)
		if boundary != "" {
			b := boundary
			out.ProspectiveApplicableFromSlot = &b
		}
		first, err := ResolveFirstMaterializableSlot(freq, current, boundary, false)
		if err != nil {
			out.Status = FirstOccurrenceStatusUnavailable
			out.UnavailableReason = err.Error()
			return out, nil
		}
		out.FirstOccurrenceSlot = first
		out.FirstOccurrenceIsCurrentCandidate = first == current
	}

	if cfg.DeadlineDays <= 0 {
		out.Status = FirstOccurrenceStatusUnavailable
		out.UnavailableReason = "deadline_days is required for schedule preview"
		return out, nil
	}

	tEff, err := ResolveOccurrenceT(freq, out.FirstOccurrenceSlot, cmsAnchorFromDeadlineConfig(cfg), loc)
	if err != nil {
		out.Status = FirstOccurrenceStatusUnavailable
		out.UnavailableReason = err.Error()
		return out, nil
	}
	tStr := formatPreviewDate(tEff, loc)
	out.T = &tStr

	openAt := ResolveOpenAt(tEff, cfg.OpenDaysBeforeT)
	openStr := formatPreviewDate(openAt, loc)
	out.OpenAt = &openStr

	durationType := previewDurationType(cfg)
	var dueAt time.Time
	if calc != nil {
		dueAt, err = calc.addDurationInclusive(ctx, tEff, cfg.DeadlineDays, durationType)
	} else if durationType == DurationTypeCalendarDays {
		dueAt = tEff.AddDate(0, 0, cfg.DeadlineDays-1)
	} else {
		out.Status = FirstOccurrenceStatusUnavailable
		out.UnavailableReason = "deadline calculator unavailable for working-day due date"
		return out, nil
	}
	if err != nil {
		out.Status = FirstOccurrenceStatusUnavailable
		out.UnavailableReason = err.Error()
		return out, nil
	}
	dueStr := formatPreviewDate(dueAt, loc)
	out.DueAt = &dueStr

	out.Status = ClassifyFirstOccurrenceScheduleStatus(evalAt, openAt, dueAt, loc)
	warnings := activationWarningsForPreview(out)
	return out, warnings
}

func activationWarningsForPreview(preview *FirstOccurrencePreviewDTO) []ActivationWarningDTO {
	if preview == nil {
		return nil
	}
	switch preview.Status {
	case FirstOccurrenceStatusOverdue:
		return []ActivationWarningDTO{{
			Code:     ActivationWarningFirstOccurrenceOverdue,
			Severity: "WARNING",
			Message:  "Kỳ đầu dự kiến theo cấu hình CMS đã quá hạn. Nếu kích hoạt bây giờ, nghĩa vụ có thể được tạo theo lịch xử lý hiện tại và được xác định là quá hạn.",
			Blocking: false,
		}}
	case FirstOccurrenceStatusDueToday:
		return []ActivationWarningDTO{{
			Code:     ActivationWarningFirstOccurrenceDueToday,
			Severity: "WARNING",
			Message:  "Kỳ đầu dự kiến theo cấu hình CMS có hạn hoàn thành trong hôm nay.",
			Blocking: false,
		}}
	default:
		return nil
	}
}

// BuildFirstOccurrencePreviewFromFrozenBoundary uses an already-resolved boundary
// (e.g. Activate freeze result) so warning and persisted freeze share one snapshot.
func BuildFirstOccurrencePreviewFromFrozenBoundary(
	ctx context.Context,
	cfg *TemplateDeadlineConfig,
	evalAt time.Time,
	mode, frozenSlot string,
	calc *DeadlineCalculator,
) (*FirstOccurrencePreviewDTO, []ActivationWarningDTO) {
	if cfg == nil {
		return nil, nil
	}
	cfgCopy := *cfg
	if IsLegacyApplicableFrom(mode, frozenSlot) && IsLegacyApplicableFrom(cfg.ApplicableFromMode, cfg.ApplicableFromSlot) {
		return BuildFirstOccurrencePreview(ctx, &cfgCopy, evalAt, calc)
	}
	if strings.TrimSpace(frozenSlot) != "" {
		cfgCopy.ApplicableFromMode = ApplicableFromModeSpecific
		cfgCopy.ApplicableFromSlot = frozenSlot
	} else {
		cfgCopy.ApplicableFromMode = mode
		cfgCopy.ApplicableFromSlot = ""
	}
	return BuildFirstOccurrencePreview(ctx, &cfgCopy, evalAt, calc)
}
