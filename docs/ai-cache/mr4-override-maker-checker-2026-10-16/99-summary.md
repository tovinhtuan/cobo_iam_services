# 99: Tổng kết MR-4 (maker-checker override workflow theo quyền, PO Q5-B)

| Mục | Giá trị |
|---|---|
| Gate A (contract) | Duyệt bởi user ngày 16/10/2026. Bốn lựa chọn đều theo khuyến nghị (xem 02-contract) |
| Gate B (premerge) | Ba reviewer, không có Critical. High duy nhất (API-01) đã sửa. Các Medium đã sửa hoặc đã giảm thiểu, có lý do (06-review) |
| Commit | BE `253c65f`, FE `95363723`. Docs deploy/smoke ở commit kế tiếp |
| DEV | 0156 → BE → FE. Bundle `index-D8LVrMPp.js`. Smoke API 20/20, trình duyệt 9/9 |

## Đã giao
- **Quyền:** mọi đường kích hoạt (PUT `publish:true`, approve, rebase-apply) cần `template.workflow.override.approve`. Người duyệt kích hoạt được bản của chính mình; audit ghi `self_approved`.
- **Trạng thái:** `draft` → `pending_approval` (gửi duyệt, quyền ghi) → `approved` (duyệt) hoặc `rejected` (từ chối, có lý do). Người có quyền duyệt đi thẳng `draft` → `approved`.
- **Nội dung version bất biến:** sửa nhóm bước tạo version mới; bản nháp bị xoá giữ dòng `discarded`. Vì vậy duyệt `v:N` luôn kích hoạt đúng nội dung người duyệt đã thấy.
- **Rebase:**
  - rebase-apply cần quyền duyệt và trả 409 khi đang có bản chờ duyệt;
  - `self_approved` tính từ người đã xử lý xung đột;
  - `merge_manual` cần giá trị hợp lệ theo từng field;
  - lựa chọn hợp lệ lấy theo loại xung đột.
- **Portal:**
  - "Gửi duyệt" cho người chỉ có quyền ghi; "Kích hoạt" chỉ dành cho người duyệt;
  - panel trạng thái có Duyệt/Từ chối và phần xem nội dung so với bản đang dùng;
  - thẻ chờ duyệt trên màn "Loại thông tin báo cáo";
  - lỗi 403 hiện tại chỗ, không chuyển trang.

## Follow-up
- Thông báo in-app cho người duyệt khi có bản gửi duyệt, hoặc khi có người tự duyệt (đã chốt ở GATE A).
- Ô nhập giá trị cho `merge_manual` (đã chốt ở GATE A). Đến lúc đó, xung đột rule 5/7 và `step_identity_unclear` không xử lý được trên Portal.
- Phương án C: doanh nghiệp tự chọn bắt buộc người duyệt khác.
- PUT draft chưa kiểm loại thông tin có thuộc công ty không (có từ trước, ROLE-07).
- Luồng tạo loại nhanh trong đề xuất ad-hoc không giữ lại các bước khi `publish` lỗi (API-10).
- `push-migration.sh` luôn exit 0 (đã ghi ở `migration-runner-gaps-2026-10-16.md`).
- Test còn thiếu: meta audit ở mức handler, MySQL chéo tenant, FE với persona chỉ có quyền duyệt.
- Có thể dọn hẳn các version thử trên loại QA ở DEV (cần root DB, chờ user duyệt).
