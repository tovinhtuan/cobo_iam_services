# C5: Verify (Gate V), 2026-10-09

Baseline: worktree sạch của HEAD `3203961` (đã gồm C4), chạy song song cùng lệnh.

## Thay đổi
- **Service (lớp 1):** `requireTargetMembership` (`admin_service_company_scope.go`): fail-closed khi token không có company, còn lại gọi `requireMembershipInCompany` → 404 `MEMBERSHIP_NOT_FOUND`.
  - Gọi tập trung ở đầu `authorizeScopedMembershipMutation` (phủ `UpdateMembership`, `DeleteMembership`, `ReplaceMembershipPrimaryRole`, `AssignDepartment`, `RemoveDepartment`, `UpdateMembershipOrgAssignments`, nhánh delegation của `AddDeptMember`/`RemoveDeptMember`).
  - Gọi tường minh ở `AssignRole`, `RemoveRole`, `AssignTitle`, `RemoveTitle`, `AddDirectPermission`, `RemoveDirectPermission`, `ListDirectPermissions`, `RevokeCompanyAdmin`, `AddTeamMember`, `RemoveTeamMember`, `RemoveTitleMember`, `AddTitleMember`; `AssignCompanyAdmin` và `TransferOwnership` đổi sang helper này (đồng nhất mã 404 `MEMBERSHIP_NOT_FOUND`).
  - `submitRBACDirectPermRemoveApproval` (config approval) kiểm tra membership trước khi xếp hàng.
  - `ReplaceMembershipPrimaryRole`, `UpdateMembershipOrgAssignments` validate theo `sub.CompanyID` (bỏ lookup `member.CompanyID`).
- **Team/department/title (lớp 2):** `requireTeamInCompany` + repo `TeamBelongsToCompany`; `CreateTeam` validate department bằng `departmentInCompany`; các hàm đếm (`CountDepartmentMembers`, `CountTitleMembers`, `CountTeamsInDepartment`) nhận company và lọc theo company (id ngoài company đếm 0 → 404 thay vì 409); `DeleteTeamRow` chạy trong transaction, kiểm team thuộc company trước khi xoá thành viên; `RemoveTeamMember` dùng join `org_units.company_id`.
- **SQL (lớp 2):** `UpdateMembershipStatus(companyID, ...)` có `AND company_id = ?`; `DeleteMembership(companyID, ...)` kiểm membership thuộc company (`FOR UPDATE`) trước khi xoá bảng con, rồi xoá có `company_id`. Các lời gọi rollback nội bộ truyền `m.CompanyID`.
- **In-memory repo:** thêm state cho team, đếm thành viên theo company, `TeamBelongsToCompany`, helper test `SeedTeam`, `TeamMemberIDs`, `TeamExists`; `ListCompanyDepartments` lọc theo company khi biết chủ sở hữu.
- **Test mới:** `admin_service_membership_scope_test.go`, `admin_service_org_scope_test.go`, `admin_handler_membership_scope_test.go`, `infra/inmemory/admin_repository_company_scope_test.go`.
- **Test cũ cập nhật fixture:** `TestDeleteTeam_OK`, `TestCreateTeam_OK`, `TestCreateTeam_LimitReached`, `TestAddTeamMember_NotInDepartment` (seed department/team/membership thật; chữ ký `d1Repo`).
- **Docs:** `docs/api-contracts-json.md`.

## Kết quả
| Kiểm tra | Kết quả |
|---|---|
| Gate R | Trước khi sửa: 26 test "membership của company khác" FAIL (thao tác thành công hoặc đổi dữ liệu), 0 test "cùng company" FAIL; 4 nhóm team/department/title FAIL (`03-repro.md`) |
| Kiểm tra ngược | Gỡ guard tập trung → 3 nhóm test FAIL; gỡ guard ở `RemoveRole` → FAIL; gỡ kiểm tra team ở `AddTeamMember` → FAIL. Khôi phục → PASS |
| Test sau khi sửa | `TestMembershipScope_*`, `TestOrgScope_*`, `TestTenantRoutes_*` (23 route có membership id đều trả 404), test repo in-memory: PASS |
| `go build ./...` | PASS |
| `go test ./... -count=1` | Exit 1 ở cả fix và baseline. **Không có fail mới.** Fix 19 dòng FAIL, baseline 20 (baseline có thêm test flaky `TestSchemaParityConditionalPeriodicity`) |
| `go vet ./...` | Chỉ lỗi copylocks có sẵn ở `workflowfulfillment/required_document_gate_test.go:326-327` |
| `go test -race -count=1 ./internal/companyaccess/... ./internal/platformcms/...` | Không có DATA RACE; chỉ 2 fail có sẵn (`TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`, `TestCreateSelfServiceCompany_FeatureFlagOff`) |
| `docker compose -f docker-compose.dev.yml build api` | PASS |
| Quét hàm service dùng membership id | 24 hàm; 23 có guard; 1 ngoại lệ hợp lệ: `ListEmergencyAccessRequests` (bộ lọc danh sách, SQL đã lọc `company_id`, id lạ chỉ trả về rỗng) |

## Giới hạn
- Repo MySQL không có công cụ test SQL (không `sqlmock`). SQL mới (`DeleteTeamRow` có transaction và `FOR UPDATE`, `RemoveTeamMember` join, các hàm đếm join, `TeamBelongsToCompany`, `UpdateMembershipStatus`/`DeleteMembership`) được kiểm bằng đọc code và build; cần smoke trên DEV để xác nhận cú pháp và hành vi thật.
- Các thay đổi chưa deploy.

## Bổ sung sau review (T9)
Reviewer: be-security, admin-role, api-compat. Không có CRITICAL; 1 HIGH (BES-01, đã sửa) và các mục MEDIUM/LOW bên dưới.

| Finding | Xử lý |
|---|---|
| BES-01 (HIGH, xác nhận trong code): `PATCH` rỗng trên department, title, team của company khác trả về chính đối tượng đó (nhánh `len(sets)==0` đọc view không lọc company; cũng là oracle tồn tại) | **Đã sửa:** `requireDepartmentInCompany`, `requireTitleInCompany` (repo `TitleBelongsToCompany` mới), `requireTeamInCompany` ở `UpdateDepartment`, `UpdateTitle`, `UpdateTeam`. Test `TestOrgScope_PatchForeignObjects_NotFound`: trước fix 6 case FAIL, sau fix PASS; PATCH trong company của mình vẫn chạy. Vấn đề này đã được thấy ở review C4 (BES-04) và chưa xử lý, nay đã đóng |
| API-01 (LOW): `CreateTeam` che lỗi DB thành 404, từ chối department inactive, và tải cả danh sách department | **Đã sửa:** repo `DepartmentBelongsToCompany` (chỉ xét company, trả lỗi lưu trữ). Test `TestOrgScope_CreateTeam_StorageErrorIsNotNotFound` (trước fix FAIL) và `..._InactiveOwnDepartment_StillAllowed` |
| ROLE-01 (LOW, admin-role): role mang quyền platform chưa bị chặn ở `ReplaceMembershipPrimaryRole` và tạo/mời user | **Đã sửa:** `assertRoleHasNoPlatformPermissions` trong nhánh non-operator của `validateEnterpriseInviteRole`. Test `TestMembershipScope_PlatformCapableRole_PrimaryRoleAndCreateUser` (trước fix FAIL: nil) |
| BES-03 (LOW): role/department/title của company khác phân biệt được với id không tồn tại qua message | **Đã sửa:** thông báo thống nhất "role/department/title not found" ở `ensureRoleForMembership`, `ensureDepartmentForMembership`, `ensureTitleForMembership` (MySQL; không có test tự động) |
| BES-04 (LOW): `UpdateMembershipStatus` trả 404 giả khi trạng thái không đổi (MySQL báo 0 dòng) | **Đã sửa:** khi 0 dòng, kiểm tra membership có trong company mới trả 404; ngược lại trả view (idempotent). Hành vi này có sẵn từ trước C5 |
| BES-05 / API-02 (INFO/LOW): tài liệu và ma trận QA | **Đã sửa:** `docs/api-contracts-json.md` ghi đúng mã (team/department/title: 404 `INVALID_REQUEST`; company admin/transfer: 404 `MEMBERSHIP_NOT_FOUND`; 422 khi token thiếu company); thêm 12 dòng âm tính vào `docs/qa-test-matrix.csv` |
| Test bảng route chỉ so số lượng | **Đã sửa:** so sánh danh tính "METHOD pattern" với route đăng ký trong `admin_handler.go`; thử thêm route giả → test fail |
| BES-02 (MEDIUM, có sẵn từ trước): `DELETE /admin/memberships/{id}` lỗi khoá ngoại (`membership_direct_permissions`, `org_unit_memberships`, `departments.head_membership_id` không có `ON DELETE`) nên trả 500 với phần lớn membership | **Chưa sửa, follow-up:** cần quyết định sản phẩm (xoá cứng hay vô hiệu hoá, xử lý trưởng phòng). Không phải rủi ro an toàn: transaction rollback, không hỏng dữ liệu. Chính điều này giải thích vì sao DEV không có event `membership.delete` |
| ROLE-03 (INFO): tab cũ sau khi đổi company nay nhận 404 | Follow-up FE, giống API-01 của C4 |
| API-03 (INFO): Postman dùng một `membership_id` cố định | Không sửa; cần seed lại biến nếu thuộc company khác |

Verify lần cuối sau tất cả thay đổi: `go build` PASS; `go test ./...` không có fail mới so với HEAD `3203961` (fix 19 dòng FAIL, baseline 20, hơn kém nhau test flaky `TestSchemaParityConditionalPeriodicity`); `go vet` chỉ lỗi có sẵn; `-race` sạch (chỉ 2 fail có sẵn); Docker build PASS.
