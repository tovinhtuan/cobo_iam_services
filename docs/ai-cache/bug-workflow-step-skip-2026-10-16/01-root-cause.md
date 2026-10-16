# 01: Root cause và tái hiện (GATE R)

## Root cause
- **WF-01 [Đã xác minh]:** `planLegacyCompleteIfNoPending` (`internal/workflow/app/service.go:508-534`) coi "không còn task pending nào khác" là điều kiện để kết thúc instance.
  - Điều kiện này chỉ đúng khi mỗi bước có một task.
  - Instance tạo từ template (không phải proposal) chỉ có task cho bước 1; từ PR-1, tiến độ bước nằm ở `workflow_instance_step_states`.
  - Vì vậy điều kiện luôn đúng ngay ở bước 1: instance `approved`, rồi `MarkRecordApproved` chạy.
  - `available_actions` tính bằng cùng logic nên Portal hiện nút "Phê duyệt" ở bước 1.
  - Thêm vào đó, khi submit thủ công, task đầu được gán cho chính người submit (`service.go:129-131`), nên người đó tự duyệt được.
- **WF-02 [Đã xác minh]:**
  - `transitionTask` và `ApplyTaskTransition` (`internal/workflow/infra/mysql/repository.go:285`) đổi trạng thái task mà không chạy cổng tài liệu bắt buộc (B3).
  - B3 chỉ nằm trong `UpsertStepCompleted` (`internal/deadlinealerts/infra/mysql/steps_repository.go:99-170`).
  - Với proposal v2/v3, mỗi task là một bước, nên duyệt task là hoàn tất bước mà không cần tài liệu.
- **Tầng:** service workflow và repository (MySQL, in-memory). FE chỉ vẽ theo `available_actions`.

## Tái hiện (GATE R): test fail trên code chưa sửa
| Test | Kết quả trước khi sửa |
|---|---|
| `internal/workflow/app` `TestTemplateWorkflow_CompletingStepOneTaskDoesNotSkipRemainingSteps` (approve/review/confirm × global_template/company_override, 3 bước) | `global_template/approve on step 1 with steps 2..3 open: err=<nil>, want 409` |
| `internal/workflow/app` `TestTemplateWorkflow_StepOneTaskOffersNoCompletionWhileStepsOpen` | `available_actions=[review approve confirm reject]` khi bước 2 còn dở |
| `internal/workflow/infra/mysql` `TestTaskTransitionGates_TemplateCompletionNeedsEveryStepCompleted` (MySQL 8.0, 148 migration của `run_dev_migrations.sh`) | Duyệt khi bước còn dở: `err=<nil>`, kỳ vọng 409 |
| `internal/workflow/infra/mysql` `TestTaskTransitionGates_ApproveRunsRequiredDocumentGate` | Duyệt khi thiếu tài liệu bắt buộc: `err=<nil>`, kỳ vọng 422 `WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING` |

Test MySQL chạy trên container `mysql:8.0` tạm, chỉ mở cổng trên localhost, DB `cobo_iam_mr1`.

## Hướng sửa (nhỏ nhất, tại root cause)
1. **WF-01:**
   - Với instance có snapshot nhưng không phải proposal, hành động hoàn tất task (approve/review/confirm) chỉ được kết thúc instance khi **mọi bước** của snapshot đã hoàn tất ở `workflow_instance_step_states`.
   - Repository kiểm trong cùng transaction (`FOR SHARE`, chặn mark-incomplete chen vào), nếu chưa đủ thì trả 409 `WORKFLOW_STEPS_INCOMPLETE`.
   - `available_actions` ẩn các hành động này khi còn bước dở.
   - Vẫn giữ một đường duyệt hợp lệ: hoàn tất đủ các bước ở màn hạn, rồi duyệt.
2. **WF-02:** hoàn tất task chạy B3 cho bước của task trong cùng transaction, theo đúng thứ tự khoá của `UpsertStepCompleted`: snapshot → step-state → kiểm.
3. **Không làm trong MR-1** (follow-up):
   - đồng bộ hai runtime (WF-06);
   - kiểm người phụ trách khi hoàn tất bước (WF-06);
   - reject/review/confirm kết thúc instance (WF-07).
