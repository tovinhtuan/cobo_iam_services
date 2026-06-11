-- Verification queries for Deadline Engine migration 0097 (MT-01..MT-11).
-- Run after: mysql ... cobo_iam < migrations/0097_deadline_engine_v2_prepare.up.sql

-- MT-05: all scoped rows with rules should have use_structure_deadline=false
SELECT 'MT-05_fail' AS check_id, v.type_id, v.version_no
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
WHERE t.company_id IS NULL AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.use_structure_deadline')), 'true') <> 'false';

-- MT-04 + MT-02 sample: periodic should have calendar + deadline_days
SELECT 'MT-04_periodic' AS check_id, v.type_id,
  JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_day_type')) AS day_type,
  JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days')) AS deadline_days
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
WHERE t.company_id IS NULL AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND LOWER(TRIM(v.template_category)) = 'periodic';

-- MT-07: mirror sync periodic
SELECT 'MT-07_fail' AS check_id, v.type_id,
  JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.deadline_days')) AS config_days,
  JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days')) AS rules_days
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
WHERE t.company_id IS NULL AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND LOWER(TRIM(v.template_category)) = 'periodic'
  AND JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days') IS NOT NULL
  AND CAST(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.deadline_days')) AS UNSIGNED)
    <> CAST(JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days')) AS UNSIGNED);

-- MT-08: irregular must NOT have deadline_days injected
SELECT 'MT-08_fail' AS check_id, v.type_id, v.template_category
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
WHERE t.company_id IS NULL AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND LOWER(TRIM(v.template_category)) <> 'periodic'
  AND JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days') IS NOT NULL;

-- MT-03 audit: fallback rows (config deadline_days was 0/missing)
SELECT 'MT-03_audit_fallback' AS check_id, v.type_id, v.version_no,
  JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.deadline_days')) AS config_days,
  JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days')) AS rules_days,
  JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_by_structure.simple_structure.days')) AS simple_structure_days
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
WHERE t.company_id IS NULL AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND LOWER(TRIM(v.template_category)) = 'periodic'
  AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.deadline_days')) AS UNSIGNED), 0) = 0;

-- MT-06: structure keys still present on periodic
SELECT 'MT-06_structure' AS check_id, v.type_id,
  JSON_EXTRACT(v.applicability_rules_json, '$.deadline_by_structure') AS structure_map
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
WHERE t.company_id IS NULL AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND LOWER(TRIM(v.template_category)) = 'periodic'
  AND JSON_EXTRACT(v.applicability_rules_json, '$.deadline_by_structure') IS NULL;
