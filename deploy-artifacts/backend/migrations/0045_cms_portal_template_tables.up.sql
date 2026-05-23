-- 0045: CMS Portal Template — New tables for D-A, D-B, D-C decisions
-- Creates: template_display_groups (many-to-many), global_workflows,
--          global_workflow_steps (step_deadline VARCHAR), deadline_rule_catalog
-- Adds: is_mandatory to disclosure_types
-- Ref: CMS-DBA-001, CMS-DBA-003, decision-record-po-answers-2026-05-23.md

SET NAMES utf8mb4;

-- D-A: Many-to-many junction table (template ↔ display_group)
CREATE TABLE IF NOT EXISTS template_display_groups (
  template_id        VARCHAR(64)  NOT NULL,
  display_group_code VARCHAR(64)  NOT NULL,
  display_order      INT          NOT NULL DEFAULT 0,
  PRIMARY KEY (template_id, display_group_code),
  KEY idx_tdg_group (display_group_code),
  CONSTRAINT fk_tdg_template FOREIGN KEY (template_id)
    REFERENCES disclosure_types(type_id) ON DELETE CASCADE,
  CONSTRAINT fk_tdg_group FOREIGN KEY (display_group_code)
    REFERENCES disclosure_display_groups(display_group_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Global workflow header (one per template, can version later)
CREATE TABLE IF NOT EXISTS global_workflows (
  workflow_id  VARCHAR(64)  NOT NULL,
  type_id      VARCHAR(64)  NOT NULL,
  version_no   INT          NOT NULL DEFAULT 1,
  is_active    TINYINT(1)   NOT NULL DEFAULT 1,
  created_by   VARCHAR(64)  NOT NULL DEFAULT 'system',
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (workflow_id),
  KEY idx_gw_type_active (type_id, is_active),
  CONSTRAINT fk_gw_type FOREIGN KEY (type_id)
    REFERENCES disclosure_types(type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- D-C: Global workflow steps with cumulative deadline string
CREATE TABLE IF NOT EXISTS global_workflow_steps (
  step_id        VARCHAR(64)   NOT NULL,
  workflow_id    VARCHAR(64)   NOT NULL,
  type_id        VARCHAR(64)   NOT NULL,
  display_order  INT           NOT NULL DEFAULT 0,
  stage_name     VARCHAR(255)  NOT NULL,
  department     VARCHAR(255)  NOT NULL DEFAULT '',
  step_deadline  VARCHAR(20)   NOT NULL,
  created_at     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (step_id),
  KEY idx_gws_workflow (workflow_id),
  KEY idx_gws_type (type_id),
  CONSTRAINT fk_gws_workflow FOREIGN KEY (workflow_id)
    REFERENCES global_workflows(workflow_id) ON DELETE CASCADE,
  CONSTRAINT fk_gws_type FOREIGN KEY (type_id)
    REFERENCES disclosure_types(type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- D-B: Deadline rule format type catalog
CREATE TABLE IF NOT EXISTS deadline_rule_catalog (
  code       VARCHAR(20)   NOT NULL,
  label_vi   VARCHAR(255)  NOT NULL,
  pattern    VARCHAR(100)  NOT NULL,
  input_type VARCHAR(20)   NOT NULL,
  is_active  TINYINT(1)    NOT NULL DEFAULT 1,
  PRIMARY KEY (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Seed 2 format types (T+N and dd/mm)
INSERT INTO deadline_rule_catalog (code, label_vi, pattern, input_type) VALUES
  ('T+N',   'Trong vòng N ngày kể từ ngày sự kiện', '^T\\+\\d+$',       'number'),
  ('dd/mm', 'Ngày dd/mm hàng năm',                  '^\\d{2}/\\d{2}$', 'date_dm')
ON DUPLICATE KEY UPDATE
  label_vi   = VALUES(label_vi),
  pattern    = VALUES(pattern),
  input_type = VALUES(input_type);

-- Add is_mandatory to disclosure_types (for periodic templates)
SET @col_exists = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'disclosure_types'
    AND column_name  = 'is_mandatory'
);
SET @sql = IF(
  @col_exists = 0,
  'ALTER TABLE disclosure_types ADD COLUMN is_mandatory TINYINT(1) NOT NULL DEFAULT 0 AFTER status',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
