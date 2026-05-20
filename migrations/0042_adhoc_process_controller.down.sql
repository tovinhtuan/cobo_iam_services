DROP INDEX IF EXISTS idx_adhoc_proposals_controller ON ad_hoc_proposals;

ALTER TABLE ad_hoc_proposals
  DROP COLUMN IF EXISTS process_controller_id;
