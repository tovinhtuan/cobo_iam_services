SET NAMES utf8mb4;

ALTER TABLE disclosure_type_versions
  ADD COLUMN deadline_strategy VARCHAR(64) NULL AFTER template_category;

UPDATE disclosure_type_versions
SET
  template_category = CASE
    WHEN LOWER(TRIM(template_category)) IN ('định kỳ', 'dinh ky', 'periodic') THEN 'periodic'
    WHEN LOWER(TRIM(template_category)) IN ('bất thường', 'bat thuong', 'irregular') THEN 'irregular'
    WHEN LOWER(TRIM(template_category)) IN ('tùy chỉnh', 'tuy chinh', 'custom') THEN 'custom'
    ELSE template_category
  END,
  periodicity = CASE
    WHEN LOWER(TRIM(periodicity)) IN ('hàng tháng', 'hang thang', 'monthly') THEN 'monthly'
    WHEN LOWER(TRIM(periodicity)) IN ('hàng quý', 'hang quy', 'quarterly') THEN 'quarterly'
    WHEN LOWER(TRIM(periodicity)) IN ('hàng năm', 'hang nam', 'yearly') THEN 'yearly'
    WHEN LOWER(TRIM(periodicity)) IN ('bất thường', 'bat thuong', 'event_based') THEN 'event_based'
    WHEN LOWER(TRIM(periodicity)) IN ('theo yêu cầu', 'theo yeu cau', 'ad_hoc') THEN 'ad_hoc'
    ELSE periodicity
  END;

UPDATE disclosure_type_versions
SET deadline_strategy = CASE
  WHEN template_category = 'periodic' THEN 'fixed_cycle_days'
  WHEN template_category = 'irregular' THEN 'event_relative_hours'
  WHEN template_category = 'custom' THEN 'configurable'
  ELSE deadline_strategy
END
WHERE deadline_strategy IS NULL OR TRIM(deadline_strategy) = '';
