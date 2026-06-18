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
SELECT id, company_id, process_controller_id, 0, NOW(3)
FROM ad_hoc_proposals
WHERE process_controller_id IS NOT NULL AND process_controller_id <> '';
