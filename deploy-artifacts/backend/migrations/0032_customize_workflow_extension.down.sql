SET NAMES utf8mb4;

DROP TABLE IF EXISTS ad_hoc_proposals;
DROP TABLE IF EXISTS workflow_step_milestones;

ALTER TABLE workflow_instances
  DROP COLUMN IF EXISTS workflow_source,
  DROP COLUMN IF EXISTS t0_policy,
  DROP COLUMN IF EXISTS t0_date,
  DROP COLUMN IF EXISTS snapshot_json;
