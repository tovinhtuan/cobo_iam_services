-- 0055: CMS-DBA-001 — Seed 3 system templates with display groups and global workflows
-- Renumbered from deploy-artifacts/0047 to avoid conflict with main pipeline.
-- Ref: decision-record-po-answers-2026-05-23.md § System Templates Seed + Workflow Steps

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ─── 1. Template headers ──────────────────────────────────────────────────────

INSERT INTO disclosure_types
  (type_id, company_id, group_id, display_group_code, active_version_no, status, is_mandatory)
VALUES
  ('dt-sys-q1-financial',    NULL, 'group-001', 'display_groups_003', 1, 'active', 1),
  ('dt-sys-hr-executive',    NULL, 'group-002', 'display_groups_002', 1, 'active', 0),
  ('dt-sys-board-resolution',NULL, 'group-002', 'display_groups_001', 1, 'active', 0)
ON DUPLICATE KEY UPDATE
  display_group_code = VALUES(display_group_code),
  status             = VALUES(status),
  is_mandatory       = VALUES(is_mandatory);

-- ─── 2. Template versions ─────────────────────────────────────────────────────

INSERT INTO disclosure_type_versions
  (type_id, version_no, name, category, template_category, description, deadline_rule, periodicity, change_note, updated_by)
VALUES
  ('dt-sys-q1-financial', 1,
   'Báo cáo tài chính quý 1', 'Định kỳ', 'periodic',
   'Công bố báo cáo tài chính quý 1 theo quy định Thông tư 96/2020/TT-BTC.',
   '15/03', 'quarterly', 'seed-cms-0055', 'system'),

  ('dt-sys-hr-executive', 1,
   'Thay đổi nhân sự cấp cao', 'Bất thường', 'irregular',
   'Công bố khi phát sinh thay đổi nhân sự cấp cao (HĐQT, Ban TGĐ, KSV).',
   'T+1', NULL, 'seed-cms-0055', 'system'),

  ('dt-sys-board-resolution', 1,
   'Nghị quyết Hội đồng quản trị', 'Bất thường', 'irregular',
   'Công bố nghị quyết Hội đồng quản trị sau khi ban hành.',
   'T+1', NULL, 'seed-cms-0055', 'system')
ON DUPLICATE KEY UPDATE
  name             = VALUES(name),
  template_category = VALUES(template_category),
  deadline_rule    = VALUES(deadline_rule),
  periodicity      = VALUES(periodicity);

-- ─── 3. Display group mappings (many-to-many, Decision D-A) ──────────────────

INSERT INTO template_display_groups (template_id, display_group_code, display_order) VALUES
  -- Báo cáo tài chính quý 1 → groups 001, 003
  ('dt-sys-q1-financial', 'display_groups_001', 1),
  ('dt-sys-q1-financial', 'display_groups_003', 2),
  -- Thay đổi nhân sự cấp cao → groups 002, 004
  ('dt-sys-hr-executive', 'display_groups_002', 1),
  ('dt-sys-hr-executive', 'display_groups_004', 2),
  -- Nghị quyết HĐQT → groups 001, 002, 004, 005, 006, 007
  ('dt-sys-board-resolution', 'display_groups_001', 1),
  ('dt-sys-board-resolution', 'display_groups_002', 2),
  ('dt-sys-board-resolution', 'display_groups_004', 3),
  ('dt-sys-board-resolution', 'display_groups_005', 4),
  ('dt-sys-board-resolution', 'display_groups_006', 5),
  ('dt-sys-board-resolution', 'display_groups_007', 6)
ON DUPLICATE KEY UPDATE display_order = VALUES(display_order);

-- ─── 4. Global workflow headers ───────────────────────────────────────────────

INSERT INTO global_workflows (workflow_id, type_id, version_no, is_active, created_by) VALUES
  ('gw-sys-q1-financial-v1',     'dt-sys-q1-financial',    1, 1, 'system'),
  ('gw-sys-hr-executive-v1',     'dt-sys-hr-executive',    1, 1, 'system'),
  ('gw-sys-board-resolution-v1', 'dt-sys-board-resolution',1, 1, 'system')
ON DUPLICATE KEY UPDATE is_active = VALUES(is_active);

-- ─── 5. Global workflow steps (Decision D-C — cumulative deadline string) ─────

INSERT INTO global_workflow_steps
  (step_id, workflow_id, type_id, display_order, stage_name, department, step_deadline)
VALUES
  -- Template 1: Báo cáo tài chính quý 1 (4 steps)
  ('gws-q1-fin-1', 'gw-sys-q1-financial-v1', 'dt-sys-q1-financial', 1,
   'Xác định nghĩa vụ',  'Phòng pháp chế',   'T+2'),
  ('gws-q1-fin-2', 'gw-sys-q1-financial-v1', 'dt-sys-q1-financial', 2,
   'Chuẩn bị dữ liệu',   'Phòng kế toán',    'T+3'),
  ('gws-q1-fin-3', 'gw-sys-q1-financial-v1', 'dt-sys-q1-financial', 3,
   'Rà soát pháp lý',    'Phòng pháp chế',   'T+5'),
  ('gws-q1-fin-4', 'gw-sys-q1-financial-v1', 'dt-sys-q1-financial', 4,
   'Phê duyệt nội bộ',   'Phòng kinh doanh', 'T+7'),

  -- Template 2: Thay đổi nhân sự cấp cao (3 steps)
  ('gws-hr-exec-1', 'gw-sys-hr-executive-v1', 'dt-sys-hr-executive', 1,
   'Phát sinh sự kiện',  'Phòng nhân sự',                    'T+1'),
  ('gws-hr-exec-2', 'gw-sys-hr-executive-v1', 'dt-sys-hr-executive', 2,
   'Phê duyệt nội bộ',   'Phòng giám đốc',                   'T+1'),
  ('gws-hr-exec-3', 'gw-sys-hr-executive-v1', 'dt-sys-hr-executive', 3,
   'Công bố thông tin',  'Phòng truyền thông, marketing',    'T+2'),

  -- Template 3: Nghị quyết Hội đồng quản trị (3 steps)
  ('gws-board-res-1', 'gw-sys-board-resolution-v1', 'dt-sys-board-resolution', 1,
   'Họp và thông qua',   'Phòng HĐQT',                       'T+3'),
  ('gws-board-res-2', 'gw-sys-board-resolution-v1', 'dt-sys-board-resolution', 2,
   'Soạn thảo văn bản',  'Phòng Hành chính nhân sự',         'T+4'),
  ('gws-board-res-3', 'gw-sys-board-resolution-v1', 'dt-sys-board-resolution', 3,
   'Công bố thông tin',  'Phòng Hành chính nhân sự',         'T+5')
ON DUPLICATE KEY UPDATE
  stage_name    = VALUES(stage_name),
  department    = VALUES(department),
  step_deadline = VALUES(step_deadline);

SET FOREIGN_KEY_CHECKS = 1;
