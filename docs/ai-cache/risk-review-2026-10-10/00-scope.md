# Risk review 2026-10-10 - scope: all (lần 3)

- Mode: read-only (`/risk-review all`), không sửa code.
- Repos:
  - `cobo_iam_services` branch `improve/ui-theme` HEAD `ff5ada2` (merge-base với `main`: `bea9376`)
  - `cobo_web_design` branch `improve/ui-theme` HEAD `453a3494`
- Lần trước: `../risk-review-2026-10-09/` (lần 1, iam `?`), `../risk-review-2026-10-09-2/` (lần 2, iam `7a131a1`, web `26ed6c4e`).

## Thay đổi code kể từ lần 2
- iam `7a131a1..ff5ada2` (commit `10a5700`, `ff5ada2` "fix bug ROLE-01" + v2): 29 file trong `internal/companyaccess/**` (rbac restore plan, rollback approval, approval apply, versioning handler) + `internal/platform/errors/errors.go`. Danh sách: `git diff --name-status 7a131a1 ff5ada2`.
- web `26ed6c4e..453a3494`: `src/features/admin-core/{hooks/useConfigApprovals*, screens/approvals/ApprovalsPanel*, services/configApprovalsApi*}`.

## Reviewers (scope `all` → tất cả)
fe-security, be-security, admin-role, perf-reliability, cache-versioning, api-compat, secrets-cors.
Trọng tâm mỗi reviewer: (1) xác minh lại bản sửa ROLE-01 và vùng thay đổi mới; (2) cập nhật trạng thái các mục còn mở của lần 2; (3) tìm lỗi mới trên toàn repo.

## Secret scan (không ghi giá trị)
- `01-secret-scan-iam.txt`: 479 match (lần 2: 471). Match mới nằm ở `rbac_rollback_approval.go`, `rbac_rollback_scope_test.go`, `rbac_restore_integration_test.go`, `docs/ai-cache/bug-*-2026-10-09/*` (cần reviewer xác minh là fixture/placeholder).
- `01-secret-scan-web.txt`: 314 match (không đổi).
- File nhạy cảm đang được track: iam `.env.example`, `configs/login_password_rsa_dev.pem` (đã biết từ lần 1); web `.env.example`.
