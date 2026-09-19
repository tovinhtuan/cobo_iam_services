-- 0140: workflow_step_comment_mentions — V1.1 @Mention persistence (Phase 2).
-- No FK (matches 0139 no-FK pattern). Indexes lead with company_id.
-- Soft-deleted parent comments: mention rows may remain; LIST omits via comment join.
-- PRODUCTION_ROLLBACK=forward_fix_or_flag_off
-- DEV_DOWN=DROP_TABLE_ALLOWED_WITH_CONFIRMATION

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS workflow_step_comment_mentions (
  id VARCHAR(36) NOT NULL,
  company_id VARCHAR(36) NOT NULL,
  comment_id VARCHAR(36) NOT NULL,
  mentioned_membership_id VARCHAR(36) NOT NULL,
  start_offset INT NOT NULL,
  end_offset INT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_wscm_company_comment
    (company_id, comment_id),
  KEY idx_wscm_company_mentioned_created
    (company_id, mentioned_membership_id, created_at),
  KEY idx_wscm_company_id
    (company_id, id)
);
