# 00: Brief, MR-4 (Q5-B: maker-checker cho override workflow theo quyền)

## Mục tiêu
Quyết định PO Q5-B (`po-decisions-template-workflow-2026-10-16/02-decisions.md`, GATE A ngày 16/10):
- **Người có quyền duyệt** (`template.workflow.override.approve`) kích hoạt được ngay, kể cả bản mình soạn. Audit ghi `self_approved`.
- **Người chỉ có quyền ghi** (`template.workflow.override.write`) chỉ lưu nháp và "Gửi duyệt".
- **Mọi đường kích hoạt** (PUT draft `publish:true`, POST approve, rebase-apply) dùng chung một quy tắc.
- **`merge_manual`** phải có giá trị hợp lệ cho từng field; `{}` trả 422.
- **Portal:** có "Gửi duyệt", danh sách bản chờ duyệt cho người duyệt, và nút "Kích hoạt" chỉ hiện khi có quyền duyệt.
- **Phạm vi:** chỉ trong công ty của token (Company Admin, `/api/v1/company/...`), không ảnh hưởng Admin Web.

## Người dùng
- **Người soạn:** thành viên có quyền ghi override.
- **Người duyệt:** thành viên có quyền duyệt. Trên DEV, mọi role có quyền ghi đều có luôn quyền duyệt (`admin_doanh_nghiep`, `truong_phong`…).

## Repo
- `cobo_iam_services`: `internal/disclosure` (app, infra mysql và in-memory, transport/http), migration.
- `cobo_web_design`: `src/pages/portal/DisclosureTypeDetail.tsx`, `workflow-customization/`, `WorkflowOverrideRebasePreviewModal.tsx`, `src/services/disclosureTypesApi.ts`, danh sách "Loại thông tin báo cáo".

## Hiện trạng (khảo sát 16/10)
- **PUT draft `publish:true`** (`internal/disclosure/app/service.go:1231-1311`):
  - cần quyền ghi, rồi quyền duyệt, và bỏ qua kiểm tra tự duyệt (`SkipSelfApprovalCheck`);
  - bản nháp được lưu **trước** khi kiểm quyền duyệt, nên người chỉ có quyền ghi gửi `publish:true` bị 403 mà bản nháp vẫn được lưu;
  - audit không ghi là tự duyệt.
- **POST approve** (`service.go:1313-1347`, `infra/mysql/repository.go:1700-1760`):
  - chặn người soạn tự duyệt (403 `SELF_APPROVAL_NOT_ALLOWED`);
  - guard "bản mới nhất" chỉ so với các bản `draft`, nên sau rebase-apply vẫn duyệt được một bản nháp cũ, đè lên nội dung vừa rebase (WF-08c);
  - Portal không gọi đường này.
- **rebase-apply** (`workflow_override_apply.go:78`): chỉ cần quyền ghi, ghi thẳng bản `approved`, không audit.
- **`merge_manual`** (`workflow_override_conflict.go:115-117`, `workflow_override_patch_apply.go:350-397`):
  - chỉ kiểm tên resolution;
  - `resolution_value` không được kiểm, kể cả việc resolution có nằm trong `resolution_options` của conflict hay không;
  - `{}` với `reminder_config` tắt nhắc của bước mà không báo; với `due_rule`/`assignee_role_ids` thì bị bỏ qua lặng lẽ;
  - FE luôn gửi `resolution_value: {}`.
- **Trạng thái version:** chỉ có `draft` và `approved`. Chưa có trạng thái chờ duyệt, danh sách chờ duyệt hay thông báo.
- **In-memory:** approve không có guard tự duyệt lẫn guard bản mới nhất, upsert không kiểm ETag.
- **FE:**
  - "Kích hoạt" chỉ gate theo quyền ghi; `canApproveWorkflowOverride` đã khai báo nhưng không dùng;
  - lỗi 403 ở mọi lệnh override chuyển sang `/app/forbidden`.
- **DEV:**
  - 16 version (8 `approved`, 8 `draft`), cả 8 lần duyệt đều là tự duyệt;
  - 7 override có bản nháp;
  - không có role nào có quyền ghi mà thiếu quyền duyệt.

## Định cỡ
**Normal**: xuyên hai repo, endpoint và trạng thái mới, migration bổ sung cột. Đi đủ các phase; dừng ở GATE A.
