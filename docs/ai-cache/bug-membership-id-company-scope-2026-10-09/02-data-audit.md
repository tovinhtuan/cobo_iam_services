# C5: Audit dữ liệu DEV (read-only, user đã duyệt), 2026-10-09

Cách chạy: SSH tới DEV, `docker exec cobo-iam-mysql mysql ... cobo_iam` với `SET SESSION TRANSACTION READ ONLY`. Chỉ chạy `SELECT`. File SQL tạm đã xoá khỏi `/tmp` trên server. Không in secret.

## Kiểm tra tính toàn vẹn quan hệ (trạng thái hiện tại)
| # | Kiểm tra | Số dòng lệch |
|---|---|---|
| Q1 | `membership_direct_permissions.company_id` khác company của membership (tổng 196 dòng, 185 đang hiệu lực) | **0** |
| Q2 | `org_unit_memberships`: company khác company của membership hoặc của org_unit, hoặc tham chiếu mồ côi (tổng 11 dòng) | **0** |
| Q3 | `department_memberships`: company của department khác company của membership | **0** |
| Q4 | `membership_titles`: company của title khác company của membership | **0** |
| Q5 | `membership_roles`: role thuộc một company khác company của membership | **0** |
| Q6 | `departments.head_membership_id` thuộc company khác | **0** |

## Audit log
- Q8: không có event `admin.*` nào mà đối tượng (membership, org unit, title, department) thuộc company khác company của actor: **0 dòng**.
- Q7 (độ phủ audit): có log cho tạo/sửa/xoá department và title, thêm/gỡ quyền trực tiếp, gán role/phòng ban/chức danh, tạo/xoá team, tạo user/membership. **Không thấy event** cho `admin.membership.delete`, `admin.membership.role.remove`, `admin.membership.title.remove`, `admin.membership.department.remove`, thêm/gỡ thành viên team, `admin.title.member.*`. Nguyên nhân là các thao tác này chưa từng được dùng trên DEV (handler đã có `auditLog`).

## Kết luận
- Không có dấu vết dữ liệu bị tác động chéo company trên DEV.
- Giới hạn: kiểm tra toàn vẹn chỉ phát hiện thao tác **ghi tạo** dữ liệu lệch. Thao tác **xoá hoặc gỡ** (xoá membership, gỡ role/title/phòng ban, xoá thành viên team) không để lại dòng lệch, và nhóm này không có audit log để đối chiếu. Vì vậy không loại trừ được hoàn toàn. (Đính chính: handler của các thao tác này đã có `auditLog`; việc không có event trên DEV là do chưa ai dùng, không cần bổ sung audit.)
