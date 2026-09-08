package app

import (
	"context"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// portalListCycleDueReader is optional — production MySQL implements it; test doubles may omit.
type portalListCycleDueReader interface {
	ListPortalListCycleDues(ctx context.Context, companyID string, typeIDs []string) ([]PortalListCycleDueRow, error)
}

const (
	resolvedDueSourceCycleDue               = "CYCLE_DUE"
	resolvedDueSourcePlannedDate            = "PLANNED_DATE"
	resolvedDueSourceDeadlineSummaryPreview = "DEADLINE_SUMMARY_PREVIEW"
)

// FormatResolvedDueAtHCMEOD maps a business due date (YYYY-MM-DD) to Asia/Ho_Chi_Minh end-of-day RFC3339.
// Presentation only — does not change DueAt calculation authority.
func FormatResolvedDueAtHCMEOD(dueDateYYYYMMDD string, loc *time.Location) (string, bool) {
	dueDateYYYYMMDD = strings.TrimSpace(dueDateYYYYMMDD)
	if dueDateYYYYMMDD == "" {
		return "", false
	}
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	day, err := time.ParseInLocation("2006-01-02", dueDateYYYYMMDD, loc)
	if err != nil {
		return "", false
	}
	eod := time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 0, loc)
	return eod.Format(time.RFC3339), true
}

// enrichPortalListResolvedDue attaches company-scoped resolved_due_at for portal list cards.
// Prefer persisted occurrence due (planned_date > cycle due) for the current logical slot;
// else BE deadline-summary preview (same calculator as GetTypeDetail). Never invent FE-side math.
func (s *service) enrichPortalListResolvedDue(
	ctx context.Context,
	companyID string,
	items []DisclosureTypeSummaryDTO,
	now time.Time,
) {
	if s == nil || len(items) == 0 || strings.TrimSpace(companyID) == "" {
		return
	}
	loc := asiaHoChiMinh()
	now = now.In(loc)

	typeIDs := make([]string, 0, len(items))
	for _, it := range items {
		if id := strings.TrimSpace(it.TypeID); id != "" {
			typeIDs = append(typeIDs, id)
		}
	}

	cycleByTypeLabel := map[string]PortalListCycleDueRow{}
	if reader, ok := s.repo.(portalListCycleDueReader); ok {
		rows, err := reader.ListPortalListCycleDues(ctx, companyID, typeIDs)
		if err == nil {
			for _, row := range rows {
				key := row.TypeID + "|" + row.CycleLabel
				cycleByTypeLabel[key] = row
			}
		}
	}

	baseCtx := CompanyDeadlineContext{CompanyID: companyID}
	if ctxVal, err := s.repo.GetCompanyDeadlineContext(ctx, companyID); err == nil {
		baseCtx = ctxVal
		baseCtx.CompanyID = companyID
	}
	prefs, prefErr := s.repo.ListCompanyTypePreferencesByTypeIDs(ctx, typeIDs)
	if prefErr != nil {
		prefs = nil
	}
	prefByType := map[string]CompanyTypePreference{}
	for _, p := range prefs {
		if p.CompanyID != companyID {
			continue
		}
		prefByType[p.TypeID] = p
	}
	profile, profileErr := s.repo.GetCompanyApplicabilityProfile(ctx, companyID)
	haveProfile := profileErr == nil

	for i := range items {
		item := &items[i]
		if item.ApplicabilityRules != nil && haveProfile {
			item.ResolvedDeadlineRule = buildResolvedDeadlineRuleDTO(
				item.ApplicabilityRules, profile, item.Periodicity, item.DeadlineConfig,
			)
		}
		cfg := item.DeadlineConfig
		if cfg == nil {
			continue
		}
		cat := strings.ToLower(strings.TrimSpace(item.TemplateCategory))
		if cat == "" {
			cat = strings.ToLower(strings.TrimSpace(cfg.TemplateCategory))
		}
		if cat == TemplateCategoryIrregular || cat == "ad_hoc" || cat == "event_based" {
			// Irregular: no company DueAt on list — FE keeps event-relative copy.
			continue
		}
		if cfg.DeadlineMode == DeadlineModeNone {
			continue
		}

		freq := strings.TrimSpace(cfg.FrequencyUnit)
		if freq == "" {
			freq = strings.TrimSpace(item.Periodicity)
		}
		slot := ResolveLogicalSlot(freq, now, loc)
		if row, ok := cycleByTypeLabel[item.TypeID+"|"+slot]; ok && strings.TrimSpace(row.DueDateYYYYMMDD) != "" {
			if at, okFmt := FormatResolvedDueAtHCMEOD(row.DueDateYYYYMMDD, loc); okFmt {
				item.ResolvedDueAt = &at
				item.ResolvedDueSource = row.Source
				if item.ResolvedDueSource == "" {
					item.ResolvedDueSource = resolvedDueSourceCycleDue
				}
				continue
			}
		}

		if s.calculator == nil || cfg.DeadlineMode != DeadlineModePeriodic {
			continue
		}
		companyCtx := baseCtx
		companyCtx.CompanyID = companyID
		if pref, ok := prefByType[item.TypeID]; ok {
			companyCtx.CycleAnchorMonth = pref.CycleAnchorMonth
			companyCtx.CycleAnchorDay = pref.CycleAnchorDay
			companyCtx.CycleAnchorWeekday = pref.CycleAnchorWeekday
			companyCtx.MonthInQuarter = pref.MonthInQuarter
			companyCtx.OverrideActive = pref.OverrideActive
			companyCtx.OverrideFrequency = pref.OverrideFrequency
		}
		deadlineCfg := cfg
		if item.ApplicabilityRules != nil && haveProfile {
			cfgCopy := *cfg
			if days, ok := applicability.ResolveDeadlineDays(item.ApplicabilityRules, profile); ok {
				cfgCopy.DeadlineDays = days
			}
			cfgCopy.DeadlineDurationType = applicability.ResolveDeadlineDurationType(item.ApplicabilityRules)
			deadlineCfg = &cfgCopy
		}
		summary, calcErr := s.calculator.CalculateDeadlineSummary(ctx, deadlineCfg, companyCtx, now)
		if calcErr != nil || summary == nil || summary.DeadlineDate == nil || strings.TrimSpace(*summary.DeadlineDate) == "" {
			continue
		}
		if at, okFmt := FormatResolvedDueAtHCMEOD(*summary.DeadlineDate, loc); okFmt {
			item.ResolvedDueAt = &at
			item.ResolvedDueSource = resolvedDueSourceDeadlineSummaryPreview
		}
	}
}
