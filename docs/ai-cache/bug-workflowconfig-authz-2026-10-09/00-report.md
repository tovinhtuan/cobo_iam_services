# Bug C2: Route `workflowconfig` chỉ kiểm tra token (2026-10-09)

Nguồn: `docs/ai-cache/risk-review-2026-10-09/10-risk-report.md` (C2). Workflow: `wf-bugfix`. Giai đoạn hiện tại: **lên solution, chưa sửa code**.

## Triệu chứng
- **Kỳ vọng:** chỉ platform CMS operator (có `platform.cms.view` + quyền template phù hợp) mới được đọc, publish hoặc activate global workflow version và tạo assignee role toàn cục.
- **Thực tế:** bất kỳ access token hợp lệ nào, kể cả nhân viên thường của bất kỳ tenant nào, cũng gọi được mọi route `workflowconfig`.

## Môi trường bị ảnh hưởng
- Server dev public (`docker-compose.artifacts.yml:125`: `WORKFLOW_VERSIONING_ENABLED: "true"`), local dev (`docker-compose.dev.yml:88`).
- `GET/POST /api/v1/platform/cms/workflow/assignee-roles` được đăng ký **bất cứ khi nào có MySQL pool**, không phụ thuộc flag (`internal/httpserver/server.go:578-581`). Vì vậy tắt flag **không** bịt được route POST assignee-roles.

## Routes
| Method | Path | Đăng ký khi |
|---|---|---|
| GET | `/api/v1/platform/cms/workflow/assignee-roles` | có pool |
| POST | `/api/v1/platform/cms/workflow/assignee-roles` | có pool |
| GET | `/api/v1/platform/cms/templates/{type_id}/workflow/configuration` | flag ON |
| GET | `.../workflow/readiness` | flag ON |
| POST | `.../workflow/validate` (chỉ đọc, không ghi) | flag ON |
| GET | `.../workflow/lifecycle` | flag ON |
| GET | `.../workflow/versions` | flag ON |
| GET | `.../workflow/versions/{version_no}` | flag ON |
| POST | `.../workflow/publish` | flag ON |
| POST | `.../workflow/versions/{version_no}/activate` | flag ON |

## Bên gọi phía FE (cobo_web_design @37a799db)
- **CMS** (sau `RequirePlatformAccess(['platform.cms.view'])`):
  - `src/features/cms-core/templates/workflow/workflowConfigApi.ts` gọi configuration/readiness/validate/lifecycle/versions/publish/activate.
  - `workflowCatalogApi.ts:31,47` gọi GET/POST assignee-roles.
- **Tenant portal:**
  - `DisclosureTypeDetail.tsx:79` dùng `createWorkflowCatalogApi` để **GET** assignee-roles, lấy label cho role.
  - `src/services/authApi.ts:72-74` đã xử lý fail-soft khi route này trả 403 (không redirect).
  - `tenantWorkflowRoleLabel.ts` có bảng label tĩnh làm fallback.

## Lịch sử
- Trong ai-cache chưa có ghi nhận nào về lỗ hổng authz của `workflowconfig`.
- Comment trong FE (`deadlineStepConfigViewModel.ts:103`: "auth-only") cho thấy việc chỉ kiểm tra token là có chủ ý cho GET catalog, nhưng các route ghi thì không có lý do.
