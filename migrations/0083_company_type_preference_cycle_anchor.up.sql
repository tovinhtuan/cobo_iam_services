-- Phase 6 Sub-feature D: per-company cycle anchor override for PERIODIC deadline mode.
-- When non-null, these values override the template's cycle_anchor_month/day
-- in DeadlineCalculator.computeCycleStart for the PERIODIC deadline mode.
ALTER TABLE company_type_preferences
  ADD COLUMN cycle_anchor_month TINYINT UNSIGNED NULL COMMENT 'Per-company fiscal year start month (1-12). NULL = use template default.',
  ADD COLUMN cycle_anchor_day   TINYINT UNSIGNED NULL COMMENT 'Per-company fiscal year start day (1-28). NULL = use template default.';
