SET NAMES utf8mb4;

ALTER TABLE disclosure_records
  ADD COLUMN type_id VARCHAR(64) NULL AFTER company_id,
  ADD COLUMN summary TEXT NULL AFTER title,
  ADD COLUMN planned_date DATE NULL AFTER content,
  ADD COLUMN published_date DATE NULL AFTER planned_date,
  ADD COLUMN attachments_json JSON NULL AFTER status,
  ADD COLUMN evidence_link VARCHAR(1024) NULL AFTER attachments_json;
