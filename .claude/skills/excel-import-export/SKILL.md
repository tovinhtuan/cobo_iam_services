---
name: excel-import-export
description: Dùng khi làm import/export Excel/CSV trong cobo_iam_services (excelize) - lịch nghỉ holiday XLSX, template import, export cấu hình/báo cáo; validate từng dòng, giới hạn kích thước, ngày tháng/timezone, tiếng Việt UTF-8, chống CSV/formula injection, báo lỗi theo dòng.
---

# Excel Import / Export

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
