# 00: Brief, MR-3 (Q1 bước đầu: nghĩa vụ đang mở của template đã lưu trữ)

## Mục tiêu
Theo quyết định PO Q1-B (`po-decisions-template-workflow-2026-10-16/02-decisions.md`, GATE A đã duyệt 16/10), cùng OQ-006 (24/05) và v1.2 §2.2:
- Hồ sơ đang mở của template đã lưu trữ vẫn là nghĩa vụ:
  - hiện ở tab Cảnh báo, dashboard Portal và "Việc của tôi", kèm nhãn "Mẫu đã ngừng áp dụng";
  - vẫn được nhắc hạn;
  - không sinh kỳ mới, không tạo hồ sơ mới.
- Lưu trữ template thì treo các kỳ chưa sinh hồ sơ. Khôi phục thì Admin Web chọn kỳ nào tạo bù; mặc định không tạo hàng loạt.
- Có một predicate `ObligationOpen` dùng chung, thay cho các bản chép tay.

## Người dùng
- **Doanh nghiệp (Portal):** người dùng thấy nghĩa vụ đang mở và nhãn.
- **Admin Web (CMS, `platform.cms.view` + `cms.template.archive`):** lưu trữ hoặc khôi phục template, xem tác động, chọn kỳ tạo bù.

## Repo
- `cobo_iam_services`: `deadlinealerts`, `platformcms`, `personalops`, `portaldashboard`, `disclosure` (archive/restore, periodic), worker.
- `cobo_web_design`: tab Cảnh báo, "Việc của tôi", dashboard, CMS Templates.

## Hiện trạng (khảo sát 16/10, chi tiết ở `01-design.md`)
- **Tab Cảnh báo:**
  - `INNER JOIN disclosure_types … active_version_no > 0` (`deadlinealerts/app/active_template.go:14-18`), dùng cho cả list, count và "kỳ sắp tới";
  - dashboard Portal dùng lại snapshot này; dashboard CMS (`platformcms/app/dashboard_overview_mysql.go:152-166`) chép lại cùng điều kiện.
  - Bộ lọc này ẩn cả template **chưa từng kích hoạt** (`status='active'`, version 0), không chỉ template đã lưu trữ.
- **"Việc của tôi" và nhắc hạn** không lọc theo trạng thái template (vẫn hiện, vẫn nhắc).
- **Lưu trữ và khôi phục** không đụng `periodic_cycles`:
  - kỳ của template đã lưu trữ vẫn `PENDING/RETRY`, khôi phục là sống lại hết (DL-05);
  - kỳ đã claim mà gặp lưu trữ thì `CreateRecord` trả 409 `TEMPLATE_ARCHIVED`, kỳ chuyển RETRY, rồi tự tạo hồ sơ sau khi khôi phục.
- **DEV (`qa_ro`):**
  - 6 template `qa-…` lưu trữ ngày 11/10 có 312 hồ sơ Draft chưa submit (4 công ty × 78, gồm `c_001`), 321 workflow instance, 4 kỳ chưa sinh hồ sơ;
  - 0 hồ sơ mở của template thật đã lưu trữ.

## Phát hiện ngoài phạm vi Q1 (v1.2 §2.2 chưa được tuân thủ)
1. Ad-hoc đã duyệt không hiện ở tab Cảnh báo. Lý do: hồ sơ được auto-submit, nên trượt điều kiện `draft AND submitted_at IS NULL` (AC-11).
2. "Việc của tôi" lọc `status <> 'draft'`, ngược với v1.2 (AC-13).
3. Các KPI trạng thái mở (#1, #3, #5) của dashboard vẫn lọc theo khoảng ngày.
4. Nhắc hạn không dùng `ObligationOpen`.

Đề xuất: tách thành MR-3b (sửa lỗi theo v1.2, không cần PO quyết). Xem `02-contract.md`.

## Định cỡ
**Normal**, xuyên hai repo, đổi API (bổ sung trường và endpoint), nên đi đủ các phase.
