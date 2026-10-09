# Bug C3: `adhoc-migrate-legacy-approvals` thiếu platform gate, chạy cross-tenant (2026-10-09)

Nguồn: `docs/ai-cache/risk-review-2026-10-09/10-risk-report.md` (C3). Workflow: `wf-bugfix`. Giai đoạn: **phân tích + solution, chưa sửa code**.

## Endpoint
`POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals`

- Đăng ký ở `internal/adhoc/transport/http/handler.go:48`, handler ở `:302-333`. Route được đăng ký mỗi khi module adhoc được wire (`WORKFLOW_ADHOC_ENABLED`, mặc định bật khi `ENV=development`, đúng với server dev).
- Được thêm ở commit `046ba96` ("add logic phase 2 adhoc"), là "one-time legacy migration (§6.7/A1)" cho migration `0098_adhoc_multi_reviewer`.
- Không có caller nào phía FE (`cobo_web_design/src`) hay Postman. Chỉ gọi tay.

## Luồng
1. Handler lấy `Subject` từ token, không kiểm tra platform.
2. `ListPendingLegacyApprovals` (`adhoc/app/service.go:1043-1049`):
   - Gọi `authorize(ctx, sub, "rbac.manage", ResourceRef{Type:"platform"})`.
   - Gọi `repo.ListPendingAdminApproval`, tức `SELECT proposal_id, company_id FROM ad_hoc_proposals WHERE status='pending_admin_approval'`, **không lọc company** (`adhoc/infra/mysql/repository.go:640-660`).
3. Với mỗi dòng, `FinalizeLegacyApproval` (`service.go:1007-1041`):
   - Chạy lại cùng `authorize`.
   - Gọi `AdminApprove` với `Subject{UserID: caller, MembershipID: cur.ProcessControllerID, CompanyID: <company của proposal>}`. Nghĩa là **mạo danh process controller của tenant khác** để vượt kiểm tra danh tính ở `service.go:618`.
4. Tác dụng phụ của `AdminApprove` trên tenant nạn nhân:
   - Proposal chuyển sang `approved` với ngày đề xuất làm ngày chốt.
   - Tạo và submit disclosure record, workflow instance, task đầu tiên.
   - Gửi notification/email.
   - Ghi audit với note "Auto-approved by migration 0098 (D9)".
5. Khi gặp lỗi, response trả `err.Error()` thô (lộ thông tin nội bộ, BES-08).

## Kỳ vọng và thực tế
- **Kỳ vọng:** chỉ platform operator (nội bộ Cobo), trong cửa sổ migration có kiểm soát.
- **Thực tế:** điều kiện duy nhất là action `rbac.manage` được authorizer cho qua **trong company của chính người gọi**. Authorizer bỏ qua `Resource` (`authorization/app/service.go:22-38`, mọi policy `ScopeType:"*"`). Phạm vi tác động lại là **mọi company**.
