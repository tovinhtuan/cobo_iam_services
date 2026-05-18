-- 0035: Add display groups table and map existing templates
-- Non-breaking: display_group_code column is nullable

CREATE TABLE IF NOT EXISTS disclosure_display_groups (
    display_group_code  VARCHAR(64)   NOT NULL,
    name_vi             VARCHAR(255)  NOT NULL,
    name_en             VARCHAR(255)  NOT NULL DEFAULT '',
    description         VARCHAR(2000) NOT NULL DEFAULT '',
    icon                VARCHAR(64)   NOT NULL DEFAULT 'FileText',
    display_order       INT           NOT NULL DEFAULT 0,
    is_active           TINYINT(1)    NOT NULL DEFAULT 1,
    is_system           TINYINT(1)    NOT NULL DEFAULT 1,
    created_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (display_group_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE disclosure_types
    ADD COLUMN display_group_code VARCHAR(64) NULL AFTER group_id;

-- Seed 7 display groups (idempotent via ON DUPLICATE KEY UPDATE)
INSERT INTO disclosure_display_groups
    (display_group_code, name_vi, name_en, description, icon, display_order)
VALUES
    ('display-governance',
     'Quản trị, Tuân thủ, Rủi ro & Pháp lý',
     'Governance, Compliance, Risk & Legal',
     'Các nghĩa vụ liên quan đến quản trị doanh nghiệp, tuân thủ pháp luật, quản trị rủi ro, pháp lý và các thay đổi quan trọng trong cơ cấu quản trị.',
     'ShieldCheck', 1),
    ('display-strategy',
     'Chiến lược, Điều hành & Tình báo thị trường',
     'Strategy, Executive Management & Market Intelligence',
     'Các thông tin về định hướng chiến lược, hoạt động điều hành, quyết định quản lý và tín hiệu thị trường có ảnh hưởng đến doanh nghiệp.',
     'GitMerge', 2),
    ('display-finance',
     'Tài chính, Hiệu quả kinh doanh & Kiểm soát hiệu suất',
     'Finance, Business Performance & Performance Control',
     'Các báo cáo và chỉ số liên quan đến tài chính, kết quả kinh doanh, hiệu quả vận hành, kiểm toán và kiểm soát hiệu suất.',
     'BarChart2', 3),
    ('display-operations',
     'Vận hành, Quy trình, Dự án & Chuyển đổi',
     'Operations, Process, Project & Change Management',
     'Các thông tin về hoạt động vận hành, quy trình nội bộ, dự án trọng điểm, chương trình chuyển đổi và thay đổi vận hành.',
     'Settings', 4),
    ('display-growth',
     'Tăng trưởng, Marketing, Bán hàng & Khách hàng',
     'Growth, Marketing, Sales & Customer Management',
     'Các thông tin liên quan đến tăng trưởng, thị trường, hoạt động marketing, bán hàng và quản lý quan hệ khách hàng.',
     'TrendingUp', 5),
    ('display-people',
     'Con người, Tri thức, Công nghệ, Dữ liệu & AI',
     'People, Knowledge, Technology, Data & AI',
     'Các thông tin liên quan đến nhân sự, tri thức tổ chức, công nghệ, dữ liệu, AI và năng lực số của doanh nghiệp.',
     'Users', 6),
    ('group-006',
     'CBTT/Báo cáo tùy chỉnh',
     'Custom Disclosure / Custom Report',
     'Các loại CBTT/Báo cáo do doanh nghiệp tự định nghĩa ngoài danh mục mặc định của hệ thống, phục vụ nhu cầu quản trị và công bố riêng.',
     'Layers', 7)
ON DUPLICATE KEY UPDATE
    name_vi       = VALUES(name_vi),
    name_en       = VALUES(name_en),
    description   = VALUES(description),
    icon          = VALUES(icon),
    display_order = VALUES(display_order);

-- Map existing catalog templates to display groups
UPDATE disclosure_types SET display_group_code = 'display-finance'    WHERE type_id = 'dt-periodic-financial';
UPDATE disclosure_types SET display_group_code = 'display-governance' WHERE type_id = 'dt-event-major-change';
UPDATE disclosure_types SET display_group_code = 'group-006'          WHERE type_id = 'dt-custom-obligation';
UPDATE disclosure_types SET display_group_code = 'display-operations' WHERE type_id = 'dt-obligation-report';
UPDATE disclosure_types SET display_group_code = 'display-strategy'   WHERE type_id = 'dt-shareholder-meeting';
UPDATE disclosure_types SET display_group_code = 'display-finance'    WHERE type_id = 'dt-disclosure-transaction';
