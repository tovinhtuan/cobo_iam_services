# C4: Root cause và solution

## Root cause (confirmed)
Ba lỗi cộng dồn trong `internal/companyaccess`:
1. **Sai tín hiệu "platform admin":** service dùng `hasPermission("rbac.manage")` (`admin_service.go:79,343,636,678,1063`; `admin_service_invite_scope.go:26`) để cho phép thao tác cross-company. `rbac.manage` là quyền **tenant** (chủ công ty tự đăng ký nào cũng có). Quyền platform thật là `platform.cms.view`, hiện chỉ role `cms_operator` có.
2. **Company đích lấy từ client:** route tenant `POST /api/v1/admin/users`, `POST /api/v1/admin/memberships`, `GET /api/v1/admin/companies/{company_id}/memberships` nhận `company_id` từ body hoặc path, không đối chiếu với token.
3. **Authorizer không ràng buộc company của resource:** `Authorize` (`authorization/app/service.go:22-38`, checker `infra/inmemory/checker.go`) chỉ đánh giá quyền trong company của subject và bỏ qua `Resource.ID`. Vì vậy `authorize(..., req.CompanyID)` **không** bảo vệ được company đích.

Layer: API transport + service (authz). Repository làm đúng phần việc của nó: chỉ kiểm tra company tồn tại và role không thuộc company khác.

## Nguyên tắc sửa
> Thao tác lên company khác company của token là **năng lực platform**. Năng lực này được quyết định bởi `platform.cms.view` **và** (`rbac.manage` | `system.settings`), đúng như gate của route `/api/v1/platform/cms/admin/*`. Không bao giờ quyết định bằng một quyền tenant. Route tenant `/api/v1/admin/*` luôn thao tác trên company của token.

Cách sửa này giữ được luồng CMS hợp lệ:
- Role `cms_operator` có cả `platform.cms.view` và `rbac.manage` (seed `0009`, `0063`, `seed_dev_identity_authorization.sql:96,135`).
- FE CMS dùng route `/platform/cms/*`, FE tenant luôn gửi company của chính mình.

## Thiết kế thay đổi (chỉ trong `cobo_iam_services`)

### 1. Helper tập trung (service), `admin_service_company_scope.go` (mới)
```go
// isPlatformCompanyOperator: quyền platform thật (khớp gate route /platform/cms/admin/*).
func (s *adminService) isPlatformCompanyOperator(ctx, sub) (bool, error)
    // = hasPermission(platform.cms.view) && (hasPermission(rbac.manage) || hasPermission(system.settings))

// resolveTargetCompany: company đích hợp lệ cho thao tác admin.
//   platform operator  → giữ requested (kể cả "" = scope không có company, nếu hàm gọi cho phép)
//   còn lại            → requested phải là "" hoặc == sub.CompanyID; trả về sub.CompanyID; khác → 403
func (s *adminService) resolveTargetCompany(ctx, sub, requested string, allowNoCompany bool) (string, error)
```
- Lỗi 403 dùng `CodeCompanyScopeMismatch` (đã có trong `platform/errors`), message rõ ràng.
- Đọc effective access **một lần** cho cả 3 quyền, tránh gọi resolver nhiều lần.

### 2. Thay mọi tín hiệu `isWebAdmin = rbac.manage`
| Vị trí | Hàm | Sửa |
|---|---|---|
| `admin_service.go:79-104` | `CreateUser` | `req.CompanyID, err = resolveTargetCompany(sub, req.CompanyID, allowNoCompany=true)`. Nhánh "user không có company" chỉ cho platform operator |
| `:873-889` | `CreateMembership` | `resolveTargetCompany(sub, req.CompanyID, false)` trước `authorize` |
| `:781-871` | `AssignUserToCompany` | như trên (route CMS vẫn chạy vì operator là platform) |
| `:343/358` | `InviteUser` | thay `isWebAdmin` bằng `isPlatformCompanyOperator`; nhánh no-company chỉ cho platform |
| `:636/645` | `ListInviteRoles` | như trên |
| `:678/682` | `ResendUserInvitation` | như trên |
| `:1063-1087` | `ListCompanyMemberships` | `list_without_company` chỉ cho platform; với `cid` từ path: `resolveTargetCompany(sub, cid, false)` (**sửa luôn H2**) |
| `admin_service_invite_scope.go:25-34` | `authorizeMembershipInvite` | bỏ `return nil` theo `rbac.manage`. Platform operator → nil; còn lại → `cid` phải == `sub.CompanyID`, rồi `authorize` |

Grep lúc implement: `hasPermission(ctx, .*"rbac.manage")` trong `internal/companyaccess/app` để không sót chỗ nào khác dùng `rbac.manage` làm tín hiệu cross-company. Các chỗ dùng `rbac.manage` làm **quyền tenant** (quản RBAC trong chính company) giữ nguyên.

### 3. Defense in depth ở handler tenant (`admin_handler.go`)
- `createUser` (`:160`), `createMembership` (`:235`): nếu body `company_id` khác rỗng và khác `sub.CompanyID` → 403 `COMPANY_SCOPE_MISMATCH`; rỗng → dùng `sub.CompanyID`.
- `listMemberships` (`:345`): path `company_id` khác `sub.CompanyID` → 403.
- Route tenant **không bao giờ** chuyển sang nhánh no-company. Handler CMS (`internal/platformcms`) giữ nguyên, vẫn truyền body company cho service; service cho phép vì caller là platform operator.

### 4. Ngoài phạm vi C4 (ghi follow-up, không làm trong PR này)
- **C5:** IDOR theo membership id (`AssignRole`, `UpdateMembership`, `DeleteMembership`…) và việc `ensureRoleForMembership` chấp nhận global role. Đây là bước 2 của "Đường 2"; C4 đã chặn bước 1 (không tạo được membership trong B), nhưng C5 vẫn cần sửa riêng.
- **Root chung:** authorizer bỏ qua `Resource` company (gốc của C4, C5, H2). Sửa ở tầng authorization là thay đổi hệ thống, cần thiết kế riêng.
- `POST /api/v1/admin/memberships` cho phép tenant admin thêm **bất kỳ user nào** trên platform vào company mình, không cần user đồng ý. Đây là rủi ro thấp hơn; cân nhắc gỡ route (FE không dùng) hoặc chuyển sang luồng invite.

## Kế hoạch tái hiện (Gate R), viết trước khi sửa
Fake auth trong `admin_service_test.go` cần trả permission theo persona (kiểm tra fake hiện có; mở rộng nếu chỉ có allow/deny chung).

| Test | Persona | Kỳ vọng sau fix | Hiện tại |
|---|---|---|---|
| CreateUser company_id=c_002, role admin | tenant admin c_001 (`rbac.manage`, `admin.membership.invite`) | 403 `COMPANY_SCOPE_MISMATCH` | tạo được → **FAIL** |
| CreateUser company_id="" | tenant admin | ép về c_001 (không tạo user no-company) | tạo user no-company → **FAIL** |
| CreateMembership company_id=c_002 | tenant admin | 403 | tạo được → **FAIL** |
| ListCompanyMemberships cid=c_002 (H2) | tenant admin | 403 | trả danh sách → **FAIL** |
| ListInviteRoles / Resend / Invite với company khác hoặc no-company | tenant admin | 403 | pass → **FAIL** |
| AssignUserToCompany c_002 | tenant admin | 403 | pass → **FAIL** |
| Các case trên | platform operator (`platform.cms.view` + `rbac.manage`) | cho phép như cũ | pass |
| CreateUser/CreateMembership cùng company | tenant admin | cho phép như cũ | pass |
| Handler `POST /api/v1/admin/users` body company khác token | tenant | 403 | 201 → **FAIL** |

- Đổi tên và đổi persona test `TestAdminService_CreateUser_WebAdminCanCreateOtherCompany` (`admin_service_test.go:338`) thành `PlatformOperatorCanCreateOtherCompany`. Test hiện đang **khẳng định hành vi lỗi**.
- Giữ `TestIntegration_platformCMSPrefix_adminUsersCreateAndList` (`server_test.go:984`) xanh: luồng CMS cross-company hợp lệ.
- Thêm test quét: mọi route `/api/v1/admin/*` nhận `company_id` (body/path/query) với token tenant và company khác phải trả 403. Có thể làm bằng bảng route trong handler test.

## Verify (Gate V)
- `go test ./internal/companyaccess/... ./internal/platformcms/... ./internal/httpserver/...` và `go test ./...` so với baseline (không có fail mới).
- `go vet ./...`, `go test -race ./internal/companyaccess/...`, `docker compose -f docker-compose.dev.yml build api`.
- Smoke DEV (khi được duyệt deploy):
  - Tenant admin `POST /api/v1/admin/users` với company khác → 403.
  - `GET /api/v1/admin/companies/{khác}/memberships` → 403.
  - Tenant tạo user trong company mình → 201.
  - CMS `/cms/admin/users` tạo, mời, liệt kê cho company khác → OK.

## Blast radius và audit dữ liệu
- **Ai mất quyền:** chỉ những ai có `rbac.manage` mà **không** có `platform.cms.view` và đang thao tác cross-company qua route tenant. Theo seed: `admin_web` (…011), `full_access`, `self_reg_company_owner`, `admin_doanh_nghiep`. FE không dùng luồng này. Script hoặc Postman nào dựa vào `rbac.manage` để tạo user cho company khác cần chuyển sang route CMS bằng tài khoản `cms_operator`.
- **Hợp đồng FE:** không đổi với FE hiện tại. Lỗi mới 403 `COMPANY_SCOPE_MISMATCH` chỉ xuất hiện khi gửi company khác token.
- **Audit dữ liệu, cần user duyệt chạy read-only trên DB DEV:**
```sql
-- Membership tạo qua route tenant mà company đích khác company của actor
SELECT a.occurred_at, a.actor_user_id, a.company_id AS actor_company, m.company_id AS target_company, m.membership_id, m.user_id
FROM audit_logs a
JOIN memberships m ON m.membership_id = a.resource_id
WHERE a.action = 'admin.membership.create' AND m.company_id <> a.company_id;
-- Tương tự cho action ghi bởi createUser (xác nhận tên action trong admin_handler lúc implement)
-- Loại trừ actor có platform.cms.view (thao tác CMS hợp lệ), dùng effective_permissions_snapshot nếu có
```
Nếu phát hiện membership hoặc user lạ: lập danh sách theo tenant để vô hiệu hoá. Không tự chạy thao tác ghi.

## Mitigation tạm thời (trước khi deploy fix, cần user duyệt)
- Chặn ở nginx: `POST /api/v1/admin/memberships` (FE không dùng).
- `POST /api/v1/admin/users` **không chặn được** vì FE tenant đang dùng. Cần deploy fix sớm.
- Lưu ý: cổng 8080 đang mở thẳng ra ngoài (H23), nên rule nginx không đủ.

## Review sau implement
`be-security-reviewer` + `admin-role-reviewer` (+ `api-compat-reviewer` vì có mã lỗi mới), sau đó `premerge-system-review`.

## Quyết định cần user
1. Định nghĩa "platform operator" = `platform.cms.view` **và** (`rbac.manage` | `system.settings`), khớp gate route CMS? (khuyến nghị) Hay chỉ cần `platform.cms.view`?
2. Route tenant gửi `company_id` khác token: trả **403** (khuyến nghị, lộ lỗi sớm) hay âm thầm ép về company của token?
3. `POST /api/v1/admin/memberships` (FE không dùng): **giữ, ép company theo token** (khuyến nghị cho PR này) hay gỡ hẳn?
4. Gộp sửa H2 vào cùng PR (khuyến nghị, cùng root, cùng helper)?

## Quyết định đã chốt (user, 2026-10-09)
1. Platform operator = `platform.cms.view` **và** (`rbac.manage` | `system.settings`).
2. Route tenant nhận `company_id` khác token → **403** `COMPANY_SCOPE_MISMATCH`.
3. `POST /api/v1/admin/memberships`: **giữ**, ép company theo token.
4. Gộp sửa **H2** vào cùng PR.
5. Đã cho phép và đã chạy query read-only trên DB DEV. Kết quả thô nằm ở output trong scratchpad của session, chưa đưa vào repo.
