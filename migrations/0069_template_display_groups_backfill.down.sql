-- 0069 down: remove junction rows created only from legacy column backfill (safe on dev re-run)

DELETE tdg FROM template_display_groups tdg
INNER JOIN disclosure_types t ON t.type_id = tdg.template_id
WHERE tdg.display_group_code = t.display_group_code
  AND tdg.display_order = 1;
