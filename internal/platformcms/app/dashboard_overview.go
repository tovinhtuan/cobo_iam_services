package app

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

// Platform CMS dashboard overview (docs/ai-cache/cms-dashboard-enrichment-2026-10-11/02-contract.md).
// Every section is computed independently: a failing or slow section is reported as
// {"status":"error"} while the others still render. Numbers are platform-wide aggregates
// (counts + company names only), never record content.

const (
	DashboardSectionCompanies     = "companies"
	DashboardSectionSubscriptions = "subscriptions"
	DashboardSectionUsers         = "users"
	DashboardSectionTemplates     = "templates"
	DashboardSectionDisclosures   = "disclosures"
	DashboardSectionSystemHealth  = "system_health"
	DashboardSectionGrowth        = "growth"

	DashboardStatusOK        = "ok"
	DashboardStatusError     = "error"
	DashboardStatusForbidden = "forbidden"

	DashboardSeverityCritical = "critical"
	DashboardSeverityWarning  = "warning"
	DashboardSeverityInfo     = "info"

	dashboardErrorTimeout  = "TIMEOUT"
	dashboardErrorInternal = "INTERNAL"

	// DashboardTimezone is the business calendar used for "today" (same as tenant KPIs).
	DashboardTimezone = "Asia/Ho_Chi_Minh"

	// Thresholds approved with the contract (D7); tune after PO feedback.
	DashboardSubscriptionExpiringWindow = 14 * 24 * time.Hour
	DashboardStuckThreshold             = 15 * time.Minute
	DashboardTopOverdueLimit            = 5
	DashboardGrowthDays                 = 30
	// From this day of November on, a missing next-year holiday calendar is flagged.
	dashboardHolidayNextYearFromMonth = time.November
)

var dashboardSectionOrder = []string{
	DashboardSectionCompanies,
	DashboardSectionSubscriptions,
	DashboardSectionUsers,
	DashboardSectionTemplates,
	DashboardSectionDisclosures,
	DashboardSectionSystemHealth,
	DashboardSectionGrowth,
}

// dashboardCompaniesGate mirrors the platform companies screen (handler + companyaccess
// authorizePlatformCompanyAdmin): platform operator = platform.cms.view AND (rbac.manage | system.settings).
var dashboardCompaniesGate = [][]string{{"rbac.manage", "system.settings"}}

// dashboardSectionGates lists the permissions required on top of platform.cms.view: every group
// must match (AND), any permission inside a group satisfies it (OR). They mirror the gate of the
// screen each section links to; nil means no extra permission.
var dashboardSectionGates = map[string][][]string{
	DashboardSectionCompanies:     dashboardCompaniesGate,
	DashboardSectionSubscriptions: dashboardCompaniesGate,
	// Same as GET /platform/cms/admin/users.
	DashboardSectionUsers: {{"rbac.manage", "system.settings"}},
	// Same set as disclosure/app requireCMSTemplateRead.
	DashboardSectionTemplates: {{
		"cms.template.read", "cms.template.write", "cms.template.activate",
		"cms.template.archive", "cms.template.config.write", "rbac.manage",
	}},
	DashboardSectionDisclosures:  nil,
	DashboardSectionSystemHealth: {{"system.settings"}},
	// New companies per day is company-directory data: same audience as the companies screen.
	DashboardSectionGrowth: dashboardCompaniesGate,
}

type DashboardCompanies struct {
	Total      int `json:"total"`
	Active     int `json:"active"`
	Inactive   int `json:"inactive"`
	Suspended  int `json:"suspended"`
	Verified   int `json:"verified"`
	Unverified int `json:"unverified"`
	New30d     int `json:"new_30d"`
}

type DashboardSubscriptions struct {
	ByPlan      map[string]int `json:"by_plan"`
	NoPlan      int            `json:"no_plan"`
	Trial       int            `json:"trial"`
	Expiring14d int            `json:"expiring_14d"`
}

type DashboardUsers struct {
	ActiveUsers       int `json:"active_users"`
	ActiveMemberships int `json:"active_memberships"`
	New30d            int `json:"new_30d"`
	WithoutCompany    int `json:"without_company"`
	// Accounts invited or waiting for email verification (not yet usable).
	PendingActivation int `json:"pending_activation"`
}

type DashboardTemplateImports struct {
	Confirmed int `json:"confirmed"`
	Failed    int `json:"failed"`
}

type DashboardTemplates struct {
	Active                int                      `json:"active"`
	NotActive             int                      `json:"not_active"`
	Archived              int                      `json:"archived"`
	OpenDrafts            int                      `json:"open_drafts"`
	ActiveMissingWorkflow int                      `json:"active_missing_workflow"`
	Imports7d             DashboardTemplateImports `json:"imports_7d"`
}

type DashboardDisclosureStatus struct {
	Draft      int `json:"draft"`
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
}

type DashboardOverdueCompany struct {
	CompanyID string `json:"company_id"`
	Name      string `json:"name"`
	Overdue   int    `json:"overdue"`
}

type DashboardDisclosures struct {
	ByStatus            DashboardDisclosureStatus `json:"by_status"`
	Open                int                       `json:"open"`
	Overdue             int                       `json:"overdue"`
	OverdueCompanies    int                       `json:"overdue_companies"`
	DueToday            int                       `json:"due_today"`
	DueNext7d           int                       `json:"due_next_7d"`
	OpenWithoutDue      int                       `json:"open_without_due"`
	OnTimeRate30d       *int                      `json:"on_time_rate_30d"`
	Completed30d        int                       `json:"completed_30d"`
	TopOverdueCompanies []DashboardOverdueCompany `json:"top_overdue_companies"`
}

type DashboardHolidayCalendar struct {
	CurrentYear bool `json:"current_year"`
	NextYear    bool `json:"next_year"`
}

type DashboardSystemHealth struct {
	EmailBacklog        int                      `json:"email_backlog"`
	EmailFailed24h      int                      `json:"email_failed_24h"`
	OutboxStuck         int                      `json:"outbox_stuck"`
	ReminderFailed24h   int                      `json:"reminder_failed_24h"`
	ReminderStuck       int                      `json:"reminder_stuck"`
	PeriodicStaleClaims int                      `json:"periodic_stale_claims"`
	HolidayCalendar     DashboardHolidayCalendar `json:"holiday_calendar"`
}

// DashboardGrowthDay is one HCM calendar day of the growth series.
type DashboardGrowthDay struct {
	Date        string `json:"date"`
	Companies   int    `json:"companies"`
	Users       int    `json:"users"`
	Disclosures int    `json:"disclosures"`
}

type DashboardGrowthTotals struct {
	Companies   int `json:"companies"`
	Users       int `json:"users"`
	Disclosures int `json:"disclosures"`
}

type DashboardGrowth struct {
	Days   []DashboardGrowthDay  `json:"days"`
	Totals DashboardGrowthTotals `json:"totals"`
}

// DashboardDailyCount is one (HCM date, count) row returned by the repository.
type DashboardDailyCount struct {
	Date  string
	Count int
}

// DashboardGrowthCounts holds the sparse per-day counts; days without rows are zero.
type DashboardGrowthCounts struct {
	Companies   []DashboardDailyCount
	Users       []DashboardDailyCount
	Disclosures []DashboardDailyCount
}

// DashboardSubscriptionRow is one company joined with one of its occupying plans
// that has not expired yet. Plan == nil for companies without such a row.
type DashboardSubscriptionRow struct {
	CompanyID string
	Plan      *companyplan.CompanyPlan
}

// DashboardRepository reads the raw aggregates. Implementations must be read-only.
type DashboardRepository interface {
	CompanyStats(ctx context.Context, now time.Time) (DashboardCompanies, error)
	// SubscriptionRows returns one row per (active company, occupying plan with expires_at > now or NULL).
	SubscriptionRows(ctx context.Context, now time.Time) ([]DashboardSubscriptionRow, error)
	UserStats(ctx context.Context, now time.Time) (DashboardUsers, error)
	TemplateStats(ctx context.Context, now time.Time) (DashboardTemplates, error)
	// DisclosureStats counts open obligations against the business date today (YYYY-MM-DD, HCM).
	DisclosureStats(ctx context.Context, today string, now time.Time) (DashboardDisclosures, error)
	SystemHealthStats(ctx context.Context, now time.Time, currentYear int) (DashboardSystemHealth, error)
	// GrowthCounts returns rows created per HCM date from fromDate (YYYY-MM-DD, inclusive).
	GrowthCounts(ctx context.Context, fromDate string, now time.Time) (DashboardGrowthCounts, error)
}

// DashboardSection is the per-section envelope.
type DashboardSection struct {
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	Data      any    `json:"data,omitempty"`
}

type DashboardAttentionItem struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

type DashboardAttention struct {
	Status string                   `json:"status"`
	Items  []DashboardAttentionItem `json:"items"`
}

type DashboardOverview struct {
	GeneratedAt string                      `json:"generated_at"`
	Cached      bool                        `json:"cached"`
	Timezone    string                      `json:"timezone"`
	Sections    map[string]DashboardSection `json:"-"`
	Attention   DashboardAttention          `json:"-"`
}

// SectionsPayload renders sections + attention as the contract's `sections` object.
func (o DashboardOverview) SectionsPayload() map[string]any {
	out := make(map[string]any, len(o.Sections)+1)
	for key, section := range o.Sections {
		out[key] = section
	}
	out["attention"] = o.Attention
	return out
}

type DashboardOverviewRequest struct {
	UserID      string
	Permissions []string
	Refresh     bool
}

type dashboardCacheEntry struct {
	data       any
	computedAt time.Time
}

// DashboardOverviewService computes the overview with a per-section in-process cache.
type DashboardOverviewService struct {
	repo           DashboardRepository
	now            func() time.Time
	loc            *time.Location
	sectionTimeout time.Duration
	cacheTTL       time.Duration
	refreshEvery   time.Duration

	mu          sync.Mutex
	cache       map[string]dashboardCacheEntry
	inflight    map[string]*dashboardFlight
	lastRefresh map[string]time.Time
}

func NewDashboardOverviewService(repo DashboardRepository) *DashboardOverviewService {
	loc, err := time.LoadLocation(DashboardTimezone)
	if err != nil {
		loc = time.FixedZone(DashboardTimezone, 7*60*60)
	}
	return &DashboardOverviewService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
		loc:  loc,
		// Per section; each query also carries MAX_EXECUTION_TIME so MySQL stops on its side.
		sectionTimeout: 3 * time.Second,
		cacheTTL:       60 * time.Second,
		refreshEvery:   10 * time.Second,
		cache:          map[string]dashboardCacheEntry{},
		inflight:       map[string]*dashboardFlight{},
		lastRefresh:    map[string]time.Time{},
	}
}

// WithClock overrides the clock (tests).
func (s *DashboardOverviewService) WithClock(now func() time.Time) *DashboardOverviewService {
	s.now = now
	return s
}

// WithSectionTimeout overrides the per-section timeout (tests).
func (s *DashboardOverviewService) WithSectionTimeout(d time.Duration) *DashboardOverviewService {
	s.sectionTimeout = d
	return s
}

// DashboardSectionAllowed reports whether permissions open the section (platform.cms.view is checked by the caller).
func DashboardSectionAllowed(section string, permissions []string) bool {
	gate, known := dashboardSectionGates[section]
	if !known {
		return false
	}
	for _, group := range gate {
		if !hasAnyDashboardPermission(permissions, group) {
			return false
		}
	}
	return true
}

func hasAnyDashboardPermission(permissions, anyOf []string) bool {
	for _, need := range anyOf {
		for _, have := range permissions {
			if strings.EqualFold(strings.TrimSpace(have), need) {
				return true
			}
		}
	}
	return false
}

func (s *DashboardOverviewService) Overview(ctx context.Context, req DashboardOverviewRequest) DashboardOverview {
	now := s.now().UTC()
	// ?refresh=true accepts data up to refreshEvery old instead of cacheTTL: at most one recompute
	// per section per refreshEvery platform-wide, plus a per-user limit on the bypass itself.
	maxAge := s.cacheTTL
	if req.Refresh && s.allowRefresh(req.UserID, now) {
		maxAge = s.refreshEvery
	}

	type result struct {
		key        string
		section    DashboardSection
		cached     bool
		computedAt time.Time
	}
	allowed := make([]string, 0, len(dashboardSectionOrder))
	sections := make(map[string]DashboardSection, len(dashboardSectionOrder))
	for _, key := range dashboardSectionOrder {
		if DashboardSectionAllowed(key, req.Permissions) {
			allowed = append(allowed, key)
		} else {
			// Forbidden sections never touch the repository.
			sections[key] = DashboardSection{Status: DashboardStatusForbidden}
		}
	}

	results := make(chan result, len(allowed))
	var wg sync.WaitGroup
	for _, key := range allowed {
		if entry, ok := s.cached(key, now, maxAge); ok {
			results <- result{key: key, section: DashboardSection{Status: DashboardStatusOK, Data: entry.data}, cached: true, computedAt: entry.computedAt}
			continue
		}
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			entry, err := s.sectionShared(ctx, key, now)
			if err != nil {
				code := dashboardErrorInternal
				if errors.Is(err, context.DeadlineExceeded) {
					code = dashboardErrorTimeout
				}
				results <- result{key: key, section: DashboardSection{Status: DashboardStatusError, ErrorCode: code}}
				return
			}
			results <- result{key: key, section: DashboardSection{Status: DashboardStatusOK, Data: entry.data}, computedAt: entry.computedAt}
		}(key)
	}
	wg.Wait()
	close(results)

	allCached := len(allowed) > 0
	// generated_at = oldest data served, so "updated at" never overstates freshness.
	generatedAt := now
	for r := range results {
		sections[r.key] = r.section
		if !r.cached {
			allCached = false
		}
		if r.section.Status == DashboardStatusOK && !r.computedAt.IsZero() && r.computedAt.Before(generatedAt) {
			generatedAt = r.computedAt
		}
	}

	return DashboardOverview{
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Cached:      allCached,
		Timezone:    DashboardTimezone,
		Sections:    sections,
		Attention:   DashboardAttention{Status: DashboardStatusOK, Items: buildDashboardAttention(sections, now.In(s.loc))},
	}
}

type dashboardFlight struct {
	done  chan struct{}
	entry dashboardCacheEntry
	err   error
}

// sectionShared computes a section once for all concurrent callers (single flight per section key).
// The computation is detached from the caller's cancellation so a closed tab does not waste the
// work: the result is still cached. A caller that goes away stops waiting.
func (s *DashboardOverviewService) sectionShared(ctx context.Context, key string, now time.Time) (dashboardCacheEntry, error) {
	s.mu.Lock()
	// A flight may have finished between the caller's cache check and here.
	if entry, ok := s.cache[key]; ok && !entry.computedAt.Before(now.Add(-s.refreshEvery)) {
		s.mu.Unlock()
		return entry, nil
	}
	flight, running := s.inflight[key]
	if !running {
		flight = &dashboardFlight{done: make(chan struct{})}
		s.inflight[key] = flight
	}
	s.mu.Unlock()

	if !running {
		detached := context.WithoutCancel(ctx)
		go func() {
			data, err := s.computeSection(detached, key, now)
			entry := dashboardCacheEntry{data: data, computedAt: now}
			s.mu.Lock()
			if err == nil {
				s.cache[key] = entry
			}
			delete(s.inflight, key)
			s.mu.Unlock()
			if err != nil {
				// Detail stays server side; clients only see TIMEOUT|INTERNAL.
				slog.WarnContext(detached, "cms dashboard section failed", "section", key, "error", err)
			}
			flight.entry, flight.err = entry, err
			close(flight.done)
		}()
	}

	select {
	case <-flight.done:
		return flight.entry, flight.err
	case <-ctx.Done():
		return dashboardCacheEntry{}, ctx.Err()
	}
}

func (s *DashboardOverviewService) computeSection(ctx context.Context, key string, now time.Time) (data any, err error) {
	ctx, cancel := context.WithTimeout(ctx, s.sectionTimeout)
	defer cancel()
	defer func() {
		// A panic in one section must not take the whole dashboard down.
		if rec := recover(); rec != nil {
			data, err = nil, errors.New("dashboard section panic")
		}
	}()
	local := now.In(s.loc)
	var out any
	switch key {
	case DashboardSectionCompanies:
		out, err = s.repo.CompanyStats(ctx, now)
	case DashboardSectionSubscriptions:
		var rows []DashboardSubscriptionRow
		rows, err = s.repo.SubscriptionRows(ctx, now)
		if err == nil {
			out = SummarizeDashboardSubscriptions(rows, now)
		}
	case DashboardSectionUsers:
		out, err = s.repo.UserStats(ctx, now)
	case DashboardSectionTemplates:
		out, err = s.repo.TemplateStats(ctx, now)
	case DashboardSectionDisclosures:
		var d DashboardDisclosures
		d, err = s.repo.DisclosureStats(ctx, local.Format("2006-01-02"), now)
		if d.TopOverdueCompanies == nil {
			d.TopOverdueCompanies = []DashboardOverdueCompany{}
		}
		out = d
	case DashboardSectionGrowth:
		from := local.AddDate(0, 0, -(DashboardGrowthDays - 1))
		var counts DashboardGrowthCounts
		counts, err = s.repo.GrowthCounts(ctx, from.Format("2006-01-02"), now)
		if err == nil {
			out = BuildDashboardGrowth(counts, local, DashboardGrowthDays)
		}
	case DashboardSectionSystemHealth:
		out, err = s.repo.SystemHealthStats(ctx, now, local.Year())
	default:
		err = errors.New("unknown dashboard section")
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DashboardOverviewService) cached(key string, now time.Time, maxAge time.Duration) (dashboardCacheEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[key]
	if !ok || now.Sub(entry.computedAt) >= maxAge {
		return dashboardCacheEntry{}, false
	}
	return entry, true
}

// allowRefresh rate-limits ?refresh=true to one cache bypass per user per refreshEvery.
func (s *DashboardOverviewService) allowRefresh(userID string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.TrimSpace(userID)
	if last, ok := s.lastRefresh[key]; ok && now.Sub(last) < s.refreshEvery {
		return false
	}
	s.lastRefresh[key] = now
	// Bound the map: drop entries that can no longer rate-limit anything.
	if len(s.lastRefresh) > 1024 {
		for k, t := range s.lastRefresh {
			if now.Sub(t) >= s.refreshEvery {
				delete(s.lastRefresh, k)
			}
		}
	}
	return true
}

// SummarizeDashboardSubscriptions resolves the effective plan per company with the
// shared companyplan selection rule.
func SummarizeDashboardSubscriptions(rows []DashboardSubscriptionRow, now time.Time) DashboardSubscriptions {
	out := DashboardSubscriptions{ByPlan: map[string]int{
		string(companyplan.PlanCodeFree):       0,
		string(companyplan.PlanCodePremium):    0,
		string(companyplan.PlanCodeEnterprise): 0,
	}}
	order := make([]string, 0)
	byCompany := map[string][]companyplan.CompanyPlan{}
	for _, row := range rows {
		id := strings.TrimSpace(row.CompanyID)
		if id == "" {
			continue
		}
		if _, seen := byCompany[id]; !seen {
			order = append(order, id)
			byCompany[id] = nil
		}
		if row.Plan != nil {
			byCompany[id] = append(byCompany[id], *row.Plan)
		}
	}
	horizon := now.Add(DashboardSubscriptionExpiringWindow)
	for _, id := range order {
		plans := byCompany[id]
		effective := companyplan.SelectEffectivePlan(plans, now)
		if effective == nil {
			out.NoPlan++
			continue
		}
		out.ByPlan[string(effective.Code)]++
		if effective.Status == companyplan.PlanStatusTrial {
			out.Trial++
		}
		if effective.ExpiresAt != nil && !effective.ExpiresAt.After(horizon) &&
			companyplan.SelectEffectivePlan(plans, *effective.ExpiresAt) == nil {
			out.Expiring14d++
		}
	}
	return out
}

var dashboardSeverityRank = map[string]int{
	DashboardSeverityCritical: 0,
	DashboardSeverityWarning:  1,
	DashboardSeverityInfo:     2,
}

// buildDashboardAttention derives the "needs action now" list from sections with status ok only,
// so a forbidden or failed section never leaks a count.
func buildDashboardAttention(sections map[string]DashboardSection, localNow time.Time) []DashboardAttentionItem {
	items := make([]DashboardAttentionItem, 0)
	add := func(code, severity string, count int) {
		if count > 0 {
			items = append(items, DashboardAttentionItem{Code: code, Severity: severity, Count: count})
		}
	}
	if c, ok := okSectionData[DashboardCompanies](sections, DashboardSectionCompanies); ok {
		add("companies_unverified", DashboardSeverityWarning, c.Unverified)
		add("companies_suspended", DashboardSeverityInfo, c.Suspended)
	}
	if sub, ok := okSectionData[DashboardSubscriptions](sections, DashboardSectionSubscriptions); ok {
		add("subscriptions_expiring", DashboardSeverityWarning, sub.Expiring14d)
	}
	if t, ok := okSectionData[DashboardTemplates](sections, DashboardSectionTemplates); ok {
		add("templates_missing_workflow", DashboardSeverityCritical, t.ActiveMissingWorkflow)
		add("templates_open_draft", DashboardSeverityInfo, t.OpenDrafts)
	}
	if d, ok := okSectionData[DashboardDisclosures](sections, DashboardSectionDisclosures); ok {
		add("disclosures_overdue", DashboardSeverityCritical, d.Overdue)
	}
	if h, ok := okSectionData[DashboardSystemHealth](sections, DashboardSectionSystemHealth); ok {
		add("email_failed_24h", DashboardSeverityCritical, h.EmailFailed24h)
		add("outbox_stuck", DashboardSeverityCritical, h.OutboxStuck)
		missing := 0
		if !h.HolidayCalendar.CurrentYear {
			missing++
		}
		if localNow.Month() >= dashboardHolidayNextYearFromMonth && !h.HolidayCalendar.NextYear {
			missing++
		}
		add("holiday_calendar_missing", DashboardSeverityWarning, missing)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return dashboardSeverityRank[items[i].Severity] < dashboardSeverityRank[items[j].Severity]
	})
	return items
}

func okSectionData[T any](sections map[string]DashboardSection, key string) (T, bool) {
	var zero T
	section, ok := sections[key]
	if !ok || section.Status != DashboardStatusOK {
		return zero, false
	}
	data, ok := section.Data.(T)
	return data, ok
}

// BuildDashboardGrowth turns sparse daily counts into a dense series of `days` HCM dates ending today.
func BuildDashboardGrowth(counts DashboardGrowthCounts, localNow time.Time, days int) DashboardGrowth {
	index := func(rows []DashboardDailyCount) map[string]int {
		m := make(map[string]int, len(rows))
		for _, row := range rows {
			m[row.Date] += row.Count
		}
		return m
	}
	companies, users, disclosures := index(counts.Companies), index(counts.Users), index(counts.Disclosures)
	out := DashboardGrowth{Days: make([]DashboardGrowthDay, 0, days)}
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location()).AddDate(0, 0, -(days - 1))
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		day := DashboardGrowthDay{Date: date, Companies: companies[date], Users: users[date], Disclosures: disclosures[date]}
		out.Totals.Companies += day.Companies
		out.Totals.Users += day.Users
		out.Totals.Disclosures += day.Disclosures
		out.Days = append(out.Days, day)
	}
	return out
}
