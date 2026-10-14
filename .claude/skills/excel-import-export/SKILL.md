---
name: excel-import-export
description: Dùng khi làm import/export Excel/CSV trong cobo_iam_services (excelize) - lịch nghỉ holiday XLSX, template import, export cấu hình/báo cáo; validate từng dòng, giới hạn kích thước, ngày tháng/timezone, tiếng Việt UTF-8, chống CSV/formula injection, báo lỗi theo dòng.
---

# Excel Import / Export

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Staff Backend Engineer - Data Import/Export & Data Quality.
- **Vai trò hỗ trợ:** Application Security Engineer (formula injection, zip bomb), i18n Engineer (tiếng Việt).
- **Cấp độ:** Staff. **Chuẩn năng lực:** tương đương chuyên gia 15 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- excelize (stream writer, giới hạn unzip), CSV/UTF-8 BOM, ngày Excel serial, chuẩn hóa NFC.
- Validate theo dòng, lỗi `{row, column, code, message}`, import all-or-nothing, upsert theo khóa tự nhiên.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Validate toàn bộ trước, áp dụng trong một transaction; export cùng phân quyền và scope với API đọc.
- Giới hạn kích thước/dòng/sheet; stream file lớn.

**Vấn đề phức tạp phải giải quyết được**
- Ngày tháng mơ hồ, header lệch phiên bản template, import lặp lại idempotent, cập nhật dữ liệu dẫn xuất sau import.

**Rủi ro phải chủ động phát hiện**
- CSV/formula injection, zip bomb, OOM khi buffer toàn bộ, sai múi giờ, mojibake.
- Quên invalidate cache phụ thuộc (holiday ở API và worker).

**Tiêu chí đánh giá chất lượng đầu ra**
- Đặc tả định dạng, mô hình lỗi, giới hạn và test (file hợp lệ, sai header, ngày sai, trùng, quá cỡ) đầy đủ.

## Current state (verify)
- Library: `github.com/xuri/excelize/v2`.
- Holiday XLSX upload (CMS): column A date, B name, C type
  (`internal/holiday`); template served from web `public/holiday-calendar-template.xlsx`.
- Template import (`internal/disclosure`, schema `docs/schema/template-import-v1.*`,
  signing secret `CMS_TEMPLATE_IMPORT_SIGNING_SECRET`).
- Company access config export/versioning (`internal/companyaccess`).

## Import rules
1. Size limits: `http.MaxBytesReader` + max rows/sheets; reject zip bombs
   (excelize options `UnzipSizeLimit`, `UnzipXMLSizeLimit`).
2. Validate header row exactly; map columns by header name, not position,
   when possible.
3. Parse dates explicitly: Excel serial numbers and text formats
   (`dd/MM/yyyy`) in `Asia/Ho_Chi_Minh`; reject ambiguous values.
4. Trim + NFC-normalize text; enforce lengths and enums.
5. Validate all rows first, return row-level errors
   (`{row, column, code, message}`), then apply in one transaction
   (all-or-nothing unless the spec says partial).
6. Idempotent re-import (upsert by natural key), audit log entry with counts.
7. After import, invalidate dependent caches (e.g. holiday caches in API and
   worker) and recompute derived data (deadlines) if required.

## Export rules
1. Authorization and company scope same as the read API.
2. Formula injection: prefix cells starting with `=`, `+`, `-`, `@`, tab or
   CR with `'` (or set as string type).
3. Stream large exports (`excelize` `StreamWriter`); cap rows.
4. Dates as real date cells with explicit format; Vietnamese text UTF-8;
   CSV for Excel needs UTF-8 BOM.
5. File name sanitized; `Content-Disposition` with RFC 5987 encoding.

## Tests
- Valid file, wrong header, bad date, duplicate rows, oversized file.
- Export with Vietnamese text and formula-like values.

## Output format
- Format spec (columns, types)
- Validation and error model
- Limits
- Tests
