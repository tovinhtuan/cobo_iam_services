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

// absoluteDueResult is the shared List/Detail absolute-due outcome (cycle wins preview).
type absoluteDueResult struct {
	DueAt  *string
	Source string
}

func loadPortalCycleDuesByTypeLabel(
	ctx context.Context,
	repo Repository,
	companyID string,
	typeIDs []string,
) map[string]PortalListCycleDueRow {
	out := map[string]PortalListCycleDueRow{}
	if repo == nil || strings.TrimSpace(companyID) == "" || len(typeIDs) == 0 {
		return out
	}
	reader, ok := repo.(portalListCycleDueReader)
	if !ok {
		return out
	}
	rows, err := reader.ListPortalListCycleDues(ctx, companyID, typeIDs)
	if err != nil {
		return out
	}
	for _, row := range rows {
		key := row.TypeID + "|" + row.CycleLabel
		out[key] = row
	}
	return out
}

func lookupPersistedCycleDueForSlot(
	cycleByTypeLabel map[string]PortalListCycleDueRow,
	typeID string,
	freq string,
	now time.Time,
	loc *time.Location,
) (dueYYYYMMDD string, source string, ok bool) {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	slot := ResolveLogicalSlot(freq, now, loc)
	row, found := cycleByTypeLabel[strings.TrimSpace(typeID)+"|"+slot]
	if !found || strings.TrimSpace(row.DueDateYYYYMMDD) == "" {
		return "", "", false
	}
	src := strings.TrimSpace(row.Source)
	if src == "" {
		src = resolvedDueSourceCycleDue
	}
	return strings.TrimSpace(row.DueDateYYYYMMDD), src, true
}

func skipsAbsoluteDueResolution(templateCategory string, cfg *TemplateDeadlineConfig) bool {
	if cfg == nil {
		return true
	}
	cat := strings.ToLower(strings.TrimSpace(templateCategory))
	if cat == "" {
		cat = strings.ToLower(strings.TrimSpace(cfg.TemplateCategory))
	}
	if cat == TemplateCategoryIrregular || cat == "ad_hoc" || cat == "event_based" {
		return true
	}
	if cfg.DeadlineMode == DeadlineModeNone {
		return true
	}
	return false
}

func frequencyForAbsoluteDue(cfg *TemplateDeadlineConfig, periodicity string) string {
	if cfg != nil {
		if freq := strings.TrimSpace(cfg.FrequencyUnit); freq != "" {
			return freq
		}
	}
	return strings.TrimSpace(periodicity)
}

// resolveAbsoluteDueFromCycleOrPreview applies shared precedence:
// persisted cycle (PLANNED_DATE|CYCLE_DUE) → preview YYYY-MM-DD → empty.
// Does not write DB. previewDeadlineYYYYMMDD is optional (Detail reuses summary; List may compute).
func resolveAbsoluteDueFromCycleOrPreview(
	cycleByTypeLabel map[string]PortalListCycleDueRow,
	typeID string,
	freq string,
	now time.Time,
	loc *time.Location,
	previewDeadlineYYYYMMDD string,
) absoluteDueResult {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	if ymd, src, ok := lookupPersistedCycleDueForSlot(cycleByTypeLabel, typeID, freq, now, loc); ok {
		if at, okFmt := FormatResolvedDueAtHCMEOD(ymd, loc); okFmt {
			return absoluteDueResult{DueAt: &at, Source: src}
		}
	}
	if at, okFmt := FormatResolvedDueAtHCMEOD(previewDeadlineYYYYMMDD, loc); okFmt {
		src := resolvedDueSourceDeadlineSummaryPreview
		return absoluteDueResult{DueAt: &at, Source: src}
	}
	return absoluteDueResult{}
}

func previewDeadlineYYYYMMDDFromSummary(summary *DeadlineSummaryDTO) string {
	if summary == nil || summary.DeadlineDate == nil {
		return ""
	}
	return strings.TrimSpace(*summary.DeadlineDate)
}

func yyyyMMDDFromResolvedDueAt(resolvedDueAt string, loc *time.Location) string {
	resolvedDueAt = strings.TrimSpace(resolvedDueAt)
	if resolvedDueAt == "" {
		return ""
	}
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	t, err := time.Parse(time.RFC3339, resolvedDueAt)
	if err != nil {
		return ""
	}
	return t.In(loc).Format("2006-01-02")
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

	cycleByTypeLabel := loadPortalCycleDuesByTypeLabel(ctx, s.repo, companyID, typeIDs)

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
		if skipsAbsoluteDueResolution(item.TemplateCategory, cfg) {
			continue
		}

		freq := frequencyForAbsoluteDue(cfg, item.Periodicity)
		previewYMD := ""
		if _, _, cycleOK := lookupPersistedCycleDueForSlot(cycleByTypeLabel, item.TypeID, freq, now, loc); !cycleOK {
			if s.calculator != nil && cfg.DeadlineMode == DeadlineModePeriodic {
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
				if calcErr == nil {
					previewYMD = previewDeadlineYYYYMMDDFromSummary(summary)
				}
			}
		}

		res := resolveAbsoluteDueFromCycleOrPreview(cycleByTypeLabel, item.TypeID, freq, now, loc, previewYMD)
		item.ResolvedDueAt = res.DueAt
		item.ResolvedDueSource = res.Source
	}
}

// attachDetailAbsoluteResolvedDue sets DisclosureTypeDTO.resolved_due_at / resolved_due_source
// with the same precedence as portal list (cycle → preview). Reuses deadline_summary for preview
// when already computed. Syncs resolved_deadline_rule.due_date to the absolute SoT date (YYYY-MM-DD).
func (s *service) attachDetailAbsoluteResolvedDue(
	ctx context.Context,
	companyID string,
	item *DisclosureTypeDTO,
	now time.Time,
	summary *DeadlineSummaryDTO,
) {
	if s == nil || item == nil || strings.TrimSpace(companyID) == "" {
		return
	}
	loc := asiaHoChiMinh()
	now = now.In(loc)
	cfg := item.DeadlineConfig
	if skipsAbsoluteDueResolution(item.TemplateCategory, cfg) {
		return
	}

	freq := frequencyForAbsoluteDue(cfg, item.Periodicity)
	cycleByTypeLabel := loadPortalCycleDuesByTypeLabel(ctx, s.repo, companyID, []string{item.TypeID})
	previewYMD := previewDeadlineYYYYMMDDFromSummary(summary)
	res := resolveAbsoluteDueFromCycleOrPreview(cycleByTypeLabel, item.TypeID, freq, now, loc, previewYMD)
	item.ResolvedDueAt = res.DueAt
	item.ResolvedDueSource = res.Source

	if res.DueAt == nil {
		return
	}
	// Keep semantic DTO due_date aligned with absolute SoT (not preview when cycle wins).
	ymd := yyyyMMDDFromResolvedDueAt(*res.DueAt, loc)
	if ymd == "" {
		return
	}
	if item.ResolvedDeadlineRule != nil {
		due := ymd
		item.ResolvedDeadlineRule.DueDate = &due
	}
}
