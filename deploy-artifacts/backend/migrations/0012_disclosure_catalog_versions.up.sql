SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS disclosure_type_groups (
  group_id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  icon VARCHAR(64) NOT NULL,
  display_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS disclosure_types (
  type_id VARCHAR(64) PRIMARY KEY,
  company_id VARCHAR(64) NULL,
  group_id VARCHAR(64) NOT NULL,
  active_version_no INT NOT NULL DEFAULT 1,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_disclosure_types_company (company_id),
  INDEX idx_disclosure_types_group (group_id),
  CONSTRAINT fk_disclosure_types_group FOREIGN KEY (group_id) REFERENCES disclosure_type_groups(group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS disclosure_type_versions (
  type_id VARCHAR(64) NOT NULL,
  version_no INT NOT NULL,
  name VARCHAR(512) NOT NULL,
  category VARCHAR(64) NOT NULL,
  template_category VARCHAR(64) NOT NULL,
  description TEXT NOT NULL,
  legal_basis TEXT NULL,
  applicability TEXT NULL,
  implementation_content TEXT NULL,
  implementation_notes TEXT NULL,
  special_cases TEXT NULL,
  report_content TEXT NULL,
  required_docs TEXT NULL,
  deadline_rule TEXT NULL,
  periodicity VARCHAR(64) NULL,
  channels_text TEXT NULL,
  beneficiaries TEXT NULL,
  receiving_authorities TEXT NULL,
  format VARCHAR(64) NULL,
  legal_risks_text TEXT NULL,
  general_info TEXT NULL,
  tags_json JSON NULL,
  change_note TEXT NULL,
  updated_by VARCHAR(64) NOT NULL,
  activated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (type_id, version_no),
  CONSTRAINT fk_disclosure_type_versions_type FOREIGN KEY (type_id) REFERENCES disclosure_types(type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO disclosure_type_groups (group_id, name, description, icon, display_order) VALUES
  ('group-001', 'Định kỳ', 'Nghĩa vụ công bố theo chu kỳ.', 'Calendar', 1),
  ('group-002', 'Bất thường', 'Nghĩa vụ công bố theo sự kiện phát sinh.', 'AlertCircle', 2),
  ('group-006', 'Tùy chỉnh doanh nghiệp', 'Template nội bộ do doanh nghiệp cấu hình.', 'Settings', 3)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  description = VALUES(description),
  icon = VALUES(icon),
  display_order = VALUES(display_order);

INSERT INTO disclosure_types (type_id, company_id, group_id, active_version_no, status) VALUES
  ('dt-periodic-financial', NULL, 'group-001', 1, 'active'),
  ('dt-event-major-change', NULL, 'group-002', 1, 'active'),
  ('dt-custom-obligation', NULL, 'group-006', 1, 'active')
ON DUPLICATE KEY UPDATE
  group_id = VALUES(group_id),
  status = VALUES(status);

INSERT INTO disclosure_type_versions (
  type_id, version_no, name, category, template_category, description, legal_basis, applicability, implementation_content,
  implementation_notes, special_cases, report_content, required_docs, deadline_rule, periodicity, channels_text,
  beneficiaries, receiving_authorities, format, legal_risks_text, general_info, tags_json, change_note, updated_by
) VALUES
  ('dt-periodic-financial', 1, 'Báo cáo tài chính định kỳ', 'Định kỳ', 'Định kỳ', 'Công bố báo cáo tài chính theo quý.',
   'Thông tư 96/2020/TT-BTC', 'Doanh nghiệp niêm yết', 'Tổng hợp dữ liệu và lập báo cáo tài chính quý.', 'Kiểm tra số liệu trước khi nộp.',
   'Điều chỉnh khi có kiểm toán ngoại lệ.', 'Báo cáo tài chính quý + thuyết minh.', 'BCTC quý, biên bản phê duyệt.',
   'Trong vòng 20 ngày kể từ khi kết thúc quý.', 'Hàng quý', 'UBCKNN, Sở GDCK', 'Nhà đầu tư, cơ quan quản lý',
   'UBCKNN, HOSE/HNX', 'PDF', 'Phạt chậm công bố theo quy định hiện hành.', 'Áp dụng mặc định cho tất cả công ty niêm yết.',
   JSON_ARRAY('Định kỳ', 'Tài chính'), 'seed', 'system'),
  ('dt-event-major-change', 1, 'Công bố sự kiện bất thường', 'Bất thường', 'Bất thường', 'Công bố khi phát sinh sự kiện quan trọng.',
   'Luật chứng khoán hiện hành', 'Doanh nghiệp niêm yết', 'Tạo cảnh báo và công bố theo sự kiện.', 'Ghi nhận thời điểm phát sinh chính xác.',
   'Sự kiện có yếu tố bảo mật nội bộ.', 'Thông báo sự kiện và tài liệu đính kèm.', 'Thông báo sự kiện, tài liệu pháp lý liên quan.',
   'Trong vòng 24 giờ kể từ khi phát sinh sự kiện.', 'Bất thường', 'UBCKNN, Website công ty', 'Nhà đầu tư, cổ đông',
   'UBCKNN', 'PDF', 'Rủi ro xử phạt nếu chậm hoặc sai nội dung công bố.', 'Deadline xác định theo thời điểm sự kiện.',
   JSON_ARRAY('Bất thường', 'Sự kiện'), 'seed', 'system'),
  ('dt-custom-obligation', 1, 'Template nghĩa vụ tùy chỉnh', 'Tùy chỉnh', 'Định kỳ', 'Template nội bộ cho nghĩa vụ đặc thù doanh nghiệp.',
   'Quy chế nội bộ doanh nghiệp', 'Theo phân quyền nội bộ', 'Thiết lập workflow và checklist tài liệu.', 'Điều chỉnh theo chính sách nội bộ.',
   'Thay đổi quy trình theo phiên bản template.', 'Nội dung theo template nội bộ.', 'Checklist hồ sơ theo cấu hình template.',
   'Theo cấu hình của admin.', 'Hàng tháng', 'Nội bộ', 'Ban điều hành', 'Nội bộ', 'PDF',
   'Rủi ro tuân thủ nội bộ nếu không thực hiện đúng quy trình.', 'Dùng cho nghĩa vụ nội bộ chưa có mẫu chuẩn ngoài thị trường.',
   JSON_ARRAY('Tùy chỉnh'), 'seed', 'system')
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  category = VALUES(category),
  template_category = VALUES(template_category),
  description = VALUES(description),
  legal_basis = VALUES(legal_basis),
  applicability = VALUES(applicability),
  implementation_content = VALUES(implementation_content),
  implementation_notes = VALUES(implementation_notes),
  special_cases = VALUES(special_cases),
  report_content = VALUES(report_content),
  required_docs = VALUES(required_docs),
  deadline_rule = VALUES(deadline_rule),
  periodicity = VALUES(periodicity),
  channels_text = VALUES(channels_text),
  beneficiaries = VALUES(beneficiaries),
  receiving_authorities = VALUES(receiving_authorities),
  format = VALUES(format),
  legal_risks_text = VALUES(legal_risks_text),
  general_info = VALUES(general_info),
  tags_json = VALUES(tags_json);
