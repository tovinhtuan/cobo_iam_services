-- 0072: Repair Vietnamese labels on disclosure_display_groups (Mojibake + PO source of truth).
-- Root cause: same as 0036 — migration applied without utf8mb4 connection (push-migration fixed in deploy script).
-- Idempotent: CONVERT only touches garbled rows; UPSERT forces correct PO labels for 001..007.

SET NAMES utf8mb4;

ALTER TABLE disclosure_display_groups
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

UPDATE disclosure_display_groups
SET
  name_vi     = CONVERT(CONVERT(name_vi     USING latin1) USING utf8mb4),
  name_en     = CONVERT(CONVERT(name_en     USING latin1) USING utf8mb4),
  description = CONVERT(CONVERT(description USING latin1) USING utf8mb4)
WHERE name_vi IS NOT NULL
  AND name_vi != ''
  AND HEX(name_vi) NOT LIKE '%E1B%';

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
  is_active     = VALUES(is_active),
  is_system     = VALUES(is_system);
