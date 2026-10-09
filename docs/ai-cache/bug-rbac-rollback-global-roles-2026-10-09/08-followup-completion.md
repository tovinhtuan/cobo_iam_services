# ROLE-01 follow-up: completion report

```text
Bug: các việc còn lại của ROLE-01 (nhóm A người duyệt + ROLE-04, B transaction, C test tích hợp, D approval lỗi thời, E FE) và các phát hiện của vòng review.
Reproduction: rbac_restore_integration_test.go (MySQL 8 thật, đỏ trên 7a131a1/10a5700, xanh trên bản sửa), rbac_rollback_scope_test.go, config_approval_approver_test.go, config_approval_notification_test.go, admin_service_company_admin_gate_test.go, rbac_restore_plan_test.go; FE: ApprovalsPanel.test.tsx, useConfigApprovals.test.tsx, configApprovalsApi.test.ts.
Root cause: (1) 0/20 công ty có system.settings nên approval cấu hình không ai duyệt được; (2) restore đọc/tính ngoài transaction, không khoá, đọc qua pool khi giữ tx; (3) approval một thay đổi áp dụng bằng cách hội tụ cả ma trận về snapshot cũ; (4) route queue bỏ qua kiểm tra mà route thường có (notification, quyền mời của primary admin).
Fix (files):
  C: infra/mysql/rbac_restore_integration_test.go, docs 07-integration-tests.md
  B: infra/mysql/queryer.go, admin_repository_rbac_{read,restore}.go, admin_repository_{approval,versioning,conflict_read}.go (đọc qua tx, khoá company/roles/grants, plan trong tx, từ chối critical khi !AllowCritical, snapshot thực sau apply); app/rbac_custom_role.go (version khi tạo/nhân bản/vô hiệu); config_versioning.go; inmemory
  A: config_approval.go (người duyệt rbac.manage hoặc system.settings); admin_service.go (ROLE-04 requireRbacManage + invalidate cache)
  D: config_approval.go (409 APPROVAL_NOTHING_TO_APPLY, compare.changes, ẩn khoá nội bộ), rbac_rollback_approval.go, configversion/types.go (explicit/role_revokes/direct_revokes/plan_digest), rbac_restore_plan.go (explicit plan, digest)
  Review round: ROLE-17 (validate notification qua queue + khi duyệt), ROLE-18 (quyền mời chỉ primary admin cho queue và rollback), ROLE-19 (approval một thay đổi áp dụng đúng điều đã duyệt; rollback gắn dấu vân tay kế hoạch), BES-01 (khoá company), BES-03 (rollback luôn cần rbac.manage), BES-04 (rollback bỏ qua chỉ thị approval), ROLE-22 (invalidate cache)
  E: cobo_web_design ApprovalsPanel.tsx, useConfigApprovals.ts, configApprovalsApi.ts (+ tests)
Verification: go test ./... không fail mới so với 10a5700; go vet (chỉ 2 cảnh báo có sẵn); -race với MySQL; Docker build; revert-check mọi guard (mỗi guard có ít nhất một test đỏ khi gỡ); FE: test mới xanh, tsc không đổi, không có fail mới (63 fail có sẵn). DEV: smoke 39/39 + 32/32, hash dữ liệu ngoài phạm vi không đổi.
Blast radius / data repair: không migration. DEV đã dọn (approval kẹt, dư lượng smoke). Prod: xem ghi chú deploy trong release-2026-10-09/10-role01-followup-post-deploy.md.
```

## Phát hiện của vòng review và cách xử lý
| Reviewer | Phát hiện | Xử lý |
|---|---|---|
| admin-role ROLE-17 | notification patch qua `POST /config-approvals` bỏ qua validate/gói cước | **Đã sửa** (queue + khi duyệt; chỉ `alert_channel_prefs`) |
| admin-role ROLE-18 | quyền mời bị vượt qua bằng rollback/approval | **Đã sửa** |
| admin-role ROLE-19, be-security BES-02 | approval hội tụ cả ma trận, thu hồi nhầm quyền trực tiếp mới | **Đã sửa**: chế độ tường minh + dấu vân tay kế hoạch (kiểm sớm và trong tx) |
| be-security BES-01 | khoá khoảng trống, deadlock hai restore | **Đã sửa** (khoá dòng company đầu tiên) |
| be-security BES-03 | `rbac.manage` chỉ kiểm theo pre-check | **Đã sửa** (luôn cần) |
| be-security BES-04 | chỉ thị approval trong phiên bản lưu | **Đã sửa** (loại bỏ khi rollback); in-memory vs MySQL khác nhau khi lỗi dựng snapshot sau apply: còn lại (INFO) |
| admin-role ROLE-22 | không invalidate cache khi gán/thu hồi admin | **Đã sửa**; chuyển quyền sở hữu hai UPDATE rời: còn lại |
| api-compat API-01 | FE hiện "Request failed: 409" | **Đã sửa** (ánh xạ mã lỗi) |
| api-compat API-02 | FE mới + BE cũ → 403 chuyển trang | **Đã sửa** (`suppressForbiddenNavigation`); thứ tự deploy BE trước |
| api-compat API-05 | `changes` bị `omitempty`; khoá nội bộ lộ | **Đã sửa** |
| api-compat API-04, API-03 | hợp đồng thiếu; cửa sổ rolling deploy | Doc + ghi chú deploy |
| fe-security FES-01..05 | lỗi so sánh bị nuốt, danh sách cũ, Hủy thiếu disabled, race compare, test yếu | **Đã sửa** + test |

## Còn lại (follow-up, không chặn)
1. **ROLE-20/23, BES-05 (policy):** 4-eyes theo membership, người duyệt `rbac.manage`/`system.settings` rộng hơn quyền yêu cầu theo từng aggregate; có thể yêu cầu `rbac.manage` cho aggregate RBAC và quyền cập nhật rule cho notification.
2. **ROLE-21:** admin tenant có thể thu hồi quyền trực tiếp `platform.cms.view` của thành viên trong công ty mình (đã có sẵn qua `DELETE /memberships/{id}/permissions/{code}`; approval chỉ nhất quán với route đó). Cân nhắc chặn mã `platform.*`/`cms.*`.
3. **Chuyển quyền sở hữu** (`TransferOwnership`) vẫn là hai UPDATE rời (không atomic).
4. **API-06:** guard FE của một số nút quản trị công ty rộng hơn BE (`system.settings`).
5. **API-07:** các route approval/rbac-matrix chưa có trong OpenAPI snapshot/Postman.
6. **PERF-10** (`ListMembershipsByCompany` 3N+1) và các mục khác của risk review vẫn mở; nay `invalidateEffectiveAccessForCompany` được gọi thêm ở ba route admin nên nên sửa sớm.
7. In-memory repo nuốt lỗi từng thao tác và không atomic (chỉ dùng cho test).
