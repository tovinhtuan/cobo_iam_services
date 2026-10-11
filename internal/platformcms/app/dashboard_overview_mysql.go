package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

// mysqlDashboardRepo reads platform-wide aggregates for the CMS dashboard.
// Read-only COUNT/SUM queries; no schema change. Each method runs under the
// per-section timeout set by DashboardOverviewService.
type mysqlDashboardRepo struct {
	db *sql.DB
}

func NewMySQLDashboardRepository(db *sql.DB) DashboardRepository {
	return &mysqlDashboardRepo{db: db}
}

func (r *mysqlDashboardRepo) CompanyStats(ctx context.Context, now time.Time) (DashboardCompanies, error) {
	var out DashboardCompanies
	err := r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */ COUNT(*),
		       COALESCE(SUM(status = 'active'), 0),
		       COALESCE(SUM(status = 'inactive'), 0),
		       COALESCE(SUM(status = 'suspended'), 0),
		       COALESCE(SUM(verification_status = 'verified'), 0),
		       COALESCE(SUM(verification_status = 'unverified'), 0),
		       COALESCE(SUM(created_at >= ?), 0)
		FROM companies`, now.Add(-30*24*time.Hour)).Scan(
		&out.Total, &out.Active, &out.Inactive, &out.Suspended, &out.Verified, &out.Unverified, &out.New30d)
	if err != nil {
		return DashboardCompanies{}, fmt.Errorf("dashboard companies: %w", err)
	}
	return out, nil
}

func (r *mysqlDashboardRepo) SubscriptionRows(ctx context.Context, now time.Time) ([]DashboardSubscriptionRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */ c.company_id, s.id, s.plan_code, s.status, s.effective_from, s.expires_at
		FROM companies c
		LEFT JOIN company_subscriptions s
		  ON s.company_id = c.company_id
		 AND s.status IN ('ACTIVE', 'TRIAL', 'SUSPENDED')
		 AND (s.expires_at IS NULL OR s.expires_at > ?)
		WHERE c.status = 'active'`, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("dashboard subscriptions: %w", err)
	}
	defer rows.Close()
	out := make([]DashboardSubscriptionRow, 0)
	for rows.Next() {
		var companyID string
		var id, code, status sql.NullString
		var from, expires sql.NullTime
		if err := rows.Scan(&companyID, &id, &code, &status, &from, &expires); err != nil {
			return nil, fmt.Errorf("dashboard subscriptions scan: %w", err)
		}
		row := DashboardSubscriptionRow{CompanyID: companyID}
		if id.Valid && from.Valid {
			plan := companyplan.CompanyPlan{
				ID:            id.String,
				CompanyID:     companyID,
				Code:          companyplan.PlanCode(strings.ToUpper(strings.TrimSpace(code.String))),
				Status:        companyplan.PlanStatus(strings.ToUpper(strings.TrimSpace(status.String))),
				EffectiveFrom: from.Time.UTC(),
			}
			if expires.Valid {
				t := expires.Time.UTC()
				plan.ExpiresAt = &t
			}
			row.Plan = &plan
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dashboard subscriptions rows: %w", err)
	}
	return out, nil
}

func (r *mysqlDashboardRepo) UserStats(ctx context.Context, now time.Time) (DashboardUsers, error) {
	var out DashboardUsers
	err := r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */
		  (SELECT COUNT(*) FROM users u
		    WHERE u.account_status = 'active'
		      AND EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = u.user_id AND m.membership_status = 'active')),
		  (SELECT COUNT(*) FROM memberships WHERE membership_status = 'active'),
		  (SELECT COUNT(*) FROM users WHERE created_at >= ?),
		  (SELECT COUNT(*) FROM users u
		    WHERE u.account_status = 'active'
		      AND NOT EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = u.user_id AND m.membership_status = 'active')),
		  (SELECT COUNT(*) FROM users WHERE account_status IN ('invited', 'pending_email_verification'))`,
		now.Add(-30*24*time.Hour)).Scan(
		&out.ActiveUsers, &out.ActiveMemberships, &out.New30d, &out.WithoutCompany, &out.PendingActivation)
	if err != nil {
		return DashboardUsers{}, fmt.Errorf("dashboard users: %w", err)
	}
	return out, nil
}

// templateNotArchived matches the CMS list portal_state filters (disclosure/infra/mysql ListTypes).
const templateNotArchived = `LOWER(COALESCE(t.status, '')) <> 'archived'`

func (r *mysqlDashboardRepo) TemplateStats(ctx context.Context, now time.Time) (DashboardTemplates, error) {
	var out DashboardTemplates
	// Global templates only (company_id IS NULL), like the CMS templates screen.
	// "Has workflow" mirrors batchLoadActiveWorkflowFlags: active version pinned with >= 1 step.
	err := r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */
		  COALESCE(SUM(CASE WHEN `+templateNotArchived+` AND t.active_version_no > 0 THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN `+templateNotArchived+` AND t.active_version_no = 0 THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN NOT (`+templateNotArchived+`) THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN `+templateNotArchived+` AND t.active_version_no > 0 AND EXISTS (
		      SELECT 1 FROM disclosure_type_versions od
		      WHERE od.type_id = t.type_id AND COALESCE(od.is_released, 0) = 0
		  ) THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN `+templateNotArchived+` AND t.active_version_no > 0 AND NOT EXISTS (
		      SELECT 1 FROM disclosure_type_versions v
		      WHERE v.type_id = t.type_id AND v.version_no = t.active_version_no
		        AND v.workflow_authority_mode = 'TEMPLATE_PINNED'
		        AND JSON_LENGTH(COALESCE(v.workflow_manifest_json, JSON_OBJECT('steps', JSON_ARRAY())), '$.steps') > 0
		  ) THEN 1 ELSE 0 END), 0)
		FROM disclosure_types t
		WHERE t.company_id IS NULL`).Scan(
		&out.Active, &out.NotActive, &out.Archived, &out.OpenDrafts, &out.ActiveMissingWorkflow)
	if err != nil {
		return DashboardTemplates{}, fmt.Errorf("dashboard templates: %w", err)
	}
	since := now.Add(-7 * 24 * time.Hour)
	err = r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */ COALESCE(SUM(status = 'CONFIRMED' AND confirmed_at >= ?), 0),
		       COALESCE(SUM(status = 'CONFIRM_FAILED'), 0)
		FROM cms_template_import_attempts
		WHERE updated_at >= ?`, since, since).Scan(&out.Imports7d.Confirmed, &out.Imports7d.Failed)
	if err != nil {
		return DashboardTemplates{}, fmt.Errorf("dashboard template imports: %w", err)
	}
	return out, nil
}

// dashboardOpenObligationSQL is the platform-wide twin of the deadlinealerts V1 open-obligation
// membership (deadlinealerts/infra/mysql list_rows_membership.go): active template, record still
// draft and not submitted, periodic cycle already open. Restricted to active companies.
// One placeholder: the HCM business date.
const dashboardOpenObligationSQL = `
		FROM disclosure_records dr
		INNER JOIN companies c ON c.company_id = dr.company_id AND c.status = 'active'
` + deadlinealertsapp.ListRowsActiveTemplateSQLJoin + `
		WHERE LOWER(TRIM(dr.status)) = 'draft'
		  AND dr.submitted_at IS NULL
		  AND (
		    NOT EXISTS (SELECT 1 FROM periodic_cycles pc_ir WHERE pc_ir.record_id = dr.record_id)
		    OR EXISTS (
		      SELECT 1 FROM periodic_cycles pc
		      WHERE pc.record_id = dr.record_id
		        AND COALESCE(pc.open_at, pc.cycle_start) IS NOT NULL
		        AND COALESCE(pc.open_at, pc.cycle_start) <= ?
		    )
		  )`

// Record status buckets for the platform activity bar. Terminal set = disclosure/app IsTerminalCompletionStatus.
const (
	dashboardDraftStatuses    = `('draft', 'notstarted', 'not_started')`
	dashboardTerminalStatuses = `('completed', 'done', 'published')`
)

func (r *mysqlDashboardRepo) DisclosureStats(ctx context.Context, today string, now time.Time) (DashboardDisclosures, error) {
	var out DashboardDisclosures
	todayDate, err := time.Parse("2006-01-02", today)
	if err != nil {
		return DashboardDisclosures{}, fmt.Errorf("dashboard disclosures: invalid business date: %w", err)
	}
	in7 := todayDate.AddDate(0, 0, 7).Format("2006-01-02")

	err = r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */ COUNT(*),
		       COALESCE(SUM(dr.planned_date < ?), 0),
		       COUNT(DISTINCT CASE WHEN dr.planned_date < ? THEN dr.company_id END),
		       COALESCE(SUM(dr.planned_date = ?), 0),
		       COALESCE(SUM(dr.planned_date >= ? AND dr.planned_date <= ?), 0),
		       COALESCE(SUM(dr.planned_date IS NULL), 0)
		`+dashboardOpenObligationSQL, today, today, today, today, in7, today).Scan(
		&out.Open, &out.Overdue, &out.OverdueCompanies, &out.DueToday, &out.DueNext7d, &out.OpenWithoutDue)
	if err != nil {
		return DashboardDisclosures{}, fmt.Errorf("dashboard disclosures open: %w", err)
	}

	var total int
	err = r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */ COUNT(*),
		       COALESCE(SUM(LOWER(TRIM(dr.status)) IN `+dashboardDraftStatuses+`), 0),
		       COALESCE(SUM(LOWER(TRIM(dr.status)) IN `+dashboardTerminalStatuses+`), 0)
		FROM disclosure_records dr
		INNER JOIN companies c ON c.company_id = dr.company_id AND c.status = 'active'`).Scan(
		&total, &out.ByStatus.Draft, &out.ByStatus.Completed)
	if err != nil {
		return DashboardDisclosures{}, fmt.Errorf("dashboard disclosures status: %w", err)
	}
	out.ByStatus.InProgress = total - out.ByStatus.Draft - out.ByStatus.Completed

	// On-time = completion day (HCM, UTC+7 without DST) <= planned_date (approved KPI #6).
	var onTime int
	err = r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */ COUNT(*), COALESCE(SUM(DATE(DATE_ADD(dr.completed_at, INTERVAL 7 HOUR)) <= dr.planned_date), 0)
		FROM disclosure_records dr
		INNER JOIN companies c ON c.company_id = dr.company_id AND c.status = 'active'
		WHERE LOWER(TRIM(dr.status)) IN `+dashboardTerminalStatuses+`
		  AND dr.completed_at >= ?
		  AND dr.planned_date IS NOT NULL`, now.Add(-30*24*time.Hour).UTC()).Scan(&out.Completed30d, &onTime)
	if err != nil {
		return DashboardDisclosures{}, fmt.Errorf("dashboard disclosures on-time: %w", err)
	}
	if out.Completed30d > 0 {
		rate := (onTime*100 + out.Completed30d/2) / out.Completed30d
		out.OnTimeRate30d = &rate
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */ dr.company_id, c.company_name, COUNT(*) AS overdue
		`+dashboardOpenObligationSQL+`
		  AND dr.planned_date < ?
		GROUP BY dr.company_id, c.company_name
		ORDER BY overdue DESC, c.company_name ASC, dr.company_id ASC
		LIMIT ?`, today, today, DashboardTopOverdueLimit)
	if err != nil {
		return DashboardDisclosures{}, fmt.Errorf("dashboard disclosures top overdue: %w", err)
	}
	defer rows.Close()
	out.TopOverdueCompanies = make([]DashboardOverdueCompany, 0, DashboardTopOverdueLimit)
	for rows.Next() {
		var item DashboardOverdueCompany
		if err := rows.Scan(&item.CompanyID, &item.Name, &item.Overdue); err != nil {
			return DashboardDisclosures{}, fmt.Errorf("dashboard disclosures top overdue scan: %w", err)
		}
		out.TopOverdueCompanies = append(out.TopOverdueCompanies, item)
	}
	if err := rows.Err(); err != nil {
		return DashboardDisclosures{}, fmt.Errorf("dashboard disclosures top overdue rows: %w", err)
	}
	return out, nil
}

// SystemHealthStats reuses the Prometheus collector predicates
// (notification|reminder|disclosure/infra/observe) with the dashboard thresholds.
func (r *mysqlDashboardRepo) SystemHealthStats(ctx context.Context, now time.Time, currentYear int) (DashboardSystemHealth, error) {
	var out DashboardSystemHealth
	var holidayCurrent, holidayNext int
	stuckBefore := now.Add(-DashboardStuckThreshold).UTC()
	err := r.db.QueryRowContext(ctx, `
		SELECT /*+ MAX_EXECUTION_TIME(2000) */
		  (SELECT COUNT(*) FROM email_notifications WHERE status IN ('pending', 'sending', 'retry')),
		  (SELECT COUNT(*) FROM email_notifications WHERE status = 'failed_permanent' AND updated_at >= ?),
		  (SELECT COUNT(*) FROM outbox_events WHERE status = 'processing' AND available_at < ?),
		  (SELECT COUNT(*) FROM reminder_occurrences WHERE status = 'FAILED' AND updated_at >= ?
		    AND (last_error_code IS NULL OR last_error_code <> 'COMPANY_INACTIVE')),
		  (SELECT COUNT(*) FROM reminder_occurrences WHERE status = 'DISPATCHING' AND updated_at < ?),
		  (SELECT COUNT(*) FROM periodic_cycles
		    WHERE record_id IS NULL AND materialization_state = 'CLAIMED'
		      AND materialized_at < NOW(3) - INTERVAL ? SECOND),
		  (SELECT COUNT(*) FROM holiday_calendars WHERE calendar_year = ?),
		  (SELECT COUNT(*) FROM holiday_calendars WHERE calendar_year = ?)`,
		// periodic_cycles.materialized_at is written with NOW(3): compare on the DB clock like the collector.
		now.Add(-24*time.Hour).UTC(), stuckBefore, now.Add(-24*time.Hour).UTC(), stuckBefore, int(DashboardStuckThreshold/time.Second),
		currentYear, currentYear+1).Scan(
		&out.EmailBacklog, &out.EmailFailed24h, &out.OutboxStuck, &out.ReminderFailed24h, &out.ReminderStuck,
		&out.PeriodicStaleClaims, &holidayCurrent, &holidayNext)
	if err != nil {
		return DashboardSystemHealth{}, fmt.Errorf("dashboard system health: %w", err)
	}
	out.HolidayCalendar = DashboardHolidayCalendar{CurrentYear: holidayCurrent > 0, NextYear: holidayNext > 0}
	return out, nil
}

// GrowthCounts buckets created_at by HCM calendar date. created_at is TIMESTAMP read in the
// session time zone, which is UTC for the API (same assumption as the on-time query): +7 h = HCM.
func (r *mysqlDashboardRepo) GrowthCounts(ctx context.Context, fromDate string, now time.Time) (DashboardGrowthCounts, error) {
	from, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return DashboardGrowthCounts{}, fmt.Errorf("dashboard growth: invalid from date: %w", err)
	}
	// HCM midnight of fromDate expressed in UTC: lets the range use the created_at index.
	sinceUTC := from.Add(-7 * time.Hour)
	var out DashboardGrowthCounts
	for _, q := range []struct {
		table string
		dest  *[]DashboardDailyCount
	}{
		{"companies", &out.Companies},
		{"users", &out.Users},
		{"disclosure_records", &out.Disclosures},
	} {
		rows, err := r.db.QueryContext(ctx, `
			SELECT /*+ MAX_EXECUTION_TIME(2000) */ DATE_FORMAT(DATE_ADD(created_at, INTERVAL 7 HOUR), '%Y-%m-%d') AS d, COUNT(*)
			FROM `+q.table+`
			WHERE created_at >= ?
			GROUP BY d`, sinceUTC)
		if err != nil {
			return DashboardGrowthCounts{}, fmt.Errorf("dashboard growth %s: %w", q.table, err)
		}
		for rows.Next() {
			var row DashboardDailyCount
			if err := rows.Scan(&row.Date, &row.Count); err != nil {
				rows.Close()
				return DashboardGrowthCounts{}, fmt.Errorf("dashboard growth %s scan: %w", q.table, err)
			}
			*q.dest = append(*q.dest, row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return DashboardGrowthCounts{}, fmt.Errorf("dashboard growth %s rows: %w", q.table, err)
		}
	}
	return out, nil
}
