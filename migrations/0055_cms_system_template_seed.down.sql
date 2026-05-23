-- 0055 rollback — remove seeded system templates and related rows

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

DELETE FROM global_workflow_steps WHERE workflow_id IN (
  'gw-sys-q1-financial-v1',
  'gw-sys-hr-executive-v1',
  'gw-sys-board-resolution-v1'
);

DELETE FROM global_workflows WHERE workflow_id IN (
  'gw-sys-q1-financial-v1',
  'gw-sys-hr-executive-v1',
  'gw-sys-board-resolution-v1'
);

DELETE FROM template_display_groups WHERE template_id IN (
  'dt-sys-q1-financial',
  'dt-sys-hr-executive',
  'dt-sys-board-resolution'
);

DELETE FROM disclosure_type_versions WHERE type_id IN (
  'dt-sys-q1-financial',
  'dt-sys-hr-executive',
  'dt-sys-board-resolution'
);

DELETE FROM disclosure_types WHERE type_id IN (
  'dt-sys-q1-financial',
  'dt-sys-hr-executive',
  'dt-sys-board-resolution'
);

SET FOREIGN_KEY_CHECKS = 1;
