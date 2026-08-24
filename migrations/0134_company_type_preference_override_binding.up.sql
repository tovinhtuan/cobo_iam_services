-- Periodicity V2 Phase 3: persistent frequency binding + active/inactive override authority.
-- ACTIVE=1 + frequency match → Company override wins; ACTIVE=0 retains historical values but is ignored
-- (including when CMS frequency later returns — no auto-reactivation).
-- NULL frequency + NULL active = no override authority (inherit CMS).

ALTER TABLE company_type_preferences
  ADD COLUMN cycle_anchor_override_frequency VARCHAR(16) NULL
    COMMENT 'CMS frequency the override was authored against (weekly|monthly|quarterly|yearly). NULL = none.',
  ADD COLUMN cycle_anchor_override_active TINYINT(1) NULL
    COMMENT '1=ACTIVE; 0=INACTIVE after frequency change; NULL=no override authority.';

-- Safe backfill: bind existing non-null anchor columns to current active CMS frequency.
-- Idempotent: only rows with NULL binding. Does not rewrite occurrences / T / DueAt.
UPDATE company_type_preferences ctp
INNER JOIN disclosure_types dt ON dt.type_id = ctp.type_id
INNER JOIN disclosure_type_versions dtv
  ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
SET
  ctp.cycle_anchor_override_frequency = LOWER(TRIM(JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json, '$.frequency_unit')))),
  ctp.cycle_anchor_override_active = 1
WHERE ctp.cycle_anchor_override_frequency IS NULL
  AND (
    ctp.cycle_anchor_month IS NOT NULL
    OR ctp.cycle_anchor_day IS NOT NULL
    OR ctp.cycle_anchor_weekday IS NOT NULL
    OR ctp.month_in_quarter IS NOT NULL
  )
  AND JSON_EXTRACT(dtv.deadline_config_json, '$.frequency_unit') IS NOT NULL
  AND TRIM(JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json, '$.frequency_unit'))) <> '';
