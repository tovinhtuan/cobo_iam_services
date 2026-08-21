package app

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	DeadlineModeNone        = "NONE"
	DeadlineModeFixedDate   = "FIXED_DATE"
	DeadlineModeDynamicRule = "DYNAMIC_RULE"
	DeadlineModePeriodic    = "PERIODIC"

	NonTradingPolicyWarnOnlyKeepDate = "WARN_ONLY_KEEP_DATE"
	NonTradingPolicyMoveNextWorking  = "MOVE_TO_NEXT_WORKING_DAY"

	BaseDateSourceCompanyEstablished = "COMPANY_ESTABLISHED_DATE"
	// BaseDateSourceCycleStart is the semantic base for PERIODIC calculatePeriodic (cycleStart + N).
	// Not currently written onto DeadlineSummaryDTO.BaseDateSource; used by resolved_deadline_rule.
	BaseDateSourceCycleStart = "CYCLE_START"

	DurationTypeWorkingDays  = "WORKING_DAYS"
	DurationTypeCalendarDays = "CALENDAR_DAYS"

	HolidayCalendarSourceByYear = "NON_TRADING_DAYS_BY_YEAR"
)

var (
	ErrMissingDeadlineConfig   = errors.New("deadline config is required")
	ErrUnsupportedDeadlineMode = errors.New("unsupported deadline mode")
	ErrInvalidDuration         = errors.New("duration must be > 0")
	ErrMissingFixedDate        = errors.New("fixed deadline date is required")
)

type CompanyDeadlineContext struct {
	CompanyID string
	// Optional current year used to infer fallback established date (01-01-currentYear).
	CurrentYear int
	// Optional absolute established date. If nil, fallback 01-01-currentYear is used.
	EstablishedDate *time.Time
	// Optional month/day source when absolute date is unavailable.
	EstablishedMonth int
	EstablishedDay   int
	// Per-company cycle anchor override for PERIODIC deadline mode.
	// 0 = use template default (CycleAnchorMonth/CycleAnchorDay in TemplateDeadlineConfig).
	CycleAnchorMonth int
	CycleAnchorDay   int
}

type HolidayCalendarProvider interface {
	IsNonTradingDay(ctx context.Context, date time.Time) (bool, string, error)
}

type DeadlineCalculator struct {
	location *time.Location
	holidays HolidayCalendarProvider
}

func NewDeadlineCalculator(holidays HolidayCalendarProvider) *DeadlineCalculator {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return &DeadlineCalculator{
		location: loc,
		holidays: holidays,
	}
}

func (c *DeadlineCalculator) CalculateDeadlineSummary(
	ctx context.Context,
	config *TemplateDeadlineConfig,
	company CompanyDeadlineContext,
	now time.Time,
) (*DeadlineSummaryDTO, error) {
	if config == nil {
		return nil, ErrMissingDeadlineConfig
	}
	now = now.In(c.location)
	switch config.DeadlineMode {
	case DeadlineModeNone:
		return nil, nil
	case DeadlineModeFixedDate:
		return c.calculateFixedDate(ctx, config.FixedDeadline, now)
	case DeadlineModeDynamicRule:
		return c.calculateDynamicRule(ctx, config.DynamicRule, company, now)
	case DeadlineModePeriodic:
		return c.calculatePeriodic(ctx, config, company, now)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDeadlineMode, config.DeadlineMode)
	}
}

func (c *DeadlineCalculator) calculateFixedDate(
	ctx context.Context,
	cfg *FixedDeadlineConfig,
	now time.Time,
) (*DeadlineSummaryDTO, error) {
	if cfg == nil || cfg.Date == "" {
		return nil, ErrMissingFixedDate
	}
	date, err := time.ParseInLocation("2006-01-02", cfg.Date, c.location)
	if err != nil {
		return nil, fmt.Errorf("invalid fixed deadline date: %w", err)
	}
	policy := cfg.NonTradingPolicy
	if policy == "" {
		policy = NonTradingPolicyWarnOnlyKeepDate
	}
	actual := date
	adjusted := false
	var reason *string
	nonTrading, nonTradingReason, err := c.isNonTradingDay(ctx, actual)
	if err != nil {
		return nil, err
	}
	if nonTrading {
		reason = ptrString(nonTradingReason)
		if policy == NonTradingPolicyMoveNextWorking {
			actual, err = c.getNextWorkingDay(ctx, actual)
			if err != nil {
				return nil, err
			}
			adjusted = true
		}
	}
	remainingDays := int(actual.Sub(stripTime(now)).Hours() / 24)
	status := deriveSummaryStatus(remainingDays)
	return &DeadlineSummaryDTO{
		DeadlineMode:                 DeadlineModeFixedDate,
		FixedDeadlineDate:            ptrString(date.Format("2006-01-02")),
		TentativeDeadline:            ptrString(date.Format("2006-01-02")),
		ActualDeadline:               ptrString(actual.Format("2006-01-02")),
		AdjustedBecauseNonTradingDay: ptrBool(adjusted),
		NonTradingDayReason:          reason,
		DeadlineDate:                 ptrString(actual.Format("2006-01-02")),
		RemainingDays:                &remainingDays,
		Status:                       status,
		Timezone:                     ptrString(c.location.String()),
	}, nil
}

func (c *DeadlineCalculator) calculateDynamicRule(
	ctx context.Context,
	cfg *DynamicDeadlineRule,
	company CompanyDeadlineContext,
	now time.Time,
) (*DeadlineSummaryDTO, error) {
	if cfg == nil {
		return nil, ErrMissingDeadlineConfig
	}
	if cfg.Duration <= 0 {
		return nil, ErrInvalidDuration
	}
	startDate, err := c.resolveStartDate(company, now.Year())
	if err != nil {
		return nil, err
	}
	tentative, err := c.addDurationInclusive(ctx, startDate, cfg.Duration, cfg.DurationType)
	if err != nil {
		return nil, err
	}
	actual := tentative
	adjusted := false
	var reason *string
	if cfg.AdjustIfNonTradingDay {
		nonTrading, nonTradingReason, err := c.isNonTradingDay(ctx, actual)
		if err != nil {
			return nil, err
		}
		if nonTrading {
			reason = ptrString(nonTradingReason)
			actual, err = c.getNextWorkingDay(ctx, actual)
			if err != nil {
				return nil, err
			}
			adjusted = true
		}
	}
	remainingDays := int(actual.Sub(stripTime(now)).Hours() / 24)
	status := deriveSummaryStatus(remainingDays)
	return &DeadlineSummaryDTO{
		DeadlineMode:                 DeadlineModeDynamicRule,
		StartDate:                    ptrString(startDate.Format("2006-01-02")),
		BaseDateSource:               ptrString(cfg.BaseDateSource),
		TentativeDeadline:            ptrString(tentative.Format("2006-01-02")),
		ActualDeadline:               ptrString(actual.Format("2006-01-02")),
		Duration:                     &cfg.Duration,
		DurationType:                 ptrString(cfg.DurationType),
		InclusiveStart:               ptrBool(cfg.InclusiveStart),
		AdjustedBecauseNonTradingDay: ptrBool(adjusted),
		NonTradingDayReason:          reason,
		DeadlineDate:                 ptrString(actual.Format("2006-01-02")),
		RemainingDays:                &remainingDays,
		Status:                       status,
		RuleCode:                     ptrString(cfg.RuleType),
		RuleDescription:              ptrString(cfg.Description),
		Timezone:                     ptrString(c.location.String()),
	}, nil
}

func (c *DeadlineCalculator) resolveStartDate(company CompanyDeadlineContext, currentYear int) (time.Time, error) {
	if company.EstablishedDate != nil {
		d := company.EstablishedDate.In(c.location)
		return stripTime(time.Date(currentYear, d.Month(), d.Day(), 0, 0, 0, 0, c.location)), nil
	}
	month := company.EstablishedMonth
	day := company.EstablishedDay
	if month <= 0 || month > 12 {
		month = 1
	}
	if day <= 0 || day > 31 {
		day = 1
	}
	year := company.CurrentYear
	if year <= 0 {
		year = currentYear
	}
	return stripTime(time.Date(year, time.Month(month), day, 0, 0, 0, 0, c.location)), nil
}

// AddDurationInclusive exposes production inclusive duration math for guarded tooling.
func (c *DeadlineCalculator) AddDurationInclusive(
	ctx context.Context,
	start time.Time,
	duration int,
	durationType string,
) (time.Time, error) {
	return c.addDurationInclusive(ctx, start, duration, durationType)
}

func (c *DeadlineCalculator) addDurationInclusive(
	ctx context.Context,
	start time.Time,
	duration int,
	durationType string,
) (time.Time, error) {
	switch durationType {
	case DurationTypeCalendarDays:
		return start.AddDate(0, 0, duration-1), nil
	case DurationTypeWorkingDays:
		current := start
		nonTrading, _, err := c.isNonTradingDay(ctx, current)
		if err != nil {
			return time.Time{}, err
		}
		if nonTrading {
			current, err = c.getNextWorkingDay(ctx, current)
			if err != nil {
				return time.Time{}, err
			}
		}
		count := 1
		for count < duration {
			current = current.AddDate(0, 0, 1)
			nonTrading, _, err := c.isNonTradingDay(ctx, current)
			if err != nil {
				return time.Time{}, err
			}
			if !nonTrading {
				count++
			}
		}
		return current, nil
	default:
		return time.Time{}, fmt.Errorf("unsupported duration_type: %s", durationType)
	}
}

func (c *DeadlineCalculator) getNextWorkingDay(ctx context.Context, date time.Time) (time.Time, error) {
	current := date
	for i := 0; i < 370; i++ {
		nonTrading, _, err := c.isNonTradingDay(ctx, current)
		if err != nil {
			return time.Time{}, err
		}
		if !nonTrading {
			return current, nil
		}
		current = current.AddDate(0, 0, 1)
	}
	return time.Time{}, errors.New("next working day search exceeded safe limit")
}

func (c *DeadlineCalculator) isNonTradingDay(ctx context.Context, date time.Time) (bool, string, error) {
	weekday := date.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return true, "WEEKEND", nil
	}
	if c.holidays == nil {
		return false, "", nil
	}
	return c.holidays.IsNonTradingDay(ctx, date)
}

func deriveSummaryStatus(remainingDays int) string {
	if remainingDays < 0 {
		return "OVERDUE"
	}
	if remainingDays == 0 {
		return "DUE_SOON"
	}
	return "UPCOMING"
}

func stripTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func ptrString(v string) *string { return &v }
func ptrBool(v bool) *bool       { return &v }

// calculatePeriodic computes the deadline as cycleStart + DeadlineDays working days,
// where cycleStart is derived from the template's frequency and cycle anchor.
// Per-company CycleAnchorMonth/Day (CompanyDeadlineContext) takes precedence over template defaults.
// Returns nil when DeadlineDays <= 0 (no deadline configured).
//
// READY_FOR_5B (Portal Preview / Deadline Hint): this is Source A — the
// preview-only twin of deadlineengine's Source C (locked Cycle Start SoT).
// Cutover replaces this body with DeadlineEngineAdapter.ResolveDeadline;
// behavior unchanged in Batch 5A.
func (c *DeadlineCalculator) calculatePeriodic(
	ctx context.Context,
	config *TemplateDeadlineConfig,
	company CompanyDeadlineContext,
	now time.Time,
) (*DeadlineSummaryDTO, error) {
	if config.DeadlineDays <= 0 {
		return nil, nil
	}
	cycleStart := c.computeCycleStart(config, company, now)
	durationType := DurationTypeWorkingDays
	if config.DeadlineDurationType != "" {
		durationType = config.DeadlineDurationType
	}
	deadline, err := c.addDurationInclusive(ctx, cycleStart, config.DeadlineDays, durationType)
	if err != nil {
		return nil, err
	}
	remainingDays := int(deadline.Sub(stripTime(now)).Hours() / 24)
	status := deriveSummaryStatus(remainingDays)
	dayLabel := "ngày làm việc"
	if durationType == DurationTypeCalendarDays {
		dayLabel = "ngày dương lịch"
	}
	ruleDesc := fmt.Sprintf("T0 ngày %s + %d %s", cycleStart.Format("2006-01-02"), config.DeadlineDays, dayLabel)
	return &DeadlineSummaryDTO{
		DeadlineMode:      DeadlineModePeriodic,
		StartDate:         ptrString(cycleStart.Format("2006-01-02")),
		TentativeDeadline: ptrString(deadline.Format("2006-01-02")),
		ActualDeadline:    ptrString(deadline.Format("2006-01-02")),
		DeadlineDate:      ptrString(deadline.Format("2006-01-02")),
		Duration:          &config.DeadlineDays,
		DurationType:      ptrString(durationType),
		RemainingDays:     &remainingDays,
		Status:            status,
		RuleDescription:   ptrString(ruleDesc),
		Timezone:          ptrString(c.location.String()),
	}, nil
}

// computeCycleStart returns Effective T for the current logical slot (canonical).
// Priority: company.CycleAnchorMonth/Day overrides config; invalid days clamp to last day of month.
func (c *DeadlineCalculator) computeCycleStart(config *TemplateDeadlineConfig, company CompanyDeadlineContext, now time.Time) time.Time {
	cms := AnchorConfig{Month: config.CycleAnchorMonth, Day: config.CycleAnchorDay}
	co := AnchorConfig{Month: company.CycleAnchorMonth, Day: company.CycleAnchorDay}
	eff, _ := ResolveEffectiveAnchor(cms, co)
	label := ResolveLogicalSlot(config.FrequencyUnit, now, c.location)
	t, err := ResolveOccurrenceT(config.FrequencyUnit, label, eff, c.location)
	if err != nil {
		return stripTime(now.In(c.location))
	}
	return t
}
