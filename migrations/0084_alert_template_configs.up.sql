-- Phase: Worker Send-Alert Workflow
-- Creates alert_template_configs to map (disclosure type, alert kind) → email template key.
-- alert_kind values: 'deadline' | 'workflow_step'
CREATE TABLE alert_template_configs (
  id           BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,
  type_id      VARCHAR(64)        NOT NULL,
  alert_kind   VARCHAR(32)        NOT NULL,
  template_key VARCHAR(128)       NOT NULL,
  enabled      TINYINT(1)         NOT NULL DEFAULT 1,
  created_by   VARCHAR(64)        NOT NULL,
  created_at   DATETIME(3)        NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)        NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_alert_cfg (type_id, alert_kind)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
