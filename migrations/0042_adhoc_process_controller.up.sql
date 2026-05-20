-- Add process_controller_id to ad_hoc_proposals.
-- This column stores the membership_id of the person the creator designates
-- as the "Kiểm soát quy trình" (Process Controller) who performs the final approval.
-- Nullable to allow zero-downtime rollout; service layer enforces non-null on create.

ALTER TABLE ad_hoc_proposals
  ADD COLUMN process_controller_id VARCHAR(64) NULL
    COMMENT 'Membership ID của người được creator chỉ định kiểm soát quy trình'
    AFTER created_by;

CREATE INDEX idx_adhoc_proposals_controller
  ON ad_hoc_proposals (company_id, process_controller_id);
