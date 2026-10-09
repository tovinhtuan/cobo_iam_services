# Release DEV 2026-10-09 (lần 2): C4 + H2, post-deploy và smoke QA

## Đã deploy
| Repo | Danh tính | Mục tiêu |
|---|---|---|
| cobo_iam_services | base `7207a95` + working-tree diff `6bea162cc8ed4f4184aa863583119368de8f11ad` | `make deploy-be` (api + worker), binary 2026-10-09 10:19 (giờ server) |
| cobo_web_design | không đổi (FE giữ nguyên bản đã deploy lần trước) | không deploy |

- Migration: không có.
- Danh tính bản deploy: HEAD `7207a95` đã gồm C2 và C3; phần diff chưa commit là C4 + H2 và tài liệu.
- Rollback point trên server: `bin/api.bak-20261009-c4`, `bin/worker.bak-20261009-c4` (bản có C2 và C3, chưa có C4). Cũng còn `*.bak-20261009` (bản trước C2).
- Các kiểm tra trước deploy (build, `go test ./...` so với HEAD, vet, race, Docker build, review 3 reviewer): xem `docs/ai-cache/bug-cross-tenant-membership-create-2026-10-09/03-verify.md`.

## Kiểm tra sau deploy
| Kiểm tra | Kết quả |
|---|---|
| `/healthz`, `/readyz` qua nginx :3000, `/healthz` :8080 | 200 / 200 / 200 |
| Container | api, worker Up; web healthy |
| Log api và worker sau deploy (20 phút) | 0 dòng ERROR/panic |
| `audit_logs`: số event `admin.user.create`, `admin.membership.create`, `admin.membership.role.assign` trong 40 phút quanh lúc smoke | 0, tức smoke không tạo dữ liệu nào |

## Smoke API (`smoke-c4/api-smoke.txt`): 14/14 PASS
Các request ghi chỉ được gửi sau khi một request đọc chứng minh binary mới đang chạy, và mọi request ghi đều là request kỳ vọng bị từ chối.
- **H2:** tenant admin (`admin_doanh_nghiep` của `c_001`) liệt kê membership của `c_002` → 403 `COMPANY_SCOPE_MISMATCH`.
- **C4:** cùng tài khoản tạo user vào `c_002` và tạo membership vào `c_002` → 403 `COMPANY_SCOPE_MISMATCH`.
- **ROLE-01:** cùng tài khoản gán role `cms_operator` (mang `platform.cms.view`) cho membership của mình → 403 `PERMISSION_DENIED`.
- **Luồng hợp lệ giữ nguyên:** tenant liệt kê membership của `c_001` (5 thành viên), `invite-roles`, `invite-scope` → 200.
- **Platform operator:** liệt kê user của company khác, liệt kê user không có company, liệt kê company qua `/api/v1/platform/cms/admin/*` → 200. Operator gọi route tenant với company khác → 403 `COMPANY_SCOPE_MISMATCH` (route tenant luôn dùng company của token).
- **Tenant trên route platform:** `/platform/cms/admin/users` và `/platform/cms/admin/companies` → 403.

## Smoke UI (Playwright headless, `smoke-c4/results.json` + screenshot): 2/2 PASS
- **Tenant `/app/admin/users`:** danh sách membership của `c_001` tải 200 (3 lời gọi), không bị chuyển sang `/app/forbidden`, không có lỗi console, không có response 403/5xx.
- **CMS `/cms/admin/users`:** chọn một công ty khác `c_001` trong dropdown → `GET /platform/cms/admin/users?company_id=…` và `/roles` trả 200.
- Chỉ đọc; không bấm tạo, mời hay xoá.
- Lần chạy đầu của kịch bản CMS fail do script chưa chọn công ty trong dropdown (màn yêu cầu chọn trước khi gọi API); đã sửa script, không phải lỗi backend.

## Chưa kiểm tra được trên DEV
- Tạo user và membership hợp lệ trong company của mình (happy path ghi). Tránh ghi dữ liệu lên DEV; đã được phủ bằng unit test và handler test.
- Hành vi tab cũ sau khi đổi company (API-01).

## Lưu ý
- Dùng tài khoản seed có password đã lộ trong repo (SEC-01) để smoke. Các tài khoản này vẫn đang hoạt động trên DEV.
- 9 user không có membership do `u_admin_dn` tạo qua route tenant (tìm thấy khi audit) vẫn còn trên DEV; chưa xử lý, cần bạn duyệt riêng.
- Rollback bản backend sẽ mở lại lỗ hổng C4.
