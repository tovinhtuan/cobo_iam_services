-- 0070: Keep only PO display_groups_001..007; remove legacy catalog codes (0035 era).
-- Remaps template junction + disclosure_types.display_group_code before delete.

SET NAMES utf8mb4;

INSERT INTO disclosure_display_groups
  (display_group_code, name_vi, name_en, display_order, is_active, is_system)
VALUES
  ('display_groups_001', 'Tuân thủ, Quản trị & Quản lý Rủi ro',
   'Compliance, Governance, & Risk Management', 1, 1, 1),
  ('display_groups_002', 'Chiến lược, Điều hành & Nhân sự',
   'Strategy, Executive Management, Human Resource', 2, 1, 1),
  ('display_groups_003', 'Tài chính, Kinh doanh',
   'Finance, Business Performance & Operation Control', 3, 1, 1),
  ('display_groups_004', 'Vận hành, Quy trình, Quản lý Dự án',
   'Operations, Process & Project Management', 4, 1, 1),
  ('display_groups_005', 'Bán hàng, Marketing & Dịch vụ Khách hàng',
   'Sales, Marketing & Customer Service', 5, 1, 1),
  ('display_groups_006', 'Nghiên cứu, Công nghệ & Số hoá',
   'R&D, Technology & Digital Transformation', 6, 1, 1),
  ('display_groups_007', 'Tùy chỉnh doanh nghiệp',
   'Template customer for company', 7, 1, 1)
ON DUPLICATE KEY UPDATE
  name_vi       = VALUES(name_vi),
  name_en       = VALUES(name_en),
  display_order = VALUES(display_order),
  is_active     = 1,
  is_system     = 1;

INSERT IGNORE INTO template_display_groups (template_id, display_group_code, display_order)
SELECT
  tdg.template_id,
  CASE tdg.display_group_code
    WHEN 'display-governance' THEN 'display_groups_001'
    WHEN 'display-strategy'   THEN 'display_groups_002'
    WHEN 'display-finance'    THEN 'display_groups_003'
    WHEN 'display-operations' THEN 'display_groups_004'
    WHEN 'display-growth'     THEN 'display_groups_005'
    WHEN 'display-people'     THEN 'display_groups_006'
    WHEN 'group-006'          THEN 'display_groups_007'
    ELSE tdg.display_group_code
  END,
  tdg.display_order
FROM template_display_groups tdg
WHERE tdg.display_group_code IN (
  'display-governance', 'display-strategy', 'display-finance',
  'display-operations', 'display-growth', 'display-people', 'group-006'
);

DELETE FROM template_display_groups
WHERE display_group_code IN (
  'display-governance', 'display-strategy', 'display-finance',
  'display-operations', 'display-growth', 'display-people', 'group-006'
);

UPDATE disclosure_types
SET display_group_code = CASE display_group_code
  WHEN 'display-governance' THEN 'display_groups_001'
  WHEN 'display-strategy'   THEN 'display_groups_002'
  WHEN 'display-finance'    THEN 'display_groups_003'
  WHEN 'display-operations' THEN 'display_groups_004'
  WHEN 'display-growth'     THEN 'display_groups_005'
  WHEN 'display-people'     THEN 'display_groups_006'
  WHEN 'group-006'          THEN 'display_groups_007'
  ELSE display_group_code
END
WHERE display_group_code IN (
  'display-governance', 'display-strategy', 'display-finance',
  'display-operations', 'display-growth', 'display-people', 'group-006'
);

UPDATE disclosure_types
SET display_group_code = 'display_groups_007'
WHERE display_group_code IS NOT NULL
  AND TRIM(display_group_code) <> ''
  AND display_group_code NOT IN (
    'display_groups_001', 'display_groups_002', 'display_groups_003',
    'display_groups_004', 'display_groups_005', 'display_groups_006', 'display_groups_007'
  );

DELETE FROM template_display_groups
WHERE display_group_code NOT IN (
  'display_groups_001', 'display_groups_002', 'display_groups_003',
  'display_groups_004', 'display_groups_005', 'display_groups_006', 'display_groups_007'
);

DELETE FROM disclosure_display_groups
WHERE display_group_code NOT IN (
  'display_groups_001', 'display_groups_002', 'display_groups_003',
  'display_groups_004', 'display_groups_005', 'display_groups_006', 'display_groups_007'
);
