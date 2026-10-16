# 07: Deploy DEV và smoke (16/10/2026)

## Bản deploy
| Mục | Giá trị |
|---|---|
| BE | `642e3ce`, deploy bằng `bash deploy-dev.sh be --skip-tests` (api và worker). Sau deploy `/healthz` ok, `/readyz` ready |
| FE | `742f408e`, bundle `index--q4crObz.js`. Deploy bằng `VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED=true VITE_STEP_COMMENT_MENTIONS_V11=true sh deploy-dev.sh fe --skip-tests` |
| Migration | Chỉ `0154`, đẩy bằng `sh deploy-artifacts/push-migration.sh 0154_periodic_cycles_suspend_archived_templates.up.sql`. Ledger có dòng; không có `ERROR` trong output |
| Không chạy | `0155` của task migration runner (`eed28f2`), vì chưa được duyệt cho DEV. Không dùng `deploy-dev.sh migrate/all`, vì hai mode này đẩy mọi migration còn thiếu trong danh sách |
| Thứ tự | 0154, rồi BE, rồi FE |

## Dữ liệu DEV (đọc qua `qa_ro`)
| | Trước | Sau |
|---|---|---|
| Kỳ chưa có hồ sơ của template chung đã lưu trữ | 4 `PENDING` | 4 `SUSPENDED` (`TEMPLATE_ARCHIVED`) |
| Kỳ `PENDING` khác | 10 | 10 (không đổi) |
| Tổng `periodic_cycles` | 346 | 346, ổn định qua nhiều tick của worker |
| Hồ sơ Draft không có template | 0 | 0 |
| Log api/worker 5 phút đầu | | 0 panic, 0 fatal |

## Smoke
- **API** (`scripts/devqa/smoke_mr3_obligation_open.py`, chỉ GET và đọc DB): **15/15**.
  - Cảnh báo c_001: 83 mục, 78 mục có cờ, khớp 78 hồ sơ đang mở của template đã lưu trữ trong DB. Không mục nào gắn cờ sai.
  - `/next` không có kỳ của template đã lưu trữ.
  - Archive-impact `qa-resmoke-periodic-20260904a`: 52 hồ sơ đang mở, 4 kỳ đã lỡ, khớp DB.
  - Template đang hoạt động trả 200; template doanh nghiệp trả 404; ENT trả 403.
  - Dashboard CMS trả 200.
  - Bundle có các chuỗi MR-3.
- **Trình duyệt** (`cobo_web_design/docs/ai-cache/mr3-obligation-open-archive-2026-10-16/smoke-mr3-ui.mjs`; mọi lệnh ghi API ngoài auth bị chặn): **8/8**.
  - Tab Cảnh báo có 78 nhãn.
  - Trang chi tiết hồ sơ của template đã lưu trữ tải được và có nhãn.
  - Dashboard có nhãn.
  - Hộp thoại Khôi phục có "Kỳ đã lỡ trong lúc ngừng cung cấp (4)", 4 ô chưa tick, đóng bằng Huỷ.
  - Hộp thoại Ngừng cung cấp hiện số hồ sơ đang mở theo công ty.
  - Ở 375px, hộp thoại khôi phục vừa màn hình.
  - Không có lệnh ghi nào bị chặn.
- **Không smoke trên DEV:** lưu trữ và khôi phục thật. Hai thao tác này đổi dữ liệu dùng chung (template lên Portal, kỳ bị `SKIPPED`). Đã kiểm bằng 10 test MySQL 8.

## Quan sát
- Trang chi tiết hồ sơ của template đã lưu trữ: Portal gọi `GET /api/v1/disclosure-types/{id}` và `/effective-workflow`, cả hai trả 404. Trang vẫn tải và có nhãn (đã sửa ở `DeadlineDetail.tsx`), nhưng "Loại thông tin báo cáo" hiện "Chưa xác định" và workflow dùng hiển thị dự phòng. Đây là follow-up API-06.
- Dashboard c_001: cả 10 mục "Việc cần làm ngay" là hồ sơ QA của template đã lưu trữ, đúng như đã chốt ở GATE A. Dọn ở MR-8.
- Smoke trình duyệt: `CmsMenuButton` đóng khi có sự kiện scroll. Playwright tự cuộn rồi click thì menu đóng ngay, nên script phải cuộn trước rồi mới click. Người dùng thật cuộn xong mới click nên không bị.

## Rollback
- **FE:** deploy lại `2e31880e` (bundle MR-2 `index-CXR8Bzca.js`) với cùng bộ cờ.
- **BE:** chạy runbook SQL trong `02-contract.md` (đưa kỳ `SUSPENDED` về `PENDING`) trước, rồi deploy BE từ `81924c1`.
