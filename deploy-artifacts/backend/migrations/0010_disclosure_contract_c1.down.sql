SET NAMES utf8mb4;

ALTER TABLE disclosure_records
  DROP COLUMN evidence_link,
  DROP COLUMN attachments_json,
  DROP COLUMN published_date,
  DROP COLUMN planned_date,
  DROP COLUMN summary,
  DROP COLUMN type_id;
