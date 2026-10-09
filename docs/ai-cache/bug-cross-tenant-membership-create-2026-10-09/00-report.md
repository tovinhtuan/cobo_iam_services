# Bug C4: Chiếm quyền tenant khác qua tạo user/membership (2026-10-09)

Nguồn: `docs/ai-cache/risk-review-2026-10-09/10-risk-report.md` (C4). Workflow: `wf-bugfix`. Giai đoạn: **phân tích + solution, chưa sửa code**.

## Kỳ vọng và thực tế
- **Kỳ vọng:** route tenant `/api/v1/admin/*` chỉ thao tác trên company của access token. Thao tác sang company khác chỉ dành cho platform operator qua `/api/v1/platform/cms/admin/*` (gate `platform.cms.view` + `rbac.manage|system.settings`).
- **Thực tế:** service dùng `rbac.manage` làm tín hiệu "web/platform admin". Đây là quyền mọi chủ công ty tự đăng ký đều có (`iam/registrationmysql/register_public.go:204`, global role `self_reg_company_owner`). Kết hợp với `company_id` lấy từ body và authorizer không so company của resource, tenant admin A thao tác được trên company B.

## Các đường tấn công (đã đọc code xác nhận)

### Đường 1: tạo admin mới cho tenant B, một bước (nghiêm trọng nhất)
`POST /api/v1/admin/users` (`admin_handler.go:68,160-217`). Body có `company_id`, `role_code`, `login_id`, `password`.
- `CreateUser` (`admin_service.go:74-104`):
  - `authorize("admin.membership.create", Subject.CompanyID)` đánh giá trong company A.
  - `isWebAdmin = hasPermission("rbac.manage")` là true với owner A, nên **bỏ qua** đoạn ép `CompanyID = Subject.CompanyID` (`:99-104`).
- `validateEnterpriseInviteRole(B, role_code="admin_doanh_nghiep")`: role admin theo company được tạo khi đăng ký (`register_public.go:124-133`) **không** nằm trong `EnterpriseInviteRoleDenylist` (`enterprise_invite_roles.go:6-14`).
- `repo.CreateUser` tạo user, credential (password do attacker đặt) và membership B với role admin của B, trong cùng một transaction (`admin_repository.go:27-94`).
- ⇒ Attacker đăng nhập bằng tài khoản mới và là **admin của tenant B** (`rbac.manage` trong B).

### Đường 2: tự gắn mình vào B rồi leo quyền
`POST /api/v1/admin/memberships` (`admin_handler.go:69,235-254`).
- `user_id`, `company_id`, `status` đều lấy từ body.
- `CreateMembership` (`admin_service.go:873-889`) chỉ gọi `authorize(..., req.CompanyID)`. Authorizer bỏ qua resource company (`authorization/infra/inmemory/checker.go`, chỉ so `Subject.CompanyID` với chính effective access của subject).
- ⇒ Tạo được membership `active` của chính attacker trong B. Không có role, chỉ có `template.workflow.override.read`.
- Leo quyền tiếp bằng C5 (IDOR theo membership id): từ token A, `POST /api/v1/admin/memberships/{membership_B}/roles` với role global `self_reg_company_owner`. `AssignRole` không kiểm scope, và `ensureRoleForMembership` (`admin_repository.go:889-905`) chấp nhận role có `company_id IS NULL`. Kết quả là owner của B.

### Đường 3: đọc và liệt kê (H2 + các bypass liên quan)
`authorizeMembershipInvite` (`admin_service_invite_scope.go:25-34`): người có `rbac.manage` thì `return nil`. Hệ quả:
- `GET /api/v1/admin/companies/{B}/memberships`: lộ nhân sự và email của B (H2).
- `ListInviteRoles`, `ResendUserInvitation`, `ListCompanyMemberships` (no-company scope): bypass bằng `rbac.manage` (`admin_service.go:636,678,1063`).
- `InviteUser` (`:343`): route tenant ép company theo token ở handler (`:1128`), nên chỉ route CMS (đã có gate) dùng được nhánh cross-company.
- `AssignUserToCompany` (`:781-871`): chỉ `authorize(..., companyID)`. Hiện chỉ đi được qua route CMS (đã có gate); nếu sau này có route tenant gọi tới thì cũng lỗ.

## Điều kiện khai thác
- Có tài khoản owner/admin tenant bất kỳ (tự đăng ký miễn phí).
- Biết `company_id` của nạn nhân. Company tự đăng ký có id UUIDv4; company seed (`c_001`, `c_002`…) đoán được.
- Kênh lộ company_id chưa được khảo sát hết; plausible qua C5/H2 hoặc dữ liệu chia sẻ.

## Ai đang gọi các API này (FE `cobo_web_design`)
- `POST /api/v1/admin/users`, `GET /api/v1/admin/companies/{id}/memberships`, `POST /api/v1/admin/users/invite` chỉ được gọi từ tenant admin `/app/admin/*`. Company luôn là `selectedCompany.id` (company của token). Invite không gửi company_id.
- **Không có** caller FE nào cho `POST /api/v1/admin/memberships`.
- CMS (`/cms/admin/users`, `/cms/admin/companies/:id`) thao tác cross-company **chỉ** qua `/api/v1/platform/cms/admin/...`.
- ⇒ Ép company theo token cho route tenant **không làm vỡ** luồng FE hiện có.

## Mức độ
**CRITICAL**: leo thang đặc quyền cross-tenant, một request là chiếm quyền admin của tenant khác. Điều kiện duy nhất là biết `company_id` của nạn nhân.
