-- 0155: forward fix for 0098_adhoc_multi_reviewer, which replaces it in run_dev_migrations.sh.
--
-- 0098 was never listed in the runner, and its backfill selects ad_hoc_proposals.id (the key
-- is proposal_id), so it fails with ERROR 1054 after creating the tables. DEV has both tables
-- but no schema_migrations row for 0098. 0098 is kept unchanged; this file:
--   1. creates the two tables when missing (same DDL as 0098; no-op where they exist);
--   2. runs the corrected backfill.
--
-- Backfill: a legacy single-reviewer proposal (process_controller_id set) gets its process
-- controller as the only reviewer, as 0098 intended. Current code never writes
-- process_controller_id (adhoc/app/service.go CreateProposal), so only legacy rows match.
-- A proposal that already has reviewer rows is skipped: adding a reviewer would change the
-- number of approvals it needs.
--
-- Idempotent: rerunning inserts nothing. Additive only; no existing row is changed.

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS ad_hoc_proposal_reviewers (
  proposal_id    VARCHAR(64) NOT NULL,
  company_id     VARCHAR(64) NOT NULL,
  membership_id  VARCHAR(64) NOT NULL,
  sort_order     INT NULL,
  created_at     DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (proposal_id, membership_id),
  KEY idx_adhoc_rev_company (company_id, proposal_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ad_hoc_proposal_approvals (
  proposal_id           VARCHAR(64) NOT NULL,
  company_id            VARCHAR(64) NOT NULL,
  membership_id         VARCHAR(64) NOT NULL,
  approved_at           DATETIME(3) NOT NULL,
  final_t0_date         DATE NULL,
  final_deadline_date   DATE NULL,
  adjustment_note       TEXT NULL,
  comment               TEXT NULL,
  PRIMARY KEY (proposal_id, membership_id),
  KEY idx_adhoc_appr_company (company_id, proposal_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO ad_hoc_proposal_reviewers (proposal_id, company_id, membership_id, sort_order, created_at)
SELECT p.proposal_id, p.company_id, p.process_controller_id, 0, NOW(3)
FROM ad_hoc_proposals p
WHERE p.process_controller_id IS NOT NULL AND p.process_controller_id <> ''
  AND NOT EXISTS (
    SELECT 1 FROM ad_hoc_proposal_reviewers r WHERE r.proposal_id = p.proposal_id
  );
