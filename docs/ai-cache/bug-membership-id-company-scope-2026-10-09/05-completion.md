# C5: Completion report (2026-10-09)

```text
Bug: Các route tenant /api/v1/admin/* nhận membership_id (hoặc id team/department/title) không kiểm
     tra đối tượng thuộc company của token. Authorizer bỏ qua resource; authorizeScopedMembershipMutation
     trả nil cho company scope; SQL theo membership_id không có company_id.
Reproduction: TestMembershipScope_* (26 case FAIL trước fix, 03-repro.md), TestOrgScope_*,
     TestTenantRoutes_ForeignMembership_NotFound (23 route), test repo in-memory.
Root cause: thiếu kiểm tra thuộc company ở service và thiếu company_id trong SQL.
Fix (backend, internal/companyaccess):
  - requireTargetMembership (guard tập trung trong authorizeScopedMembershipMutation + guard tường
    minh ở 12 hàm, config approval, AssignCompanyAdmin, TransferOwnership); đối tượng ngoài company
    trả 404 MEMBERSHIP_NOT_FOUND (không lộ tồn tại)
  - Team/department/title: requireTeamInCompany/DepartmentInCompany/TitleInCompany; hàm đếm lọc
    company; DeleteTeamRow trong transaction kiểm company trước; RemoveTeamMember join company_id;
    CreateTeam validate department; PATCH rỗng không còn trả đối tượng của company khác
  - SQL: UpdateMembershipStatus/DeleteMembership có company_id (đổi chữ ký repo); trạng thái không
    đổi không còn 404 giả
  - Role mang quyền platform bị chặn ở primary-role và tạo/mời user (như AssignRole)
  - Thông báo "not found" thống nhất cho role/department/title ngoài company
  - Docs: api-contracts-json.md, qa-test-matrix.csv
Verification: 04-verify.md. Không có test fail mới so với HEAD 3203961; -race sạch; Docker build PASS;
     kiểm tra ngược (gỡ guard thì test fail). SQL MySQL mới chưa chạy trên MySQL thật (không có sqlmock):
     cần smoke DEV.
Review: be-security, admin-role, api-compat. 1 HIGH (PATCH rỗng, đã sửa), còn lại LOW/INFO đã sửa
     hoặc ghi follow-up.
Blast radius / data repair: không migration; FE không đổi (id lấy từ danh sách của company đang chọn).
     Audit DEV (02-data-audit.md) sạch.
Follow-ups:
  1. [BES-02] DELETE /admin/memberships/{id} lỗi khoá ngoại trên MySQL (có sẵn từ trước): cần quyết định
     xoá cứng hay vô hiệu hoá.
  2. [FE] Tab cũ sau khi đổi company nhận 404 MEMBERSHIP_NOT_FOUND/COMPANY_SCOPE_MISMATCH; làm mới
     ngữ cảnh khi đổi company.
  3. [Authorizer] So company của resource ở tầng authorizer (gốc chung của C4 và C5).
  4. [ROLE-02 của C4] Hợp nhất hai định nghĩa "platform operator".
  5. [BES-02 của C4] CreateMembership gắn user bất kỳ vào company của mình.
  6. [Test] Repo MySQL không có công cụ test SQL; cân nhắc sqlmock hoặc test tích hợp.
  7. [Ops] 9 user không có membership trên DEV (cần duyệt riêng).
  8. [Deploy] Chưa deploy; smoke DEV cần kiểm SQL mới (DeleteTeamRow, RemoveTeamMember, TeamBelongsToCompany,
     các hàm đếm, UpdateMembershipStatus/DeleteMembership).
```
