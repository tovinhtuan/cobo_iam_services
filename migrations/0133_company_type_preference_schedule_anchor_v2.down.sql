ALTER TABLE company_type_preferences
  DROP COLUMN IF EXISTS cycle_anchor_weekday,
  DROP COLUMN IF EXISTS month_in_quarter;
