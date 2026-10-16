-- 0155 down: intentionally a no-op (forward-only).
-- Do NOT drop ad_hoc_proposal_reviewers / ad_hoc_proposal_approvals: they hold live votes,
-- the adhoc code requires them, and on DEV they predate 0155.
-- Do NOT delete the backfilled reviewer rows: without them a legacy proposal has no reviewer
-- and nobody can approve it.
-- Rolling back the application does not need this: older binaries read the same tables.

SELECT 1;
