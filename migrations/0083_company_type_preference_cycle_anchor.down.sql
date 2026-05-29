ALTER TABLE company_type_preferences
  DROP COLUMN IF EXISTS cycle_anchor_month,
  DROP COLUMN IF EXISTS cycle_anchor_day;
