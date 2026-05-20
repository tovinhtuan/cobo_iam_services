SET NAMES utf8mb4;

-- ============================================================
-- workflow_instances: add snapshot + T0 fields
-- ============================================================
ALTER TABLE workflow_instances
  ADD COLUMN snapshot_json     JSON         NULL AFTER current_step_code,
  ADD COLUMN t0_date           DATE         NULL AFTER snapshot_json,
  ADD COLUMN t0_policy         VARCHAR(32)  NULL AFTER t0_date,
  ADD COLUMN workflow_source   VARCHAR(32)  NULL AFTER t0_policy;
-- workflow_source values: 'system_template' | 'company_override'
-- t0_policy values: 'system_date' | 'event_date' | 'user_defined'

-- ============================================================
-- workflow_step_milestones: per-step reminder schedule rows
-- 5 milestone types per step (before_start_5d, before_start_3d,
-- before_start_1d, step_start, step_end)
-- ============================================================
CREATE TABLE IF NOT EXISTS workflow_step_milestones (
  id                    BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,
  milestone_id          VARCHAR(80)        NOT NULL,
  company_id            VARCHAR(64)        NOT NULL,
  workflow_instance_id  VARCHAR(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  step_id               VARCHAR(64)        NOT NULL,
  step_order            INT UNSIGNED       NOT NULL DEFAULT 0,
  milestone_type        ENUM(
                          'before_start_5d',
                          'before_start_3d',
                          'before_start_1d',
                          'step_start',
                          'step_end'
                        )                  NOT NULL,
  scheduled_date        DATE               NOT NULL,
  reminder_sent         TINYINT(1)         NOT NULL DEFAULT 0,
  reminder_occurrence_id VARCHAR(80)       NULL,
  created_at            DATETIME(3)        NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_wsm_milestone_id (milestone_id),
  KEY idx_wsm_instance (workflow_instance_id),
  KEY idx_wsm_dispatch (company_id, reminder_sent, scheduled_date),
  CONSTRAINT fk_wsm_instance FOREIGN KEY (workflow_instance_id)
    REFERENCES workflow_instances(workflow_instance_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- ad_hoc_proposals: per-type ad-hoc submission workflow
-- State machine:
--   ad_hoc_draft → pending_focal_approval → pending_admin_approval
--   → approved → (triggers workflow_instance creation)
--   terminal: rejected | cancelled
-- ============================================================
CREATE TABLE IF NOT EXISTS ad_hoc_proposals (
  proposal_id              VARCHAR(64)    NOT NULL,
  company_id               VARCHAR(64)    NOT NULL,
  type_id                  VARCHAR(64) COLLATE utf8mb4_0900_ai_ci NOT NULL,
  status                   VARCHAR(32)    NOT NULL DEFAULT 'ad_hoc_draft',
  proposed_workflow_json   JSON           NOT NULL,
  proposed_t0_date         DATE           NULL,
  proposed_deadline_date   DATE           NULL,
  change_note              TEXT           NULL,
  focal_approved_by        VARCHAR(64)    NULL,
  focal_approved_at        DATETIME(3)    NULL,
  admin_approved_by        VARCHAR(64)    NULL,
  admin_approved_at        DATETIME(3)    NULL,
  rejected_by              VARCHAR(64)    NULL,
  rejected_at              DATETIME(3)    NULL,
  reject_reason            TEXT           NULL,
  record_id                VARCHAR(36)    NULL,
  workflow_instance_id     VARCHAR(36)    NULL,
  created_by               VARCHAR(64)    NOT NULL,
  created_at               DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at               DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (proposal_id),
  KEY idx_adhoc_company_status (company_id, status),
  KEY idx_adhoc_type (company_id, type_id),
  CONSTRAINT fk_adhoc_type FOREIGN KEY (type_id)
    REFERENCES disclosure_types(type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
