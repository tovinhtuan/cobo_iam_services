-- 0056: DBA-001 — Company-defined template lifecycle fields
-- Adds review_status lifecycle column to disclosure_types for company-scoped templates.
-- Values: draft | in_review | published | rejected | archived
-- Ref: portal-template-sprint-ready-execution-board-2026-05-23.md § DBA-001

SET NAMES utf8mb4;

-- Add review_status to disclosure_types (nullable — only meaningful for company-scoped templates)
SET @col_exists = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'disclosure_types'
    AND column_name  = 'review_status'
);
SET @sql = IF(
  @col_exists = 0,
  "ALTER TABLE disclosure_types ADD COLUMN review_status VARCHAR(32) NULL DEFAULT NULL AFTER status",
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Index for company template lifecycle queries
SET @idx_exists = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'disclosure_types'
    AND index_name   = 'idx_dt_company_review_status'
);
SET @sql = IF(
  @idx_exists = 0,
  'ALTER TABLE disclosure_types ADD INDEX idx_dt_company_review_status (company_id, review_status)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
