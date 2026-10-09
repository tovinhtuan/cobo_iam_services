# C3: Phân tích rủi ro chi tiết, root cause, solution

## 1. Hiệu chỉnh finding trong risk report (quan trọng)
Risk report ghi "gate = permission `rbac.manage`, mà mọi chủ công ty tự đăng ký đều có". Trace kỹ hơn cho thấy điều này **không chính xác**:

- `"rbac.manage"` được truyền vào authorizer như một **action code**, không phải permission code.
- `GetActionPolicy` (`authorization/infra/mysql/repository.go:153-183`) tra bảng `action_policy_matrix`:
  - Repo này **không có migration nào tạo bảng đó** (chỉ `0068` sửa bảng nếu đã tồn tại).
  - Khi thiếu bảng hoặc thiếu dòng, code fallback về `legacyPolicy(action)`.
- `legacyPolicy` (`:205-300`) không có `case "rbac.manage"`, nên rơi vào giá trị mặc định `required := "system.settings"`.
- ⇒ **Permission thực sự cần là `system.settings`**, không phải `rbac.manage`.
  - Ngoại lệ: môi trường có bảng `action_policy_matrix` với dòng `action_code='rbac.manage'`. Khi đó cần đúng permission trong dòng đó.
  - Repo không có bằng chứng về bảng này; cần kiểm tra DB thật.

Ai có `system.settings`?
- Self-register (`iam/registrationmysql/register_public.go`) **không** cấp. Vậy chủ công ty tự đăng ký **không** khai thác được, khác với kết luận cũ.
- `system.settings` thuộc `GrantTierTenantAdminOnly` (`companyaccess/app/rbac_grant_policy.go:58`), nên `ValidateAssignPermissionGrant` trả `errPermissionNotGrantable` (`:171-184`). Tenant admin **không tự cấp được** qua API role-permission.
- Migration `0034_seed_org_structure_demo` (có trong `run_dev_migrations.sh:42`, đã chạy trên server dev) cấp `system.settings` cho role **tenant** `admin_doanh_nghiep` (`role_org_admin_001`, company demo `cmp_org_demo_001`).
  - Grant này dùng `SELECT ... WHERE permission_code IN (...)`, nên chỉ có hiệu lực **nếu dòng permission `system.settings` tồn tại** trong bảng `permissions`.
  - Không migration nào trong runner tạo dòng đó (chỉ `seed_demo_package_1.sql`, file không có trong runner). Có thể đã được tạo tay trên server.
- Platform operator thật (role `cms_operator`, 0009/0063) cũng **không** có `system.settings` theo seed.
  - ⇒ Trên DB sạch, rất có thể **không ai** gọi được endpoint, kể cả admin nội bộ. Endpoint vừa không an toàn về thiết kế, vừa có thể không dùng được.

## 2. Đánh giá rủi ro (đã hiệu chỉnh)

| Khía cạnh | Đánh giá |
|---|---|
| Lỗi thiết kế (confirmed) | Thao tác **cross-tenant** chỉ được bảo vệ bằng một check **trong phạm vi tenant**: authorizer bỏ qua `Resource{Type:"platform"}`, không kiểm `platform.cms.view`. Ai có `system.settings` ở **bất kỳ** company nào cũng chạy được trên mọi company. |
| Khả năng khai thác hiện tại | **Có điều kiện** (plausible). Cần một tài khoản tenant có `system.settings` hiệu lực: (a) demo tenant admin `admin_doanh_nghiep` trên server dev nếu dòng permission tồn tại (password seed đã lộ, xem C1); (b) role nào được cấp tay; (c) bảng `action_policy_matrix` bị cấu hình sai. Chủ công ty tự đăng ký: **không**. |
| Tác động nếu khai thác | Cao. Hàng loạt proposal `pending_admin_approval` của mọi tenant bị duyệt bằng danh tính process controller giả mạo. Hệ quả: tạo disclosure record và workflow thật, gửi email cho người dùng thật, ngày chốt lấy từ ngày đề xuất mà không ai duyệt. Không có undo tự động. |
| Số proposal bị ảnh hưởng | `pending_admin_approval` là trạng thái legacy, proposal mới không bao giờ vào được (`service.go:742`). Số dòng là hữu hạn và có thể đã bằng 0. **Cần đếm trên DB.** |
| Rủi ro phụ | Response lộ `err.Error()`. Endpoint chạy đồng bộ, không giới hạn số dòng, dùng request ctx với `WriteTimeout` 15s, nên có thể dừng giữa chừng (idempotency key `migration-0098:<id>` giảm rủi ro chạy lại). Không có dry-run. Không có flag tắt riêng. Chỉ có test với auth fake luôn cho qua (`service_test.go:747`). |
| **Mức độ hiệu chỉnh** | **HIGH** (thay vì CRITICAL). Thành CRITICAL nếu DB server có `system.settings` gán cho role tenant, hoặc `action_policy_matrix` map `rbac.manage` sang permission tenant. |

## 3. Root cause (confirmed)
Endpoint vận hành platform/cross-tenant được gate bằng `authorize(action="rbac.manage")`. Lời gọi này (1) dùng tên permission làm action code nên quyền thật phụ thuộc fallback `legacyPolicy` (= `system.settings`), (2) chỉ đánh giá quyền trong company của người gọi, và (3) không yêu cầu `platform.cms.view`. Sau đó service cố ý mạo danh process controller của từng tenant.

- Layer: API transport + service (authz). Repository cố ý không scope tenant.

## 4. Solution

> **Superseded (2026-10-09):** phương án A (CLI) và B dưới đây **không được triển khai**. Đã chốt và implement **A′**, xem mục 9: gỡ hẳn endpoint, không có CLI duyệt hộ cross-tenant. Không tạo `cmd/adhoc-migrate-legacy-approvals`.

### Khuyến nghị: gỡ endpoint HTTP, thay bằng lệnh CLI chạy có kiểm soát (phương án A)
Lý do:
- Đây là migration một lần; không có caller FE; trạng thái legacy không phát sinh mới.
- CLI chạy với quyền truy cập DB/server, nằm ngoài bề mặt tấn công HTTP.
- Đã có tiền lệ: `cmd/legal-basis-backfill`, `cmd/template-workflow-migrate`, `cmd/periodic-materialize-one`.

Bước 0 (cần user duyệt, read-only):
```sql
SELECT company_id, COUNT(*) FROM ad_hoc_proposals WHERE status='pending_admin_approval' GROUP BY company_id;
-- Kiểm chứng gate thực tế:
SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='action_policy_matrix';
SELECT r.company_id, r.role_code FROM role_permissions rp JOIN roles r ON r.role_id=rp.role_id
  JOIN permissions p ON p.permission_id=rp.permission_id
  WHERE p.permission_code='system.settings' AND rp.status='active';
-- Audit dấu vết đã bị lạm dụng chưa:
SELECT proposal_id, company_id, updated_at FROM ad_hoc_proposals WHERE adjustment_note='Auto-approved by migration 0098 (D9)';
```
(Cột `adjustment_note`, đã đối chiếu với migration.)

Thay đổi code:
1. **Gỡ route** ở `adhoc/transport/http/handler.go:46-48` và handler `migrateLegacyApprovals` (`:296-333`).
2. **Tạo `cmd/adhoc-migrate-legacy-approvals/main.go`**:
   - Dùng lại `FinalizeLegacyApproval` / `ListPendingLegacyApprovals`, bỏ `authorize` trong 2 hàm này (chuyển thành hàm nội bộ cho CLI, ví dụ `MigrateLegacyApprovals(ctx, actorUserID, opts)`). Hoặc giữ service và truyền một authorizer "system" chỉ dùng trong CLI.
   - Cờ: `--dry-run` (mặc định true, chỉ liệt kê), `--company-id` (tùy chọn), `--limit`, `--actor-user-id` (bắt buộc, ghi audit).
   - Dừng ở lỗi đầu tiên, giữ semantics §12.3.
   - Log từng proposal; không in secret.
3. **Interface `adhocapp.Service`**: bỏ 2 method khỏi interface HTTP (`contracts.go:39-50`) và cập nhật `handler_test.go:67-71` fake.
4. **Tests**:
   - Gate R trước khi sửa: thêm test HTTP trong `adhoc/transport/http`. Dùng service thật với auth fake chỉ cho qua khi permission = `system.settings` của company A (mô phỏng `legacyPolicy`), seed 1 proposal `pending_admin_approval` ở company B. POST endpoint với token company A → hiện tại **200 và `processed:1`** (proposal của B bị duyệt). Đây là repro.
   - Sau fix: route trả 404/405 (không đăng ký). Test service/CLI: dry-run không ghi gì; chạy thật duyệt đúng proposal, idempotent khi chạy lần 2; `--company-id` chỉ chạm company đó.

### Phương án B (chỉ khi team cần giữ HTTP tạm thời)
- Gate tường minh qua `GetEffectiveAccess`: yêu cầu **`platform.cms.view` + `system.settings`** (hoặc `rbac.manage`) trên chính membership platform, theo mẫu `workflowdoctemplate.requireCMSEditor`. Không dùng action-code mapping.
- Thêm flag `ADHOC_LEGACY_MIGRATION_ENABLED` (mặc định false), chỉ đăng ký route khi bật.
- Body `{dry_run, company_id, limit}`; response không trả `err.Error()`, chỉ trả mã lỗi + `proposal_id`.
- Ghi audit `ADHOC_LEGACY_MIGRATION_RUN` với actor thật.
- Gỡ hẳn sau khi đếm còn 0 dòng trên mọi môi trường.

## 5. Blast radius
- **Cùng pattern "permission name dùng làm action code":** `companyaccess/app/admin_service.go:983,1008,1026` gọi `authorize(..., "rbac.manage", "")`. Theo fallback, các lời gọi này thực ra đòi `system.settings`. Đó là lỗi chức năng: owner có `rbac.manage` nhưng thiếu `system.settings` sẽ bị 403. Cần ticket riêng: thêm `case "rbac.manage": required = "rbac.manage"` hoặc đổi sang action code chuẩn, sau khi xác định rõ các endpoint này.
- **Authorizer bỏ qua `Resource`:** đây là root chung của C4/C5/H2. Ngoài scope ticket này.
- **Cross-repo:** không có FE caller, contract FE không đổi.
- **Dữ liệu:** nếu query audit ở Bước 0 trả dòng có note "Auto-approved by migration 0098 (D9)" do actor **không phải** platform operator, cần lập kế hoạch xử lý từng tenant (liên hệ khách hàng, huỷ hoặc archive record sinh ra). Không tự chạy.

## 6. Mitigation tạm thời (trước khi deploy fix, cần user duyệt)
- Chặn `POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals` ở nginx. Rule này chỉ hiệu quả khi cổng 8080 không còn mở thẳng (H23).
- Hoặc tắt module adhoc (`WORKFLOW_ADHOC_ENABLED=false`) nếu chấp nhận mất tính năng adhoc tạm thời.
- Thu hồi `system.settings` khỏi các role tenant demo trên server dev (`role_org_admin_001`).

## 7. Verify dự kiến (khi implement)
- `go test ./internal/adhoc/...` (lưu ý PERF-10: test race adhoc flaky có sẵn).
- `go test ./...` so với baseline; `go vet ./...`; `go build ./...` (đảm bảo `cmd/` mới build được).
- `docker compose -f docker-compose.dev.yml build api`.
- Review: `be-security-reviewer` + `admin-role-reviewer` + `perf-reliability-reviewer` (vì có thao tác ghi hàng loạt).

## 8. Quyết định cần user
1. Chọn A (gỡ HTTP → CLI, khuyến nghị) hay B (giữ HTTP có gate + flag)?
2. Cho phép chạy các query read-only ở Bước 0 trên DB server dev không?
3. Đồng ý hạ mức độ C3 từ CRITICAL xuống HIGH (điều kiện) trong `10-risk-report.md` sau khi có kết quả query?

## 9. Quyết định (2026-10-09)

Yêu cầu của user: **không được tác động sang tenant khác nếu không có permission tương ứng trong chính tenant đó.**

Chọn **A′: gỡ hẳn endpoint, không dùng CLI duyệt hộ cross-tenant làm đường chính.**

### Lý do
- Phương án B (giữ HTTP, gate `platform.cms.view`) bị loại vì vẫn vi phạm yêu cầu. Platform operator không phải member của tenant B, nhưng vẫn duyệt proposal của B bằng cách mạo danh process controller.
- Phương án A với CLI chạy toàn bộ cũng bị loại làm đường mặc định, vì lý do tương tự: CLI vẫn duyệt hộ tenant bằng quyền vận hành.

### Cách xử lý proposal legacy `pending_admin_approval`, hoàn toàn trong phạm vi tenant
1. **Duyệt:** process controller của chính tenant gọi `POST /api/v1/company/ad-hoc-proposals/{id}/admin-approve`.
   - Company lấy từ token. Có kiểm tra danh tính `ProcessControllerID == Subject.MembershipID` (`service.go:618`).
   - FE vẫn có client `adHocAlertsApi.adminApprove`.
2. **Không duyệt:** người tạo `cancel`, hoặc người có quyền trong tenant `reject` (`service.go:797` chấp nhận trạng thái này).
3. **Process controller không còn hoạt động:** tenant admin của chính tenant gán lại controller (quyền `ad_hoc_alert.process_control`), rồi làm bước 1. Lúc implement cần xác minh API gán lại có tồn tại.
4. Thông báo cho các tenant còn dòng `pending_admin_approval`, dựa trên query đếm ở Bước 0 (read-only, chỉ đọc số lượng).

### Thay đổi code
- Gỡ route `adhoc-migrate-legacy-approvals` và handler.
- Gỡ `FinalizeLegacyApproval` / `ListPendingLegacyApprovals` khỏi interface và service.
- Gỡ `ListPendingAdminApproval` (query không scope tenant) khỏi repo.
- Cập nhật fake trong test.
- Gate R: test HTTP chứng minh tenant A làm thay đổi proposal của tenant B. Sau fix: route không tồn tại (404).
- Thêm test khoá hành vi: `admin-approve` với token tenant A trên proposal của tenant B → 404/403.

### Ngoại lệ (chỉ khi team quyết định riêng, ngoài scope)
Nếu còn dòng mồ côi không tenant nào xử lý được, giải pháp là một lệnh vận hành có biên bản. Mặc định chạy dry-run, chạy theo từng `--company-id`, và phải có xác nhận của tenant. Lệnh này không được expose qua HTTP.
