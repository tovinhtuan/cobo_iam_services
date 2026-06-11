SET NAMES utf8mb4;

-- Revert Batch 1 JSON keys only. Preserves classes, sectors, deadline_by_structure.
UPDATE disclosure_type_versions v
INNER JOIN disclosure_types t
  ON t.type_id = v.type_id AND v.version_no = t.active_version_no
SET v.applicability_rules_json = JSON_REMOVE(
  JSON_REMOVE(
    JSON_REMOVE(v.applicability_rules_json, '$.deadline_days'),
    '$.deadline_day_type'
  ),
  '$.use_structure_deadline'
)
WHERE t.company_id IS NULL
  AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL;
