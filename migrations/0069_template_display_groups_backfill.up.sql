-- 0069: Backfill template_display_groups junction from legacy display_group_code (PO D4-A)

SET NAMES utf8mb4;

INSERT INTO template_display_groups (template_id, display_group_code, display_order)
SELECT t.type_id, t.display_group_code, 1
FROM disclosure_types t
WHERE t.display_group_code IS NOT NULL
  AND TRIM(t.display_group_code) <> ''
  AND NOT EXISTS (
    SELECT 1 FROM template_display_groups tdg WHERE tdg.template_id = t.type_id
  )
ON DUPLICATE KEY UPDATE display_order = VALUES(display_order);
