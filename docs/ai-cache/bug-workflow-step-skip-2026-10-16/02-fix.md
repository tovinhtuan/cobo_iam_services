# 02: Sửa lỗi và kiểm chứng (chưa commit, chưa deploy)

Đã gồm các bản sửa sau review (`03-review.md`).

## Thay đổi
| File | Nội dung |
|---|---|
| `internal/platform/errors/errors.go` | Mã lỗi mới `WORKFLOW_STEPS_INCOMPLETE` (409). `gofmt` căn lại thêm 1 dòng comment có sẵn (`CodeEndpointRemoved`) |
| `internal/workflow/app/contracts.go` | `Repository.ListCompletedStepCodes`. `TaskTransitionApply` thêm `WorkflowInstanceID`, `StepCode`, `GateRequiredDocuments`, `RequireCompletedSteps`, `CompleteStepBy`, `StepCompletedAt` |
| `internal/workflow/app/service.go` | `transitionTask` bật cổng tài liệu cho approve/review/confirm (không cho reject). `planLegacyCompleteIfNoPending` gắn `RequireCompletedSteps` khi định kết thúc instance có snapshot nhưng không phải proposal. `planV2AdvanceOrComplete` gắn `CompleteStepBy`/`StepCompletedAt` (review DB-01). Helper `requiredCompletedSteps`, `MissingSteps`, `StepsIncompleteError` |
| `internal/workflow/app/task_action_authority.go` | `available_actions` ẩn approve/review/confirm khi instance template còn bước dở (`stepsOpen`; mỗi instance chỉ đọc một lần khi liệt kê task) |
| `internal/workflow/infra/mysql/repository.go` | Trước transaction: dò bước có tài liệu bắt buộc không (đọc ngoài transaction, review V2-DB-04). Trong transaction, trước khi đổi task: nếu có thì khoá snapshot tài liệu → khoá step-state → kiểm B3. Đọc các bước đã xong (đọc thường). Sau khi đổi task: với proposal, upsert `completed_at` của bước, giữ lần hoàn tất trước nếu đã có. `ListCompletedStepCodes`, `stepHasRequiredDocument` |
| `internal/workflow/infra/inmemory/repository.go` | Lưu bước đã xong (`MarkStepCompleted`, và `CompleteStepBy` khi chuyển task), kiểm như MySQL. Không chạy B3 (in-memory không có file fulfillment, giống `UpsertStepCompleted` in-memory) |
| Test | Mới: `template_step_completion_test.go` (app), `task_transition_gates_integration_test.go` (MySQL, 5 test, có 1 test xen kẽ hai transaction), `repository_steps_test.go` (in-memory). Sửa: fake trong `service_test.go`; `TestLegacy_CompleteFirstTaskCompletesInstance` thành `...OnceStepsAreDone` (test cũ khoá đúng hành vi lỗi) |

## Hành vi sau khi sửa
- **Instance template (`global_template`, `company_override`, và mọi nguồn có snapshot nhưng không phải proposal):**
  - Task bước 1 chỉ kết thúc được instance, và chỉ duyệt được hồ sơ, khi **mọi bước** đã hoàn tất ở màn hạn.
  - Nếu còn bước dở: 409 `WORKFLOW_STEPS_INCOMPLETE`, kèm `details.missing_step_codes`. Portal không hiện nút vì `available_actions` đã ẩn.
- **Proposal v2/v3:**
  - Vẫn đi từng bước bằng task. Mỗi lần hoàn tất task chạy B3 cho bước của task; thiếu tài liệu bắt buộc thì trả 422 `WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING`.
  - Hoàn tất task cũng ghi `completed_at` cho bước ở step runtime, trong cùng transaction. Bước sau trở thành bước hiện tại nên tải được tài liệu. Tài liệu của bước đã xong bị khoá (upload/thay/xoá trả `WORKFLOW_STEP_NOT_CURRENT`), giống bước hoàn tất ở màn hạn.
- **Reject:** không đổi (xem follow-up WF-07).
- **Thứ tự lỗi khi duyệt task:** 422 thiếu tài liệu → 409 còn bước dở → 409 task không còn pending.

## Kiểm chứng
| Lệnh / test | Kết quả |
|---|---|
| GATE R (trước khi sửa) | 2 test app + 2 test MySQL fail đúng lỗi (`01-root-cause.md`) |
| `go test ./internal/workflow/...` | ok |
| MySQL 8.0 thật (148 migration, DB `cobo_iam_mr1`): `TaskTransitionGates` | 5/5 pass; test xen kẽ chạy lặp 5 lần đều pass |
| GATE V vòng 1: tắt các dòng hành vi WF-01/WF-02 | 3 test app + 2 test MySQL fail lại; khôi phục thì pass |
| GATE V vòng 2 (bản sửa review) | V1 bỏ `CompleteStepBy`: `TestProposalTask_CompletesItsStepInTheStepRuntime` + `TestTaskTransitionGates_ProposalTaskCompletesItsStep` fail. V2 upsert ghi đè: `...KeepsAnEarlierStepCompletion` fail. V3 probe luôn false: `...ApproveRunsRequiredDocumentGate` + `...ProposalTaskCompletesItsStep` fail. Khôi phục (file giống hệt bản lưu): pass |
| GATE V vòng 3 (V2-DB-04, V2-DB-05) | V4 thêm câu đọc thường trong transaction trước khoá: `...GateSeesADeleteCommittedWhileItWaits` fail 3/3 (approve thành công dù tài liệu đã bị xoá). V5 đảo thứ tự hai phép gán ODKU: `...ProposalTaskCompletesItsStep` fail (`completed_by` NULL). Khôi phục: pass |
| `go test ./...` so với HEAD (`git archive`) | 0 fail mới (HEAD 21, sau sửa 20, danh sách giống hệt các lần trước; `TestSchemaParityConditionalPeriodicity` chỉ fail ở bản archive) |
| `go vet ./...` | sạch (cảnh báo có sẵn ở `internal/workflowfulfillment/required_document_gate_test.go`) |
| `gofmt -l` trên file đã đổi/mới | sạch |
| `docker compose -f docker-compose.dev.yml build api` | Built |

## Blast radius và dữ liệu
- `MarkRecordApproved` chỉ được gọi từ side effect khi workflow hoàn tất (`internal/workflow/app/service.go`), nên không còn đường duyệt nào khác mắc cùng lỗi.
- Các chỗ đọc `workflow_instance_step_states.completed_at`:
  - B3 ở màn hạn (`deadlinealerts`);
  - khoá tài liệu (`workflowfulfillment`);
  - khoá bằng chứng bước (`workflowstepevidence`).

  Với proposal, các chỗ này giờ thấy bước đã xong ngay khi task được duyệt, đúng ý nghĩa nghiệp vụ.
- Tạo instance bắt buộc có snapshot (`ValidateSnapshot`, "Model A invariant"). Instance không snapshot chỉ có thể là dữ liệu cũ (BES-01).
- **DEV** (`qa_ro`, 16/10). Không cần sửa dữ liệu:
  - 0 instance không phải proposal đã `approved` mà còn bước dở;
  - 0 task đã hoàn tất mà bước còn thiếu tài liệu bắt buộc;
  - 0 instance không có snapshot;
  - 3 instance proposal `in_progress`, chưa task nào được duyệt, nên không có bước nào cần backfill `completed_at`.
- **Prod:** chạy 4 truy vấn chỉ đọc dưới đây trước khi deploy. Nếu có kết quả, lập kế hoạch sửa dữ liệu riêng; không tự chạy.

```sql
-- WF-01: hồ sơ đã duyệt khi còn bước dở
SELECT wi.workflow_instance_id, wi.record_id FROM workflow_instances wi
WHERE wi.status = 'approved' AND wi.workflow_source NOT IN ('proposal_snapshot_v2','proposal_snapshot_v3')
  AND JSON_LENGTH(wi.snapshot_json) > 0
  AND (SELECT COUNT(*) FROM workflow_instance_step_states s
       WHERE s.workflow_instance_id = wi.workflow_instance_id AND s.completed_at IS NOT NULL) < JSON_LENGTH(wi.snapshot_json);
-- WF-02: task đã hoàn tất mà bước còn thiếu tài liệu bắt buộc
SELECT t.task_id, t.workflow_instance_id, t.step_code FROM workflow_tasks t
JOIN workflow_step_document_requirement_snapshots r
  ON r.workflow_instance_id = t.workflow_instance_id AND r.step_code = t.step_code AND r.required = 1
WHERE t.status IN ('approved','reviewed','confirmed')
  AND NOT EXISTS (SELECT 1 FROM workflow_step_document_fulfillment_files f
                  WHERE f.requirement_snapshot_id = r.id AND f.lifecycle_status = 'active');
-- DB-01: bước proposal đã duyệt bằng task nhưng chưa có completed_at (cần backfill để bước sau thành bước hiện tại)
SELECT t.workflow_instance_id, t.step_code, t.task_id FROM workflow_tasks t
JOIN workflow_instances wi ON wi.workflow_instance_id = t.workflow_instance_id AND wi.company_id = t.company_id
LEFT JOIN workflow_instance_step_states s ON s.workflow_instance_id = t.workflow_instance_id AND s.step_code = t.step_code
WHERE wi.workflow_source IN ('proposal_snapshot_v2','proposal_snapshot_v3')
  AND t.status IN ('approved','reviewed','confirmed') AND s.completed_at IS NULL;
-- BES-01: instance đang chạy không có snapshot (cổng WF-01 không áp dụng được)
SELECT wi.workflow_instance_id FROM workflow_instances wi
WHERE (wi.snapshot_json IS NULL OR JSON_LENGTH(wi.snapshot_json) = 0) AND wi.status = 'in_progress'
  AND COALESCE(wi.workflow_source, '') NOT IN ('proposal_snapshot_v2','proposal_snapshot_v3');
```

Backfill cho truy vấn DB-01 (nếu có dòng):
- upsert `completed_at` cho từng bước như câu lệnh trong `ApplyTaskTransition` (giữ `completed_at` đã có);
- `completed_by_membership_id` để `NULL`, vì không biết ai duyệt.

## Tương thích và rollout
- Chỉ BE. Không có migration, không có cờ tính năng.
- Thay đổi contract đều là bổ sung:
  - mã 409 mới;
  - approve/review/confirm có thể trả 422 thiếu tài liệu (mã lỗi đã có);
  - `available_actions` ít đi khi còn bước dở;
  - với proposal, tài liệu của bước đã duyệt bị khoá (mã lỗi đã có).
- FE hiện tại hiển thị `message` cho mã lỗi chưa map. Follow-up FE: map `WORKFLOW_STEPS_INCOMPLETE` sang câu tiếng Việt.
- Rollback: revert commit BE.
  - `completed_at` đã ghi cho bước proposal vẫn ở lại; bản cũ chỉ đọc cột này, nên không gây lỗi.
  - Sau rollback, tài liệu của các bước đó vẫn bị khoá (đúng nghiệp vụ).

## Rủi ro còn lại và follow-up
- **WF-06:**
  - hoàn tất bước ở màn hạn chưa kiểm người phụ trách bước, nên người nộp vẫn có thể tự hoàn tất mọi bước rồi duyệt (rủi ro có từ trước, đã hẹp hơn nhiều);
  - hai runtime vẫn chưa đồng bộ hoàn toàn: hoàn tất bước proposal ở màn hạn không hoàn tất task.
- **WF-07:** reject ở bước 1 vẫn kết thúc instance; review/confirm kết thúc instance mà không duyệt hồ sơ.
- **In-memory:**
  - không chạy B3;
  - chưa có cách hoàn tất bước template từ màn hạn, nên workflow template ở in-memory không duyệt được nữa (trước đây duyệt được do chính lỗi này).
- Container MySQL tạm `cobo-bugfue-mysql` (DB `cobo_iam_mr1`) chờ xoá.
