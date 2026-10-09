# ROLE-01: Data audit DEV (read-only, 2026-10-09)

- Phiên `SET SESSION TRANSACTION READ ONLY` + `START TRANSACTION READ ONLY`. Không ghi dữ liệu.
- Chỉ xuất id/mã role, mã quyền và số đếm, không xuất dữ liệu cá nhân. Đã được user cho phép.
- Script và output: scratchpad `role01_audit.sql` / `role01_audit2.sql` (`.out`).

## Kết quả

| Q | Nội dung | Kết quả |
|---|---|---|
| Q1 | Loại role | 2 `system_global` (protected); 50 `tenant_default` (45 protected); 3 `tenant_custom` active, 8 inactive |
| Q2 | Role global | `self_reg_company_owner` 19 quyền, `dept_lead` 2 quyền; **0** quyền `cms`/`platform` |
| Q3 | Role đang giữ `platform.cms.view` | `cms_operator` c_001 (11 quyền cms/platform, 2 member), `admin_web` c_001 (8, 1 member), `cms_operator` c_002 (11, 1 member). Cả ba là `tenant_default` protected |
| Q4 | Phạm vi role global | `self_reg_company_owner`: 5 member ở **5 company**; `dept_lead`: 4 member ở 3 company |
| Q5/Q12 | Snapshot RBAC | c_001: v1–v4 (v3 do rollback); tenant tự đăng ký `144ca32b…`: v1–v3. Snapshot có chứa mục của role global (Q8 `in_snapshot` = 18) |
| Q7/Q11 | Lịch sử | **Đã có 1 rollback RBAC thật** ở c_001 lúc 2026-07-01 02:09:52. 1 approval `rbac.direct_permission.remove` đang `pending` ở `144ca32b…` |

## Nếu rollback về snapshot mới nhất ngay bây giờ (Q10)

| Company thực hiện | Role bị đổi | Quyền bị gỡ |
|---|---|---|
| c_001 (v4) | `cms_operator` (c_001) | 12 quyền: `platform.cms.view`, `cms.template.read/write/activate/archive/config.write`, `cms.record.read/write/publish/materialize`, `disclosure_type.config.write`, `deadline.comment` |
| c_001 (v4) | `admin_web` (c_001) | 10 quyền, gồm `platform.cms.view` và `cms.*` |
| c_001 (v4) | `self_reg_company_owner` (**global**) | `deadline.comment` |
| `144ca32b…` (v3, tenant tự đăng ký) | `self_reg_company_owner` (**global**) | `deadline.comment` |

## Kết luận
- **Tình huống A xác nhận bằng dữ liệu:** một rollback ở c_001 sẽ làm cả 3 membership operator CMS (`cms_operator` ×2, `admin_web` ×1) mất `platform.cms.view`, tức mất quyền vào `/cms`.
- **Tình huống B xác nhận bằng dữ liệu:** một **tenant tự đăng ký** (`144ca32b…`) rollback cũng đủ gỡ `deadline.comment` khỏi role chủ sở hữu dùng chung của **5 company**. Quyền này được thêm sau thời điểm snapshot.
- **Rollback 2026-07-01 ở c_001** rất có thể đã gỡ quyền CMS của các role trên. Hiện các quyền đó đang có lại, có thể do migration sau đó cấp lại. Không truy được lịch sử `role_permissions`, nên ghi là *có thể*.
- Role global hiện **không** giữ quyền `cms`/`platform`, nên nhánh "gỡ quyền platform khỏi role global" hiện không gây hại. Nhánh "đưa role global về snapshot cũ" thì có (B).
- **Không cần sửa dữ liệu ngay:** trạng thái hiện tại đúng. Rủi ro nằm ở lần rollback hoặc apply approval RBAC tiếp theo. Approval `pending` ở `144ca32b…` nếu được duyệt cũng sẽ chạy cùng đường reconcile toàn ma trận.
