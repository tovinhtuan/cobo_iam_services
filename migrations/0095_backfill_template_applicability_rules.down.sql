SET NAMES utf8mb4;

-- Revert backfill: clear rules on global active versions (grace behavior relies on strict flag).
UPDATE disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
SET v.applicability_rules_json = NULL
WHERE t.company_id IS NULL;
