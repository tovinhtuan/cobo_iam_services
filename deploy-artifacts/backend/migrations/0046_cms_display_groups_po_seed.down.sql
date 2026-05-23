-- 0046 rollback — remove PO-seeded display groups
-- Only safe if no template_display_groups rows reference these codes.

SET NAMES utf8mb4;

DELETE FROM disclosure_display_groups
WHERE display_group_code IN (
  'display_groups_001', 'display_groups_002', 'display_groups_003',
  'display_groups_004', 'display_groups_005', 'display_groups_006',
  'display_groups_007'
);
