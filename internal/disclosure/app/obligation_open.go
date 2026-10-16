package app

import "strings"

// ObligationOpenSQL is the shared ObligationOpen(record) predicate (business contract v1.2 §2.2)
// for the alerts tab, the Portal dashboard snapshot, the platform CMS dashboard, the CMS archive
// impact, "Việc của tôi" (personalops) and deadline reminders. It is one AND-clause on
// disclosure_records dr, to follow a WHERE condition.
//
// Periodic / custom / manual: Draft + submitted_at IS NULL, and
// Periodic AlertWindow: EXISTS cycle with COALESCE(open_at, cycle_start) <= :todayHCM
//
//	(open_at NULL → cycle_start = LEGACY_COMPATIBILITY_FALLBACK only).
//
// Irregular: NOT EXISTS cycle → no OpenAt gate.
// Malformed cycle (both dates NULL): EXISTS branch fails → excluded.
// Ad-hoc (PO Q-12 a′, AC-11): its proposal is approved and the record has not reached
// Approved/Published/Completed/Done, although finalize auto-submits it (PendingReview).
//
// The template's state is NOT part of it: an obligation stays open after its template is archived
// (OQ-006, PO decision Q1-B of 16/10/2026). Template state only stops new cycles and new records.
//
// Requires one bind argument: todayHCM as YYYY-MM-DD (Asia/Ho_Chi_Minh business date).
// EXISTS (not JOIN) avoids row duplication when multiple cycles share a record_id.
const ObligationOpenSQL = `
		  AND (
		    (
		      LOWER(TRIM(dr.status)) = 'draft'
		      AND dr.submitted_at IS NULL
		      AND (
		        NOT EXISTS (
		          SELECT 1 FROM periodic_cycles pc_ir
		          WHERE pc_ir.record_id = dr.record_id
		        )
		        OR EXISTS (
		          SELECT 1 FROM periodic_cycles pc
		          WHERE pc.record_id = dr.record_id
		            AND COALESCE(pc.open_at, pc.cycle_start) IS NOT NULL
		            AND COALESCE(pc.open_at, pc.cycle_start) <= ?
		        )
		      )
		    )
		    OR (
		      LOWER(TRIM(dr.status)) NOT IN ('approved', 'published', 'completed', 'done')
		      AND EXISTS (
		        SELECT 1 FROM ad_hoc_proposals ahp
		        WHERE ahp.company_id = dr.company_id
		          AND ahp.record_id = dr.record_id
		          AND ahp.status = 'approved'
		      )
		    )
		  )`

// TemplateArchivedSQL is true when the template joined as dt is archived: a global template
// archived in the CMS (status) or a company template archived through its lifecycle
// (review_status). active_version_no = 0 alone is not archived: it also means never activated or
// restored to draft.
const TemplateArchivedSQL = `(LOWER(COALESCE(dt.status, '')) = 'archived' OR (dt.company_id IS NOT NULL AND LOWER(COALESCE(dt.review_status, '')) = 'archived'))`

// TemplateArchived is the in-process equivalent of TemplateArchivedSQL.
func TemplateArchived(status, companyID, reviewStatus string) bool {
	if strings.EqualFold(strings.TrimSpace(status), "archived") {
		return true
	}
	return strings.TrimSpace(companyID) != "" && strings.EqualFold(strings.TrimSpace(reviewStatus), "archived")
}
