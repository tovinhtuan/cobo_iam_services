# Bug C5: Thao tác theo `membership_id` không kiểm tra membership thuộc company của token (2026-10-09)

Nguồn: `docs/ai-cache/risk-review-2026-10-09/10-risk-report.md` (C5, ROLE-04, BES-03); tiếp nối C4. Workflow: `wf-bugfix`. Giai đoạn: **phân tích và solution, chưa sửa code**.

## Kỳ vọng và thực tế
- **Kỳ vọng:** mọi route tenant `/api/v1/admin/*` chỉ đọc hoặc ghi dữ liệu của company trong access token. Với route nhận `membership_id` (hoặc id của đối tượng thuộc membership), membership đó phải thuộc company của token.
- **Thực tế:** phần lớn service theo `membership_id` chỉ gọi `authorize(..., membershipID)`. Authorizer bỏ qua resource (xem C4), nên không có kiểm tra thuộc company. Helper `requireMembershipInCompany` và `repo.MembershipBelongsToCompany` đã tồn tại nhưng chỉ được gọi ở 5 nơi. `authorizeScopedMembershipMutation` trả `nil` ngay khi scope là "company".
- Phía SQL: các câu `UPDATE/DELETE/SELECT` theo `membership_id` không có `company_id`.

## Phạm vi tiếp cận
- Toàn bộ hàm bị ảnh hưởng chỉ được gọi từ `AdminHandler` (route tenant), xác nhận bằng grep: `platformcms` và các package khác không gọi chúng. ⇒ **Một quy tắc duy nhất** cho mọi hàm này: membership đích phải thuộc `Subject.CompanyID`. Không cần ngoại lệ cho platform operator (operator làm việc cross-company qua `/api/v1/platform/cms/*`).
- Điều kiện để tác động: biết UUID của membership hoặc đối tượng ở company khác (không đoán được bằng cách thông thường; chưa xác định được kênh lộ UUID sau khi H2 đã được sửa). Vì vậy đánh giá: **HIGH, có thể thành CRITICAL** nếu tìm ra kênh lộ id.

## Kiểm kê (đã xác minh bằng đọc code)

### A. Chưa có kiểm tra thuộc company (cần sửa)
| Route | Service | Ghi chú |
|---|---|---|
| `PATCH /memberships/{id}` | `UpdateMembership` | ghi trạng thái; SQL không có company_id |
| `DELETE /memberships/{id}` | `DeleteMembership` | xoá membership và các bảng con; SQL không có company_id |
| `POST/DELETE /memberships/{id}/roles` | `AssignRole`, `RemoveRole` | role được kiểm theo company của membership đích, không theo company của token |
| `PUT /memberships/{id}/primary-role` | `ReplaceMembershipPrimaryRole` | validate theo `member.CompanyID` (của đích) |
| `POST/DELETE /memberships/{id}/departments` | `AssignDepartment`, `RemoveDepartment` | |
| `POST/DELETE /memberships/{id}/titles` | `AssignTitle`, `RemoveTitle` | chỉ `authorize` |
| `PUT /memberships/{id}/org-assignments` | `UpdateMembershipOrgAssignments` | validate theo company của đích |
| `GET/POST/DELETE /memberships/{id}/permissions` | `ListDirectPermissions`, `AddDirectPermission`, `RemoveDirectPermission` | INSERT ghi `company_id` của người gọi với membership của người khác; SQL revoke/list theo `membership_id` |
| `DELETE /company/admins/{id}` | `RevokeCompanyAdmin` | lookup role theo company của token nhưng không kiểm membership đích |
| `POST/DELETE /teams/{id}/members` | `AddTeamMember`, `RemoveTeamMember` | INSERT ghi company của người gọi; team và membership đều không được validate; `RemoveTeamMember` bỏ qua tham số company |
| `DELETE /titles/{id}/members/{mid}` | `RemoveTitleMember` | |
| `POST/DELETE /departments/{id}/members` | `AddDeptMember`, `RemoveDeptMember` | nhánh người gọi không có `rbac.manage` (dựa trên delegation) không có kiểm tra company |
| `POST /config-approvals` loại `RBAC_DIRECT_PERM_REMOVE` | `SubmitConfigApproval` | `membership_id` lấy từ body, không validate |

### B. Đã có kiểm tra (giữ nguyên, thêm test)
`AssignCompanyAdmin`, `TransferOwnership` (đích), `AddTitleMember` (`requireMembershipInCompany`), delegation, break-glass, `AddDeptMember`/`RemoveDeptMember` nhánh `rbac.manage`, department head. Các thực thể không theo membership (delegation, config approval, break-glass, notification rule, role, department PATCH) đã lọc `company_id` ở SQL.

### C. Các khuyết điểm liên quan thực thể không theo membership
| Vị trí | Vấn đề |
|---|---|
| `DeleteTeamRow` (`admin_repository_teams.go:90`) | xoá `org_unit_memberships WHERE org_unit_id=?` **trước** khi kiểm company của team, không trong transaction (đã ghi ở ROLE-05/BES-03) |
| `CreateTeam` / `CountTeamsInDepartment` | `department_id` không được validate thuộc company; count không lọc company |
| `DeleteDepartment` / `DeleteTitle` | `CountDepartmentMembers`/`CountTitleMembers` theo id, không lọc company; id của company khác nhận 409 thay vì 404 (lộ sự tồn tại) |

## Bằng chứng dữ liệu
- Audit DEV đã chạy read-only ở bước C4 (Q3): **0 dòng** event `admin.membership.*` (trừ `create`) mà membership đích thuộc company khác company của actor. Giới hạn: chỉ các event qua `auditLog` của handler; team, title, department và direct permission có thể không được ghi audit.

## Test hiện có và khoảng trống
- Chưa có test nào gọi các hàm ở bảng A với một membership **tồn tại ở company khác**. Các test cross-tenant hiện có chỉ phủ role (`TestPhaseE_AssignCrossTenantCustomRoleRejected`), config export, break-glass, và nhóm `TestScope_*`/`TestTenantRoutes_*` của C4.
