# 00: Báo cáo lỗi MR-1 (WF-01, WF-02), 16/10/2026

- **Nguồn:** rà soát nghiệp vụ template/workflow ngày 16/10. MR-1 trong `docs/ai-cache/po-decisions-template-workflow-2026-10-16/02-decisions.md` (GATE A đã duyệt). Đây là lỗi, không cần PO quyết.
- **Workflow:** `wf-bugfix`. Chưa commit, chưa deploy.

## WF-01 (Critical): workflow nhiều bước hoàn tất ngay từ bước 1
- **Triệu chứng:** với instance tạo từ template (`workflow_source` = `global_template` hoặc `company_override`), duyệt task đầu tiên làm instance chuyển `approved` và hồ sơ thành `Approved`, kèm `published_date`. Bước 2..N bị bỏ qua.
- **Kỳ vọng:** chỉ được kết thúc instance khi mọi bước trong snapshot đã hoàn tất. Từ PR-1 (10/2026), tiến độ bước của instance template nằm ở `workflow_instance_step_states`.
- **Ai làm được:**
  - Khi submit thủ công, task đầu được gán cho chính người submit (`internal/disclosure/infra/workflow/bootstrap.go:50-61`, `internal/workflow/app/service.go:117-137`).
  - Người có quyền duyệt thấy nút "Phê duyệt" ở màn chi tiết hạn (`cobo_web_design/src/pages/portal/DeadlineDetail.tsx:395-459` vẽ theo `available_actions`).
- **DEV (`qa_ro`, 16/10):**
  - 0 instance template ở trạng thái `approved`, tức chưa xảy ra;
  - 344 instance nhiều bước đang có đúng 1 task.
  - Task đầu gán cho: 334 cho worker hệ thống (không ai bấm được), 2 cho chính người submit, 7 cho người khác.
- **Lịch sử:** `c892df4` (07/08) thêm nhánh v2 cho proposal. Nhánh legacy giữ "hoàn tất khi không còn task pending", và được khoá bằng test `TestLegacy_CompleteFirstTaskCompletesInstance` (`internal/workflow/app/multistep_runtime_test.go:343-378`).
- **Liên quan:** RMC-01 trong `bug-worker-alert-fixes-2026-10-10/01-root-cause.md:13` đã ghi nhận "workflow template chỉ có task bước 1; tiến độ bước nằm ở `workflow_instance_step_states`".

## WF-02 (High): duyệt qua API task bỏ qua cổng tài liệu bắt buộc (B3)
- **Triệu chứng:** `approve`/`review`/`confirm` qua `POST /api/v1/workflows/tasks/{id}/...` hoàn tất bước, hoặc cả instance, mà không kiểm tài liệu bắt buộc của bước.
- **Kỳ vọng:** giống `/complete` của màn hạn: thiếu tài liệu bắt buộc thì trả 422 `WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING`.
- **Bằng chứng:** `ValidateRequiredDocumentFulfillmentLocked` chỉ được gọi ở `UpsertStepCompleted` (`internal/deadlinealerts/infra/mysql/steps_repository.go:111,155`); `transitionTask` và `ApplyTaskTransition` không gọi.
- **Tác động lớn nhất:** proposal ad-hoc v3. Mỗi bước là một task, nên bấm duyệt là hoàn tất bước, kể cả bước có tài liệu bắt buộc.

## Môi trường
- MySQL (DEV và prod) và in-memory. Ở in-memory, runtime step-state không chạy: `UpsertStepCompleted` là no-op (`internal/deadlinealerts/infra/inmemory/repository.go:72`).
