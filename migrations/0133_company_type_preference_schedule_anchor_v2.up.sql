-- Periodicity V2 Phase 0: additive company schedule-anchor columns (nullable).
-- Runtime Company typed override wiring is Phase 3; columns are expand-only for forward compat.
-- NULL = inherit CMS / legacy read-defaults (weekly Sunday, quarterly MiQ=1).
ALTER TABLE company_type_preferences
  ADD COLUMN cycle_anchor_weekday TINYINT NULL COMMENT 'Go weekday 0=Sunday..6=Saturday. NULL = inherit CMS / legacy Sunday.',
  ADD COLUMN month_in_quarter TINYINT UNSIGNED NULL COMMENT '1..3 month within calendar quarter. NULL = inherit CMS / legacy 1.';
