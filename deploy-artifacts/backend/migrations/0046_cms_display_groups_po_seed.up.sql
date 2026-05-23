-- 0046: CMS-DBA-002 — Seed PO-defined 7 display groups
-- Inserts display_groups_001..007 into disclosure_display_groups.
-- Existing legacy codes (display-governance, display-strategy, etc.) are NOT removed.
-- Ref: decision-record-po-answers-2026-05-23.md § Display Groups (lock)

SET NAMES utf8mb4;

INSERT INTO disclosure_display_groups
  (display_group_code, name_vi, name_en, display_order)
VALUES
  ('display_groups_001', 'Tuân thủ, Quản trị & Quản lý Rủi ro',
   'Compliance, Governance, & Risk Management', 1),
  ('display_groups_002', 'Chiến lược, Điều hành & Nhân sự',
   'Strategy, Executive Management, Human Resource', 2),
  ('display_groups_003', 'Tài chính, Kinh doanh',
   'Finance, Business Performance & Operation Control', 3),
  ('display_groups_004', 'Vận hành, Quy trình, Quản lý Dự án',
   'Operations, Process & Project Management', 4),
  ('display_groups_005', 'Bán hàng, Marketing & Dịch vụ Khách hàng',
   'Sales, Marketing & Customer Service', 5),
  ('display_groups_006', 'Nghiên cứu, Công nghệ & Số hoá',
   'R&D, Technology & Digital Transformation', 6),
  ('display_groups_007', 'Tùy chỉnh doanh nghiệp',
   'Template customer for company', 7)
ON DUPLICATE KEY UPDATE
  name_vi       = VALUES(name_vi),
  name_en       = VALUES(name_en),
  display_order = VALUES(display_order);
