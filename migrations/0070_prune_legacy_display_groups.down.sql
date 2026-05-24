-- 0070 down: Re-seed legacy display groups from 0035 (does not restore deleted junction rows).

SET NAMES utf8mb4;

INSERT INTO disclosure_display_groups
  (display_group_code, name_vi, name_en, description, icon, display_order, is_active, is_system)
VALUES
  ('display-governance', 'Quản trị, Tuân thủ, Rủi ro & Pháp lý', 'Governance, Compliance, Risk & Legal',
   'Legacy group restored by rollback.', 'ShieldCheck', 1, 1, 1),
  ('display-strategy', 'Chiến lược, Điều hành & Tình báo thị trường', 'Strategy, Executive Management & Market Intelligence',
   'Legacy group restored by rollback.', 'GitMerge', 2, 1, 1),
  ('display-finance', 'Tài chính, Hiệu quả kinh doanh & Kiểm soát hiệu suất', 'Finance, Business Performance & Performance Control',
   'Legacy group restored by rollback.', 'BarChart2', 3, 1, 1),
  ('display-operations', 'Vận hành, Quy trình, Dự án & Chuyển đổi', 'Operations, Process, Project & Change Management',
   'Legacy group restored by rollback.', 'Settings', 4, 1, 1),
  ('display-growth', 'Tăng trưởng, Marketing, Bán hàng & Khách hàng', 'Growth, Marketing, Sales & Customer Management',
   'Legacy group restored by rollback.', 'TrendingUp', 5, 1, 1),
  ('display-people', 'Con người, Tri thức, Công nghệ, Dữ liệu & AI', 'People, Knowledge, Technology, Data & AI',
   'Legacy group restored by rollback.', 'Users', 6, 1, 1),
  ('group-006', 'CBTT/Báo cáo tùy chỉnh', 'Custom Disclosure / Custom Report',
   'Legacy group restored by rollback.', 'Layers', 7, 1, 1)
ON DUPLICATE KEY UPDATE
  name_vi = VALUES(name_vi),
  name_en = VALUES(name_en),
  description = VALUES(description);
