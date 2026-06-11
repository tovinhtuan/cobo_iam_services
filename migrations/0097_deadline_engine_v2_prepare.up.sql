SET NAMES utf8mb4;

-- Deadline Engine v2 data preparation (Batch 1).
-- Scope: active global template versions with applicability_rules_json (same as 0095).
-- Does NOT change runtime behavior while DEADLINE_ENGINE_V2=false.

-- 0097-A: disable structure override at runtime (data prep; legacy worker ignores until Batch 5).
UPDATE disclosure_type_versions v
INNER JOIN disclosure_types t
  ON t.type_id = v.type_id AND v.version_no = t.active_version_no
SET v.applicability_rules_json = JSON_SET(
  v.applicability_rules_json,
  '$.use_structure_deadline', CAST(FALSE AS JSON)
)
WHERE t.company_id IS NULL
  AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL;

-- 0097-B + 0097-C: periodic global — seed deadline_days + deadline_day_type.
-- Fallback when deadline_config.deadline_days is null/0/missing:
--   1) deadline_by_structure.simple_structure.days
--   2) else 30 (ADR I-18 default N audit baseline)
UPDATE disclosure_type_versions v
INNER JOIN disclosure_types t
  ON t.type_id = v.type_id AND v.version_no = t.active_version_no
SET v.applicability_rules_json = JSON_SET(
  JSON_SET(
    v.applicability_rules_json,
    '$.deadline_days',
    COALESCE(
      NULLIF(CAST(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.deadline_days')) AS UNSIGNED), 0),
      NULLIF(CAST(JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_by_structure.simple_structure.days')) AS UNSIGNED), 0),
      30
    )
  ),
  '$.deadline_day_type', 'calendar'
)
WHERE t.company_id IS NULL
  AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND LOWER(TRIM(v.template_category)) = 'periodic';

-- 0097-D: deadline_by_structure intentionally not modified (preserves 0095 30/30/20).

-- 0097-E: mirror applicability_rules.deadline_days into deadline_config (transition write-only).
UPDATE disclosure_type_versions v
INNER JOIN disclosure_types t
  ON t.type_id = v.type_id AND v.version_no = t.active_version_no
SET v.deadline_config_json = JSON_SET(
  COALESCE(v.deadline_config_json, JSON_OBJECT()),
  '$.deadline_days',
  CAST(JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days')) AS UNSIGNED)
)
WHERE t.company_id IS NULL
  AND t.active_version_no > 0
  AND v.applicability_rules_json IS NOT NULL
  AND LOWER(TRIM(v.template_category)) = 'periodic'
  AND JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days') IS NOT NULL;

-- Audit: rows where deadline_days did NOT come from deadline_config (review fallback usage).
-- Run manually after migrate:
-- SELECT v.type_id, v.version_no,
--   JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.deadline_days')) AS config_days,
--   JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_days')) AS rules_days,
--   JSON_UNQUOTE(JSON_EXTRACT(v.applicability_rules_json, '$.deadline_by_structure.simple_structure.days')) AS simple_structure_days
-- FROM disclosure_type_versions v
-- INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
-- WHERE t.company_id IS NULL AND t.active_version_no > 0
--   AND v.applicability_rules_json IS NOT NULL
--   AND LOWER(TRIM(v.template_category)) = 'periodic'
--   AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.deadline_days')) AS UNSIGNED), 0) = 0;
