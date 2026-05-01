SET NAMES utf8mb4;

INSERT INTO disclosure_types (type_id, company_id, group_id, active_version_no, status) VALUES
  ('dt-obligation-report', NULL, 'group-001', 1, 'active'),
  ('dt-shareholder-meeting', NULL, 'group-001', 1, 'active'),
  ('dt-disclosure-transaction', NULL, 'group-002', 1, 'active')
ON DUPLICATE KEY UPDATE
  group_id = VALUES(group_id),
  status = VALUES(status);

INSERT INTO disclosure_type_versions (
  type_id, version_no, name, category, template_category, description, legal_basis, applicability, implementation_content,
  implementation_notes, special_cases, report_content, required_docs, deadline_rule, periodicity, channels_text,
  beneficiaries, receiving_authorities, format, legal_risks_text, general_info, tags_json, change_note, updated_by
) VALUES
  ('dt-obligation-report', 1, 'Báo cáo nghĩa vụ', 'Định kỳ', 'Định kỳ', 'Mẫu báo cáo nghĩa vụ tuân thủ định kỳ theo yêu cầu quản trị.',
   'Quy chế công bố thông tin và tuân thủ nội bộ', 'Đơn vị/phòng ban có nghĩa vụ báo cáo', 'Tổng hợp tình trạng thực hiện nghĩa vụ và hành động khắc phục.',
   'Đối chiếu checklist nghĩa vụ trước khi trình duyệt.', 'Nghĩa vụ phát sinh đột xuất được chuyển sang template bất thường.',
   'Báo cáo tổng hợp tình trạng nghĩa vụ, các mục chưa hoàn thành và kế hoạch xử lý.', 'Checklist nghĩa vụ, biên bản rà soát nội bộ.',
   'Trong vòng 05 ngày làm việc kể từ kỳ chốt báo cáo.', 'Hàng tháng', 'Nội bộ, email quản trị', 'Ban điều hành, bộ phận kiểm soát',
   'Nội bộ', 'PDF', 'Rủi ro vi phạm nội bộ và chậm xử lý nghĩa vụ.', 'Dùng cho các nghĩa vụ vận hành/tuân thủ cần theo dõi định kỳ.',
   JSON_ARRAY('Nghĩa vụ', 'Định kỳ'), 'seed-extra', 'system'),
  ('dt-shareholder-meeting', 1, 'Tổ chức đại hội cổ đông', 'Định kỳ', 'Định kỳ', 'Template điều phối công bố thông tin liên quan tổ chức đại hội cổ đông.',
   'Luật Doanh nghiệp và quy chế công bố thông tin hiện hành', 'Doanh nghiệp cổ phần/niêm yết', 'Lập kế hoạch, thông báo mời họp, công bố tài liệu đại hội theo timeline.',
   'Kiểm tra thời hạn chốt danh sách cổ đông và hạn công bố tài liệu.', 'Đại hội bất thường theo nghị quyết đột xuất có thể dùng template bất thường bổ sung.',
   'Thông báo mời họp, chương trình họp, tài liệu đại hội và nghị quyết.', 'Thông báo họp, tài liệu đại hội, danh sách cổ đông, biên bản và nghị quyết.',
   'Ít nhất 21 ngày trước ngày họp (hoặc theo điều lệ/quy định áp dụng).', 'Hàng năm', 'UBCKNN, Sở GDCK, Website công ty', 'Cổ đông, nhà đầu tư, cơ quan quản lý',
   'UBCKNN, HOSE/HNX', 'PDF', 'Rủi ro khiếu kiện và xử phạt nếu công bố không đúng hạn hoặc thiếu hồ sơ.', 'Áp dụng cho kế hoạch và công bố thông tin Đại hội cổ đông thường niên.',
   JSON_ARRAY('Đại hội cổ đông', 'Định kỳ'), 'seed-extra', 'system'),
  ('dt-disclosure-transaction', 1, 'Công bố thông tin về giao dịch', 'Bất thường', 'Bất thường', 'Template công bố thông tin khi phát sinh giao dịch thuộc diện phải công bố.',
   'Luật Chứng khoán và các quy định công bố giao dịch hiện hành', 'Doanh nghiệp niêm yết và chủ thể liên quan giao dịch', 'Tiếp nhận thông tin giao dịch, kiểm tra điều kiện công bố và phát hành thông báo.',
   'Ghi nhận chính xác thời điểm phát sinh giao dịch và thông tin chủ thể liên quan.', 'Giao dịch có yếu tố bảo mật cần kiểm tra phạm vi công bố trước khi phát hành.',
   'Thông tin giao dịch, giá trị, đối tượng liên quan và tài liệu xác nhận.', 'Thông báo giao dịch, tài liệu pháp lý/chứng từ liên quan.',
   'Trong vòng 24 giờ kể từ khi phát sinh hoặc nhận được thông tin giao dịch thuộc diện công bố.', 'Bất thường', 'UBCKNN, Sở GDCK, Website công ty',
   'Nhà đầu tư, cổ đông, cơ quan quản lý', 'UBCKNN, HOSE/HNX', 'PDF', 'Rủi ro xử phạt và ảnh hưởng uy tín khi công bố sai/chậm.',
   'Sử dụng cho các giao dịch phải công bố theo cơ chế bất thường.', JSON_ARRAY('Giao dịch', 'Bất thường'), 'seed-extra', 'system')
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
