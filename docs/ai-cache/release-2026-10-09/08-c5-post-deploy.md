# Post-deploy C5 (DEV, backend only)

## Candidate
- HEAD `3203961` + working tree diff (C5), không commit. Chỉ backend `cobo_iam_services`; FE và migration không đổi.
- Rollback point trên server: `bin/api.bak-20261009-c5`, `bin/worker.bak-20261009-c5` (bản C4).
- Deploy: `make deploy-be`. `/healthz`, `/readyz` (:3000) và `/healthz` (:8080) = 200; api/worker Up; 0 ERROR/panic trong log.

## Smoke API: 35/35 pass (`smoke-c5/smoke_dev_c5.out`)
Token tenant admin `c_001` (`admin.dn`), đối tượng đích thuộc company khác (`m_002` của `c_002`, team/phòng ban/chức danh của company khác).

| Nhóm | Kết quả |
|---|---|
| Probe đọc `GET /memberships/{m_002}/permissions` | 404 `MEMBERSHIP_NOT_FOUND` (chứng minh binary mới) |
| Ghi vào membership ngoài company: PATCH status, DELETE, POST role, PUT primary-role, POST/DELETE direct permission, PUT org-assignments | 404 `MEMBERSHIP_NOT_FOUND` |
| Team / phòng ban / chức danh ngoài company: PATCH, DELETE, tạo team trong phòng ban ngoài, thêm/xoá thành viên team | 404 (`INVALID_REQUEST` "not found"), không còn 409 |
| `POST /titles/{foreign}/members` với member của mình | 400 `INVALID_REQUEST` (xem Lưu ý 1) |
| Luồng trong company mình (đối tượng tạm `smoke-c5-<ts>`) | tạo phòng ban, tạo team, thêm thành viên phòng ban và team, sửa team, xoá thành viên team, thêm lại, xoá team, xoá thành viên phòng ban, sửa/xoá phòng ban, tạo/xoá chức danh: đều 200/201 |
| `PATCH` membership của mình với status không đổi (idempotent) | 200 |
| CMS operator `GET /platform/cms/admin/users?company_id=c_002` | 200 |

SQL mới đã chạy thật trên MySQL DEV, không lỗi cú pháp: `DepartmentBelongsToCompany`, `CountTeamsInDepartment`, `TeamBelongsToCompany`, `RemoveTeamMember` (DELETE ... JOIN), `DeleteTeamRow` (tx, `FOR UPDATE`), `CountDepartmentMembers`, `CountTitleMembers`, `UpdateMembershipStatus` (company_id). `DeleteMembership` mới chỉ được thử với membership ngoài company (404 trước khi vào tx); nhánh thành công không thử vì lỗi FK có sẵn BES-02.

## Smoke UI (chỉ đọc)
Tenant `/app/admin/users` và CMS `/cms/admin/users`: đăng nhập, chọn company, mở trang, API 200, `bad: []`, không có console error (`smoke-c5/results.json`, ảnh `tenant-users.png`, `cms-users.png`).

## Kiểm tra sau smoke (SQL chỉ đọc, `smoke-c5/c5_after*.out`)
- Dữ liệu company khác không đổi: `m_002` active với 1 role; team `ou_org_legal_tv_001` còn 2 thành viên; phòng ban đích còn 5 thành viên active; chức danh đích còn 1.
- Dữ liệu tạm còn lại trên DEV: 2 phòng ban và 2 chức danh `smoke-c5-*` ở trạng thái `inactive` (xoá mềm theo thiết kế). Team và liên kết thành viên đã sạch. Không có thao tác ghi nào ngoài `c_001`.
- Log api 15 phút sau smoke: 0 dòng lỗi/panic.

## Lưu ý
1. `POST /titles/{foreign}/members` trả 400 `INVALID_REQUEST` ("title not found") thay vì 404, vì lỗi đến từ `AddTitle` ở repo. Không ghi dữ liệu và không lộ tồn tại; chỉ lệch mã trạng thái so với các route khác. Follow-up nhỏ: gọi `requireTitleInCompany` trước trong `AddTitleMember`.
2. Hai phòng ban và hai chức danh `inactive` tên `smoke-c5-*` có thể dọn khi cần (cần duyệt ghi DB DEV).
3. Chưa kiểm: nhánh thành công của `DeleteMembership` (BES-02, lỗi FK có sẵn, task đã tách).

## Kết luận
GO. C5 đã chạy trên DEV, các đường từ chối cross-company hoạt động và luồng trong company không bị hồi quy.
