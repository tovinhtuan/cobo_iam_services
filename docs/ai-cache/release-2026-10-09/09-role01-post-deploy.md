# Post-deploy ROLE-01 (DEV, backend only) và smoke có ghi

User duyệt smoke có ghi trên DEV để chạy thật các câu SQL mới (2026-10-09).

## Candidate
- HEAD `7a131a1` + diff làm việc ROLE-01 (fingerprint `1b2e7872bf41f7c1`), không commit. Chỉ backend; FE và migration không đổi.
- Rollback point trên server: `bin/api.bak-20261009-r1`, `bin/worker.bak-20261009-r1` (bản C5).
- `make deploy-be` lúc 12:27 (giờ server). `/healthz` và `/readyz` (:3000), `/healthz` (:8080) = 200; api/worker Up; 0 dòng lỗi/panic trong log api và worker sau deploy và sau smoke.

## Smoke có ghi: 38/38 PASS (`smoke-role01/smoke_dev_role01.out`)
Tài khoản tenant admin `admin.dn` (c_001, `m_102`). Dữ liệu ghi được giới hạn và tự trả lại.

| Bước | SQL mới chạy thật | Kết quả |
|---|---|---|
| `SubmitConfigApproval` `rbac.permission.remove` trên `cms_operator` c_001 / role của c_002 | không ghi | 403 `protected_role_read_only` / 404 `NOT_FOUND` (chứng minh binary mới) |
| Role tạm `smoke-r1-*`: gán `dashboard.view` (v5), gán `deadline.view` (v6), rollback về v5 | `DELETE rp FROM role_permissions ... JOIN roles` (`applyRBACRestorePlanTx`) | 200, `{rolled_back_from:5, new_version:{source:"rollback"}}`; role còn `[dashboard.view]` |
| Gỡ `dashboard.view` (v8), rollback về v5 | `INSERT ... SELECT ... FROM (derived) ON DUPLICATE KEY UPDATE` | 200; role có lại `[dashboard.view]` |
| `m_102` cấp trực tiếp `template.workflow.override.approve` (v10), thu hồi (v11), rollback về v10 | `INSERT membership_direct_permissions ... FROM memberships` (`restoreDirectGrantsTx`) | 200; `m_102` có lại quyền |
| Rollback về v11 | `UPDATE membership_direct_permissions SET revoked_at ... AND company_id = ?` | 200; quyền bị thu hồi lại |
| Dọn | gỡ quyền và inactivate role tạm | 200 |
| CMS operator `GET /platform/cms/admin/companies` | không ghi | 200 |

## Kiểm tra trước/sau (SQL chỉ đọc, `smoke-role01/r1_state_{before,after}.out`)
| Kiểm tra | Trước | Sau |
|---|---|---|
| Hash `role_permissions` của mọi role không phải `tenant_custom` c_001 (global, mặc định, `cms_operator`, company khác) | 1086 dòng, `4d99f41f…` | **giống hệt** |
| Hash quyền trực tiếp ngoài `GrantablePermissions` (gồm `platform.cms.view`, `ad_hoc_alert.*`) | 26 dòng, `74fa69d8…` | **giống hệt** |
| `platform.cms.view` trên role CMS | c_001: 2, c_002: 1 | không đổi |
| Dòng quyền trực tiếp active của `m_102` (`template.workflow.override.read` 2, `ad_hoc_alert.propose` 2, `.read` 2) | như trái | không đổi; rollback không tạo dòng trùng mới (BES-04) |
| Approval pending c_001 | 1 (`notification_rule.patch`) | không đổi |

## Dư lượng trên DEV (cố ý, có thể dọn khi cần)
- Role `01a12000-4dd2-771c-8d8a-ff2a866e5fc9` (`smoke-r1-*`): `inactive`, 0 quyền.
- `m_102`: 2 dòng đã thu hồi của `template.workflow.override.approve` (lịch sử; không có dòng active).
- Phiên bản ma trận RBAC c_001 v5–v14 (lý do `ROLE-01 smoke ...`); audit log tương ứng.

## Chưa kiểm được trên DEV
- Nhánh **approval** (`rbac.matrix.rollback` 202 → duyệt → `ApplyPendingApprovalInTx`/`restoreRBACMatrixInTx`): ở c_001 không membership nào có `system.settings` (người duyệt) và không có primary admin (người cấp được quyền critical), nên không tạo được thay đổi critical qua API. Không tự cấp `system.settings` vì ngoài phạm vi smoke. Đường apply dùng chung hai hàm SQL đã chạy ở trên (`applyRBACRestorePlanTx`, `restoreDirectGrantsTx`), chỉ khác transaction bọc ngoài. Nhánh này có test service (in-memory) và test handler cho dạng 202.
- Kiểm tra yêu cầu `rbac.manage` (403 với tài khoản chỉ có `system.settings`): không có tài khoản như vậy ở c_001; có test service.

## Kết luận
GO cho DEV. Cả bốn câu SQL mới chạy đúng trên MySQL thật; dữ liệu ngoài phạm vi không đổi.
