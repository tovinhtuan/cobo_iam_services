# C4 + H2: Verify (Gate V), 2026-10-09

Baseline: worktree sạch của HEAD `7207a95` (đã gồm fix C2 và C3), chạy song song cùng lệnh.

## Thay đổi
- **Mới:** `internal/companyaccess/app/admin_service_company_scope.go`
  - `isPlatformCompanyOperator`: `platform.cms.view` và (`rbac.manage` | `system.settings`), đọc quyền thật từ một lần `GetEffectiveAccess`. Không dùng overlay break-glass vì break-glass chỉ cấp `system.settings` tạm thời trong một company.
  - `resolveTargetCompany(sub, requested, allowNoCompany)`: non-operator bị ghim vào company của token (khác → 403 `COMPANY_SCOPE_MISMATCH`); operator giữ nguyên.
- **Sửa service** (`admin_service.go`, `admin_service_invite_scope.go`): `CreateUser`, `CreateMembership`, `AssignUserToCompany`, `ListCompanyMemberships` (H2), `InviteUser`, `ListInviteRoles`, `ResendUserInvitation`, `authorizeMembershipInvite`, `authorizePlatformCompanyAdmin` (phòng thủ thêm, ngoài plan gốc).
- **Sửa handler tenant** (`admin_handler.go`): `createUser`, `createMembership`, `listMemberships` dùng `tenantRouteCompany`. Company khác token → 403; rỗng → company của token, kể cả với operator.
- **Test mới:** `admin_service_company_scope_test.go` (nhóm `TestScope_`), `admin_handler_company_scope_test.go` (nhóm `TestTenantRoutes_`).
- **Test cũ cập nhật persona** (thêm `platform.cms.view`): 5 test `WebAdmin` trong `admin_service_test.go` (2 test được đổi tên sang `PlatformOperator`), `company_status_allowlist_test.go`, `admin_service_patch_plan_safety_test.go`, `platformcms/.../company_plan_activate_handlers_test.go`.

## Kết quả
| Kiểm tra | Kết quả |
|---|---|
| Gate R | Trước khi sửa, mọi test persona non-operator với company khác token đều FAIL (`02-repro.md`). Test handler persona operator qua route tenant FAIL (201 vào company khác). Sau khi sửa: PASS |
| Kiểm tra ngược helper | Tắt kiểm tra company → 6 nhóm test FAIL. Đổi định nghĩa operator thành chỉ `rbac.manage` → 17 nhóm test FAIL. Khôi phục → PASS |
| `go build ./...` | PASS |
| `go test ./... -count=1` | Exit 1 ở cả fix lẫn baseline. **Không có fail mới.** Fix 19 dòng FAIL, baseline 20 (baseline có thêm `TestSchemaParityConditionalPeriodicity`, test flaky đã gặp ở C2/C3) |
| `go test ./internal/companyaccess/... ./internal/platformcms/...` | Chỉ còn 2 fail có sẵn: `TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`, `TestCreateSelfServiceCompany_FeatureFlagOff` |
| `go vet ./...` | Chỉ lỗi copylocks có sẵn ở `workflowfulfillment/required_document_gate_test.go:326-327` |
| `go test -race -count=1 ./internal/companyaccess/... ./internal/platformcms/...` | Không có DATA RACE; chỉ 2 fail có sẵn |
| `docker compose -f docker-compose.dev.yml build api` | PASS |
| Smoke DEV | Chưa chạy: chưa deploy (cần user yêu cầu) |

## Các chỗ còn dùng `rbac.manage` trong `companyaccess/app` (đã rà)
- Quyền tenant hợp lệ trong company của token: `authorizeMembershipInvite` (lối tắt trong company của mình), `requireRbacManage`, `authorizeDeptMemberMutation` (đã có `requireMembershipInCompany`), `resolveMembershipAdminScope`, `config_*` (delegation, break-glass, export).
- **Chưa xử lý (thuộc C5):** `authorizeScopedMembershipMutation` (`admin_delegation_scope.go`) vẫn trả nil cho company scope, nên các thao tác theo `membership_id` chưa kiểm tra membership thuộc company của token.
- `admin_service.go:989,1014,1032`: `authorize("rbac.manage", ...)` dùng tên permission làm action code, thực tế đòi `system.settings` (follow-up đã ghi ở C3).

## Hợp đồng và tương thích
- Hành vi mới: tenant gửi `company_id` khác token → 403 `COMPANY_SCOPE_MISMATCH` (mã đã có sẵn trong `platform/errors`).
- FE không đổi: mọi caller tenant gửi company của token; CMS dùng `/api/v1/platform/cms/admin/*`, nơi operator vẫn thao tác cross-company bình thường.
- Không có migration.

## Bổ sung sau review (T9)
Reviewer: be-security, admin-role, api-compat. Không có CRITICAL; 1 HIGH (ROLE-01, đã xử lý), còn lại LOW/INFO.

| Finding | Xử lý |
|---|---|
| ROLE-01 (HIGH, xác nhận trong code): `AssignRole` không kiểm tra role mang quyền platform; chỉ chặn khi membership đã có role chính. Tenant admin có thể gán role mang `platform.cms.view` cho một membership chưa có role chính và trở thành platform operator | **Đã sửa:** `assertRoleHasNoPlatformPermissions` (dựa trên `IsEnterprisePermission`) cho non-operator trong `AssignRole`. Test `TestScope_AssignRole_*`: trước fix FAIL (nil), sau fix PASS; role thường vẫn gán được; operator vẫn gán được |
| BES-01 (LOW): resolver không fail-closed khi token không có company | **Đã sửa:** 422 `COMPANY_CONTEXT_REQUIRED`. Test `TestScope_NonOperator_EmptyTokenCompany_FailsClosed`: trước fix FAIL, sau fix PASS |
| Khoảng trắng và chữ hoa của `company_id` | Đã thêm test `TestScope_CompanyIDWhitespaceAndCase` (khoảng trắng được trim, chữ hoa không bị coi là company của mình) |
| API-03 (LOW): hợp đồng mô tả hành vi cũ | Đã cập nhật `docs/api-contracts-json.md` |
| BES-02 (LOW): `CreateMembership` gắn bất kỳ `user_id` tồn tại vào company của mình | Chưa xử lý, follow-up (luồng mời nên thay thế) |
| API-01 (MEDIUM, plausible): tab cũ sau khi đổi company gửi company cũ → 403 | Chưa xử lý, follow-up FE (coi `COMPANY_SCOPE_MISMATCH` là ngữ cảnh cũ, làm mới) |
| ROLE-02 (LOW): còn hai định nghĩa "platform operator" (`isPlatformCMSOperator` chỉ cần `platform.cms.view`, dùng cho các nhánh bỏ qua denylist/validate) | Chưa hợp nhất; ghi follow-up |
| ROLE-03, ROLE-04 (INFO) | ROLE-04 chính là C5 (mutation theo `membership_id` chưa kiểm company); ROLE-03 không ảnh hưởng DEV |

Verify lần cuối sau các sửa đổi: `go build` PASS; `go test ./...` không có fail mới so với HEAD (fix 19 dòng FAIL, HEAD 20, hơn kém nhau test flaky `TestSchemaParityConditionalPeriodicity`); `go vet` chỉ lỗi có sẵn; `-race` không có DATA RACE (chỉ 2 fail có sẵn); Docker build PASS.
