package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/deadlineengine"
	personalopsapp "github.com/cobo/cobo_iam_services/internal/personalops/app"
)

type Repository struct {
	db       *sql.DB
	holidays deadlineengine.NonTradingDayChecker

	dayTypeColMu     sync.RWMutex
	dayTypeColCached bool
	dayTypeColOK     bool
}

type Option func(*Repository)

// WithHolidays wires the shared working-day holiday checker.
func WithHolidays(h deadlineengine.NonTradingDayChecker) Option {
	return func(r *Repository) { r.holidays = h }
}

func NewRepository(db *sql.DB, opts ...Option) *Repository {
	r := &Repository{db: db}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func toAny(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

// ListMineRecords returns distinct disclosure records matching mine semantics
// for the given membership IDs. Does NOT expand via rbac.manage / department visibility.
func (r *Repository) ListMineRecords(ctx context.Context, membershipIDs []string) ([]personalopsapp.MineRecord, error) {
	if len(membershipIDs) == 0 {
		return nil, nil
	}
	ph := placeholders(len(membershipIDs))
	args := toAny(membershipIDs)
	// Duplicate args for both EXISTS clauses.
	args = append(args, toAny(membershipIDs)...)

	q := fmt.Sprintf(`
SELECT
  dr.company_id,
  c.company_name,
  dr.record_id,
  COALESCE(dr.title, ''),
  dr.status,
  COALESCE(DATE_FORMAT(dr.planned_date, '%%Y-%%m-%%d'), ''),
  CASE WHEN dac.record_id IS NOT NULL THEN 1 ELSE 0 END AS confirmed,
  CASE WHEN EXISTS (
    SELECT 1 FROM workflow_tasks wt
    INNER JOIN workflow_instances wi
      ON wi.workflow_instance_id = wt.workflow_instance_id AND wi.company_id = wt.company_id
    WHERE wi.record_id = dr.record_id
      AND wt.company_id = dr.company_id
      AND wt.assignee_membership_id IN (%s)
      AND LOWER(TRIM(wt.status)) NOT IN ('completed','done','cancelled','skipped')
  ) THEN 1 ELSE 0 END AS via_task,
  CASE WHEN EXISTS (
    SELECT 1 FROM assignments a
    WHERE a.company_id = dr.company_id
      AND a.resource_type = 'disclosure_record'
      AND a.resource_id = dr.record_id
      AND a.assignee_type = 'membership'
      AND a.assignee_ref_id IN (%s)
      AND LOWER(TRIM(a.status)) = 'active'
  ) THEN 1 ELSE 0 END AS via_asgn,
  dr.completed_at
FROM disclosure_records dr
INNER JOIN companies c ON c.company_id = dr.company_id
LEFT JOIN deadline_alert_confirmations dac
  ON dac.company_id = dr.company_id AND dac.record_id = dr.record_id
WHERE LOWER(TRIM(dr.status)) <> 'draft'
  AND (
    EXISTS (
      SELECT 1 FROM workflow_tasks wt
      INNER JOIN workflow_instances wi
        ON wi.workflow_instance_id = wt.workflow_instance_id AND wi.company_id = wt.company_id
      WHERE wi.record_id = dr.record_id
        AND wt.company_id = dr.company_id
        AND wt.assignee_membership_id IN (%s)
        AND LOWER(TRIM(wt.status)) NOT IN ('completed','done','cancelled','skipped')
    )
    OR EXISTS (
      SELECT 1 FROM assignments a
      WHERE a.company_id = dr.company_id
        AND a.resource_type = 'disclosure_record'
        AND a.resource_id = dr.record_id
        AND a.assignee_type = 'membership'
        AND a.assignee_ref_id IN (%s)
        AND LOWER(TRIM(a.status)) = 'active'
    )
    OR (
      dr.completed_at IS NOT NULL
      AND LOWER(TRIM(dr.status)) IN ('completed','done','published')
      AND (
        EXISTS (
          SELECT 1 FROM workflow_tasks wt
          INNER JOIN workflow_instances wi
            ON wi.workflow_instance_id = wt.workflow_instance_id AND wi.company_id = wt.company_id
          WHERE wi.record_id = dr.record_id
            AND wt.company_id = dr.company_id
            AND wt.assignee_membership_id IN (%s)
        )
        OR EXISTS (
          SELECT 1 FROM assignments a
          WHERE a.company_id = dr.company_id
            AND a.resource_type = 'disclosure_record'
            AND a.resource_id = dr.record_id
            AND a.assignee_type = 'membership'
            AND a.assignee_ref_id IN (%s)
        )
      )
    )
  )
`, ph, ph, ph, ph, ph, ph)

	// 6x membership ID args for the six IN clauses.
	args = toAny(membershipIDs)
	args = append(args, toAny(membershipIDs)...)
	args = append(args, toAny(membershipIDs)...)
	args = append(args, toAny(membershipIDs)...)
	args = append(args, toAny(membershipIDs)...)
	args = append(args, toAny(membershipIDs)...)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]personalopsapp.MineRecord, 0)
	for rows.Next() {
		var rec personalopsapp.MineRecord
		var confirmed, viaTask, viaAsgn int
		var completedAt sql.NullTime
		if err := rows.Scan(
			&rec.CompanyID,
			&rec.CompanyName,
			&rec.RecordID,
			&rec.Title,
			&rec.RecordStatus,
			&rec.PlannedDate,
			&confirmed,
			&viaTask,
			&viaAsgn,
			&completedAt,
		); err != nil {
			return nil, err
		}
		rec.Confirmed = confirmed == 1
		rec.MatchedViaTask = viaTask == 1
		rec.MatchedViaAsgn = viaAsgn == 1
		if completedAt.Valid {
			t := completedAt.Time.UTC()
			rec.CompletedAt = &t
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *Repository) ListMineOpenTasks(ctx context.Context, membershipIDs []string, limit int) ([]personalopsapp.MineTask, error) {
	if len(membershipIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	ph := placeholders(len(membershipIDs))
	args := toAny(membershipIDs)
	args = append(args, limit)

	q := fmt.Sprintf(`
SELECT
  wt.task_id,
  wt.company_id,
  c.company_name,
  wt.assignee_membership_id,
  wi.record_id,
  COALESCE(dr.title, ''),
  COALESCE(wt.step_code, ''),
  wt.status,
  COALESCE(DATE_FORMAT(dr.planned_date, '%%Y-%%m-%%d'), ''),
  dr.status
FROM workflow_tasks wt
INNER JOIN workflow_instances wi
  ON wi.workflow_instance_id = wt.workflow_instance_id AND wi.company_id = wt.company_id
INNER JOIN disclosure_records dr
  ON dr.company_id = wi.company_id AND dr.record_id = wi.record_id
INNER JOIN companies c ON c.company_id = wt.company_id
WHERE wt.assignee_membership_id IN (%s)
  AND LOWER(TRIM(wt.status)) = 'pending'
  AND LOWER(TRIM(dr.status)) <> 'draft'
ORDER BY wt.task_id ASC
LIMIT ?
`, ph)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]personalopsapp.MineTask, 0)
	for rows.Next() {
		var t personalopsapp.MineTask
		if err := rows.Scan(
			&t.TaskID,
			&t.CompanyID,
			&t.CompanyName,
			&t.MembershipID,
			&t.RecordID,
			&t.Title,
			&t.StepCode,
			&t.TaskStatus,
			&t.PlannedDate,
			&t.RecordStatus,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) ListApprovedAdHocDues(ctx context.Context, companyIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(companyIDs) == 0 {
		return out, nil
	}
	includeDayType, err := r.hasProposedDeadlineDayTypeColumn(ctx)
	if err != nil {
		return nil, err
	}
	ph := placeholders(len(companyIDs))
	dayTypeCol := ""
	if includeDayType {
		dayTypeCol = ",\n  proposed_deadline_day_type"
	}
	q := fmt.Sprintf(`
SELECT
  company_id,
  record_id,
  final_deadline_date,
  proposed_t0_date,
  proposed_deadline_days,
  proposed_deadline_date%s,
  updated_at
FROM ad_hoc_proposals
WHERE company_id IN (%s)
  AND LOWER(TRIM(status)) = 'approved'
  AND record_id IS NOT NULL
  AND TRIM(record_id) <> ''
ORDER BY updated_at DESC
`, dayTypeCol, ph)
	rows, err := r.db.QueryContext(ctx, q, toAny(companyIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var companyID, recordID string
		var finalDue, proposedT0, proposedDue sql.NullTime
		var proposedDays sql.NullInt64
		var dayTypeRaw sql.NullString
		var updatedAt sql.NullTime
		var scanErr error
		if includeDayType {
			scanErr = rows.Scan(&companyID, &recordID, &finalDue, &proposedT0, &proposedDays, &proposedDue, &dayTypeRaw, &updatedAt)
		} else {
			scanErr = rows.Scan(&companyID, &recordID, &finalDue, &proposedT0, &proposedDays, &proposedDue, &updatedAt)
		}
		if scanErr != nil {
			return nil, scanErr
		}
		key := companyID + "|" + recordID
		if _, exists := out[key]; exists {
			continue // already have latest (ORDER BY updated_at DESC)
		}
		due, dueErr := adhocapp.FormatProposalDueDate(ctx, adhocapp.ProposalDueInput{
			FinalDeadlineDate:    finalDue,
			ProposedT0Date:       proposedT0,
			ProposedDeadlineDays: proposedDays,
			ProposedDeadlineDate: proposedDue,
			DayType:              adhocapp.ParsePersistedDeadlineDayType(dayTypeRaw),
		}, r.holidays)
		if dueErr != nil {
			continue
		}
		if due != "" {
			out[key] = due
		}
	}
	return out, rows.Err()
}

func (r *Repository) hasProposedDeadlineDayTypeColumn(ctx context.Context) (bool, error) {
	r.dayTypeColMu.RLock()
	if r.dayTypeColCached {
		ok := r.dayTypeColOK
		r.dayTypeColMu.RUnlock()
		return ok, nil
	}
	r.dayTypeColMu.RUnlock()

	r.dayTypeColMu.Lock()
	defer r.dayTypeColMu.Unlock()
	if r.dayTypeColCached {
		return r.dayTypeColOK, nil
	}
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'ad_hoc_proposals'
		  AND column_name = 'proposed_deadline_day_type'
	`).Scan(&count)
	if err != nil {
		return false, err
	}
	r.dayTypeColOK = count > 0
	r.dayTypeColCached = true
	return r.dayTypeColOK, nil
}
