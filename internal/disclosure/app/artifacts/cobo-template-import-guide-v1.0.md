# Hướng dẫn tạo file JSON import template CoBo (schema 1.0)

Phiên bản guide: 1.0

Tài liệu này mô tả cách tạo file JSON để CMS import template ở trạng thái bản nháp. Không chứa token, mật khẩu hay dữ liệu tenant.

## Định dạng gốc

- Mã hóa UTF-8.
- Đúng một đối tượng JSON ở gốc. Không có chữ trước hay sau đối tượng.
- Không bọc trong hàng rào Markdown, không thêm chú thích, không dấu phẩy thừa.
- `schema_version` phải đúng `"1.0"`.
- Trường lạ bị từ chối.

Ví dụ gốc hợp lệ (định kỳ, có `deadline_days`, có thể bỏ `deadline_rule`):

```text
{"schema_version":"1.0","template":{"name":"Báo cáo ví dụ","template_category":"periodic","periodicity":"yearly","applicability_rules":{"applicable_company_classes":["listed"],"applicable_sectors":["commercial"],"deadline_days":20,"deadline_day_type":"calendar"},"workflow":{"steps":[{"stage":"Soạn","processing_days":1,"assignee_roles":["editor"],"department":{"code":"finance","name":"Tài chính"}}]}}}
```

Ví dụ không hợp lệ (có chữ bọc ngoài JSON) sẽ gặp `INVALID_JSON_PAYLOAD`.

## Trường bắt buộc và chu kỳ

- Bắt buộc ở gốc: `schema_version`, `template`.
- Trong `template`: `name`, `template_category` (`periodic` hoặc `irregular`).
- Periodic: `periodicity` chỉ được `daily`, `weekly`, `monthly`, `quarterly`, `yearly`. `event_based` và `ad_hoc` không hợp lệ.
- Irregular: `deadline_rule` bắt buộc. Không cần `periodicity`. Nếu file cũ vẫn ghi `periodicity` thì chỉ `event_based` hoặc `ad_hoc`. Guide này khuyên bỏ `periodicity` và dùng `deadline_rule`.

## Thời hạn

- Periodic: `deadline_rule` không bắt buộc. Nếu `applicability_rules.deadline_days` lớn hơn 0, hệ thống ghi đè `deadline_rule` thành `T+{deadline_days}`, kể cả khi file đang ghi một giá trị khác.
- Periodic thiếu cả `deadline_rule` lẫn `deadline_days` dương thì không đạt.
- Irregular: `deadline_rule` không được rỗng. Hệ thống không suy ra rule từ `deadline_days`.
- Ngày dùng `YYYY-MM-DD`.
- `NEXT` và `NEXT_SLOT` không cần `applicable_from_slot`.

## Phạm vi và phòng ban

- Lớp công ty hợp lệ: `listed`, `large_public`, `non_large_public`.
- Phòng ban dùng `code` và `name` mang đi được. Không ghi UUID của một tenant.
- Nếu mã không khớp catalog, import cần mapping tường minh. Không tự đoán mapping.
- `mapping_required` có thể đúng ngay cả khi một số phòng đã tự khớp. `activation_ready` không phải điều kiện xác nhận bản nháp.

## Workflow và mã template

- Mỗi bước có `stage`. `processing_days` tối thiểu là 1.
- Vai trò phải thuộc danh mục hệ thống hoặc các mã `admin`, `creator`, `viewer`, `publisher`, `editor`.
- Không có workflow thì chặn kích hoạt, không tự chặn tạo bản nháp.
- `type_id` nếu có: `^[a-z0-9][a-z0-9_-]*$`, tối đa 64 ký tự. Mã đã tồn tại sẽ bị từ chối lúc xác nhận.

## Luồng

1. Tải guide và file mẫu.
2. Sửa bản sao.
3. Validate trên CMS.
4. Gán phòng ban còn thiếu.
5. Confirm chỉ khi màn hình cho phép. Kết quả là bản nháp, chưa publish.

## Lỗi thường gặp

| Mã | Cách sửa |
|---|---|
| `INVALID_JSON_PAYLOAD` | Để lại đúng một đối tượng JSON, xóa chữ bọc ngoài và trường lạ |
| `INVALID_PERIODICITY` | Periodic chỉ dùng năm chu kỳ trên |
| `DEADLINE_RULE_REQUIRED` | Thêm `deadline_rule` cho irregular |
| `APPLICABILITY_DEADLINE_DAYS_REQUIRED` | Đặt `deadline_days` lớn hơn 0 cho periodic |
| `INVALID_APPLICABILITY_RULES` | Dùng ba lớp công ty hợp lệ |
| `UNRESOLVED_DEPARTMENT_MAPPING` | Gán phòng ban trên màn hình import rồi xác nhận |
| `INVALID_IMPORT_TOKEN` | Validate lại, không dùng token hết hạn |
| `TARGET_TYPE_CONFLICT` | Chọn `type_id` chưa tồn tại |
