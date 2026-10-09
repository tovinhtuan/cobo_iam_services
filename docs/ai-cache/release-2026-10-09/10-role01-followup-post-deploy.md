# Post-deploy ROLE-01 follow-up (DEV, backend)

DEV là dữ liệu mẫu, được phép ghi (user, 2026-10-09). Candidate: HEAD `10a5700` ("fix bug ROLE-01") + thay đổi follow-up chưa commit (fingerprint `c36d8895cf6b0dce`). Chỉ backend (FE chưa deploy). Rollback point trên server: `bin/api.bak-20261009-r1b`, `bin/worker.bak-20261009-r1b` (bản ROLE-01 đã deploy trước đó); bản C5 vẫn ở `*.bak-20261009-r1`.

`make deploy-be` OK; `/healthz`, `/readyz` (:3000) và `/healthz` (:8080) = 200; log api và worker 0 dòng lỗi/panic sau deploy và sau smoke.

## Smoke nhánh duyệt: 39/39 PASS (`smoke-role01-approval/smoke_dev_role01_approval.out`)
Người yêu cầu: `admin.dn` (`m_102`). Người duyệt: `platform.tenant.admin` (`m_107`), một thành viên khác của c_001 chỉ có `rbac.manage` (không có `system.settings`).

| Bước | Kết quả |
|---|---|
| ROLE-04: `POST /company/admins` với membership không tồn tại | 404 `MEMBERSHIP_NOT_FOUND` (qua được cổng `rbac.manage`; trước đây 403 từ `system.settings`) |
| Role tạm giữ quyền critical `rbac.manage` (chèn bằng SQL vì API không cho) | role có `dashboard.view`, `deadline.view`, `rbac.manage` |
| `DELETE /roles/{id}/permissions/{rbac.manage}` | **202** body phẳng `{approval_id,status:"pending"}`; dữ liệu chưa đổi |
| `GET /config-approvals/{id}/compare` | `changes` có đúng 1 mục: gỡ `rbac.manage` khỏi role tạm, `critical: true` |
| Người yêu cầu tự duyệt | 403 `SELF_APPROVAL_NOT_ALLOWED` |
| Admin thứ hai (chỉ `rbac.manage`) duyệt | **200**; `ApplyPendingApprovalInTx` chạy thật; role còn `dashboard.view`, `deadline.view`; phiên bản mới nhất `source=approval_apply` |
| Rollback về phiên bản đó khi quyền critical quay lại | **202**; duyệt → 200; quyền bị gỡ (`rbac.matrix.rollback` chạy thật trên MySQL) |
| Approval không còn gì để áp dụng (quyền đã bị gỡ ngoài hàng đợi) | duyệt → **409** `APPROVAL_NOTHING_TO_APPLY`; từ chối → 200 |
| Hash `role_permissions` của mọi role ngoài role custom c_001 (1086 dòng) | **giống hệt** trước/sau |
| Dọn | gỡ quyền, vô hiệu hoá role tạm |

## Dọn approval kẹt (D-T3)
- c_001: `notification_rule.patch` từ tháng 7 → từ chối qua API.
- `144ca32b…`: 2 approval từ tháng 7 (`rbac.direct_permission.remove`, `notification_rule.patch`) → huỷ bằng SQL vì không có tài khoản đăng nhập của công ty đó (`reject_reason = "obsolete sample data (ROLE-01 follow-up cleanup)"`).
- Sau dọn: không còn approval `pending` nào.

## Dư lượng trên DEV
Role tạm `custom_smoke_appr_*` (`inactive`, 0 quyền); 4 approval của smoke ở trạng thái approved/rejected; phiên bản RBAC c_001 v15 trở đi; 2 approval cũ của `144ca32b…` ở trạng thái `cancelled`.

## Chưa kiểm
- FE chưa deploy: nhãn `rbac.matrix.rollback`, nút duyệt theo `rbac.manage`, danh sách `changes` mới có trong repo web nhưng chưa build/deploy lên DEV.
- Nhánh "mismatch trong transaction" (rollback chuyển sang approval giữa chừng): kiểm bằng test tích hợp (`RestoreInTx_RefusesCriticalUnlessAllowed`) và test service với repo giả gây race; không tái hiện được qua API vì cần chen thay đổi đúng lúc.

## Vòng 2 (sau review): deploy lại và smoke
Candidate: HEAD `10a5700` + thay đổi follow-up chưa commit (fingerprint `897ade8e455ac362`). Rollback point: `bin/api.bak-20261009-r1c`, `bin/worker.bak-20261009-r1c` (bản follow-up vòng 1). `:3000` trả 502 vài giây đầu khi nginx chờ upstream, sau đó `/healthz` và `/readyz` = 200; log api/worker 0 dòng lỗi.

Smoke vòng 2: **32/32 PASS** (`smoke-role01-approval2/`):
| Bước | Kết quả |
|---|---|
| ROLE-17: `notification_rule.patch` với payload sai (`version: 2`) qua `POST /config-approvals` | 400 `INVALID_REQUEST`, không xếp hàng |
| ROLE-18: admin không phải primary xếp hàng gỡ `admin.membership.invite` | 403 `PERMISSION_DENIED` |
| ROLE-19 một thay đổi: xếp hàng gỡ `rbac.manage` khỏi role tạm, rồi ghi quyền trực tiếp mới bằng SQL (không tạo phiên bản, như luồng mời người dùng), duyệt | 200; `rbac.manage` bị gỡ; **quyền trực tiếp ghi sau vẫn còn** |
| ROLE-19 rollback: rollback critical về phiên bản mới nhất → 202, rồi ghi thêm một quyền trực tiếp, duyệt | **409 `STALE_PROPOSAL`**, không áp dụng gì (role vẫn giữ quyền, quyền trực tiếp còn nguyên) |
| Tạo role tùy chỉnh trong lúc approval đang chờ, duyệt | 409 `STALE_PROPOSAL` |
| Hash `role_permissions` ngoài role custom c_001 (1086 dòng) | giống hệt trước/sau |
| Dọn | role tạm `inactive`; quyền trực tiếp của `m_102` về đúng trạng thái ban đầu; không còn approval `pending` |

Ghi chú smoke: một approval `pending` tạm bị tạo ở lần chạy đầu (script nhận diện sai mã rule `company.alert_channel_prefs.v1`, nên một patch hợp lệ được xếp hàng đúng) và đã từ chối ngay.

## Ghi chú khi deploy ra môi trường dùng chung
- Thứ tự: **BE trước, FE sau**. FE mới với BE cũ khiến người chỉ có `rbac.manage` thấy nút Duyệt nhưng 403 (FE đã chặn chuyển trang forbidden khi quyết định, nên chỉ hiện thông báo).
- Trước khi cuộn bản mới: xử lý (từ chối hoặc huỷ) các approval `pending` loại `rbac.direct_permission.remove` đang chờ. Bản cũ khi duyệt một yêu cầu do bản mới tạo (có `explicit`/`direct_revokes`) sẽ coi là đã duyệt nhưng không gỡ gì cho mã không thuộc danh sách grantable; bản mới khi duyệt yêu cầu do bản cũ tạo cho role được bảo vệ trả 409 `APPROVAL_NOTHING_TO_APPLY`.
- FE (`cobo_web_design`) chưa build/deploy lên DEV: nhãn `rbac.matrix.rollback`, nút duyệt theo `rbac.manage`, danh sách thay đổi, thông báo lỗi theo mã.
