# PR-1a: Completion report (wf-bugfix), 2026-10-11

Trạng thái: **đã implement và verify ở local, CHƯA commit, CHƯA deploy.** Q-25 (thao tác trên server DEV), commit và deploy đều đang chờ user cho phép.

Phạm vi theo `02a-plan-pr1-reminder.md` (PR-1a) và các quyết định Phase 0 (PO đồng ý toàn bộ, 2026-10-11).

```text
Bug:
  REM-01  Milestone trùng làm occurrence SENT bị reset về PENDING, nên reminder bị gửi lặp.
  REM-06  Milestone "chết" (instance đóng / bước xong / bước hết hạn) nằm lại hàng đợi mãi, chặn batch 200 dòng của scanner.
  RMC-01  Workflow global_template / company_override chỉ có task cho bước 1, nên từ bước 2 trở đi không được nhắc.
  RMC-04  Bước trước trễ làm hạn bước sau bị dời, nhưng reminder vẫn bắn theo ngày cũ.
  RMC-06  Trạng thái reminder trên API lấy từ reminder_sent=1 nên luôn báo "sent", kể cả khi gửi lỗi hoặc bị huỷ.
  DUP-*   Snapshot / override / proposal / facade CMS chấp nhận step_id trùng, sinh milestone_id trùng (nguồn của REM-01).

Reproduction (GATE R: đã fail trên HEAD a8acb51 trước khi sửa):
  internal/reminder/app/workflow_step_seed_regression_test.go
    TestSeedOccurrences_DuplicateMilestonesKeepSentOccurrence   -> "tick 0: occurrence status=PENDING, want SENT"
  internal/reminder/app/workflow_step_eligibility_test.go
    TestSeedOccurrences_LegacyStep2EligibleAfterStep1CompletedOnScreen, TestDecideWorkflowStepReminder,
    TestSeedOccurrences_DeadMilestonesDoNotStarveBatch, TestSeedOccurrences_WaitRowsDoNotStarve  -> fail biên dịch (API mới)
  internal/reminder/infra/mysql/workflow_step_runtime_reader_test.go                              -> fail biên dịch
  internal/workflow/app/snapshot_duplicate_test.go                                                -> fail biên dịch
  internal/disclosure/app/workflow_override_duplicate_test.go                                     -> fail biên dịch
  internal/adhoc/app/runtime_materialize_duplicate_test.go                                        -> fail runtime
  internal/workflow/infra/mysql/milestones_status_test.go                                         -> fail biên dịch

Root cause (evidence): xem 01-root-cause.md (REM-01, REM-06, RMC-01, RMC-04, RMC-06).

Fix (files):
  Reminder
    internal/reminder/app/contracts.go                        MilestoneCursor, MilestonePageScanner, MilestoneObsoleter,
                                                              OccurrenceEnsurer, WorkflowStepRuntime(+Reader)
    internal/reminder/app/workflow_step_eligibility.go (mới)  decideWorkflowStepReminder: Eligible / Wait / Obsolete,
                                                              grace 2 ngày, giữ Wait cho bước chưa tới lượt (PERF-48)
    internal/reminder/app/service.go                          phân trang theo keyset (tối đa 5x200), EnsureOccurrence
                                                              (không reset), cache runtime trong 1 tick, metric
                                                              reminder_milestone_obsoleted_total{reason}
    internal/reminder/infra/mysql/milestone_scanner.go        keyset (scheduled_date, milestone_id), MarkMilestoneObsolete
                                                              (reminder_sent=1, reminder_occurrence_id=NULL; không dùng giá trị 2)
    internal/reminder/infra/mysql/repository.go               EnsureOccurrence: INSERT ... ON DUPLICATE KEY UPDATE id=id
    internal/reminder/infra/inmemory/repository.go            EnsureOccurrence
    internal/reminder/infra/mysql/workflow_step_runtime_reader.go (mới)
                                                              ComputeDeadlineSteps trên workflow_instance_step_states,
                                                              scope theo company_id (BES-43)
    cmd/worker/main.go, internal/httpserver/server.go         wiring WithWorkflowStepRuntimeReader
  Chặn step trùng
    internal/workflow/errs/snapshot.go                        ErrDuplicateWorkflowStep
    internal/workflow/app/snapshot.go                         ValidateSnapshot báo trùng step_id/step_code (không phân biệt hoa/thường),
                                                              SnapshotValidationMessage
    internal/workflow/app/service.go, internal/adhoc/infra/disclosure/record_creator.go,
    internal/disclosure/infra/workflow/bootstrap.go           dùng SnapshotValidationMessage
    internal/disclosure/app/workflow_override_validation.go   400 CodeWorkflowOverrideInvalid, field_errors["workflow[i].step_id"]
    internal/disclosure/app/cms_service.go                    normalizeFacadeStepIdentities (chặn trùng cả ID nhập vào và ID tự sinh;
                                                              so không phân biệt hoa/thường, API-46)
    internal/adhoc/app/runtime_materialize.go                 422 "proposal step id must be unique"
    internal/disclosure/app/periodic.go                       trùng step -> MarkPeriodicCycleFailed("DUPLICATE_WORKFLOW_STEP") + log ERROR
                                                              (API-45/PERF-51)
    internal/disclosure/app/service.go                        SubmitRecord: log WARN khi bootstrap lỗi
  Trạng thái reminder (RMC-06)
    internal/workflow/infra/mysql/milestones.go               LEFT JOIN reminder_occurrences (scope WORKFLOW_STEP + instance, BES-44),
                                                              scan reminder_sent dạng int, ra pending|sent|failed|cancelled
  Không đổi schema, không có migration (phù hợp H19: deploy-be không chạy migrate).

Verification (GATE V, 2026-10-11, sau mọi bản sửa từ review):
  go build ./...                                OK
  go test ./...                                 16 fail, trùng khớp baseline HEAD: companyaccess/app x1, companyaccess/transport/http x1,
                                                disclosure/app x6 (template-import), legal_basis_backfill x1, httpserver integration x5,
                                                notification VariableParity x1, platform/config x1. Không có fail mới.
  go vet ./...                                  chỉ còn cảnh báo copylocks có sẵn ở internal/workflowfulfillment/required_document_gate_test.go
  gofmt -l (bản copy LF của 27 file .go đổi)    sạch
  go test -race ./internal/reminder/... ./internal/workflow/infra/mysql/...   OK
  docker compose -f docker-compose.dev.yml build api                          OK
  go build ./cmd/api ./cmd/worker trong image cobo_iam_services_api            OK (image dev dùng `go run` khi start nên phải
                                                                                compile riêng mới có bằng chứng)
  Race có sẵn từ trước (HEAD cũng fail): workflow/app TestV2MultiStep_ConcurrentCompleteSingleNext, ngoài phạm vi.

Pre-deploy data check trên DEV (qa_ro, read-only):
  step_id trùng (LOWER(TRIM)): snapshot workflow_instances 0, override versions 0, global_workflow_steps 0 (cả step_key),
    ad_hoc_proposals.proposed_workflow_json 0, milestone 0
    -> validation mới không chặn dữ liệu đang có; periodic không bị chuyển FAILED hàng loạt.
  136 milestone chết: tick đầu sẽ xử lý xong trong 1 lượt (tối đa 1000/tick), metric obsoleted tăng một đợt.
  occurrence PENDING/RETRY cũ hơn 48h: 0; occurrence gửi lặp: 0; snapshot lệch display_order: 0.

Review (Phase 6): be-security, perf-reliability, api-compat, cache-versioning (kiểm một phần).
  Không có CRITICAL/HIGH.
  Đã sửa: BES-43 (scope company_id khi đọc step states), BES-44 (scope khi JOIN occurrence), API-45 (FAILED khi trùng + log
    bootstrap), API-46 (so trùng không phân biệt hoa/thường), PERF-48 (bước chưa tới lượt thì Wait), PERF-51 (log/metric periodic).

Blast radius / data repair:
  Không cần repair dữ liệu: milestone chết tự được đánh obsolete khi worker chạy bản mới.
  Mixed-version: bản cũ (ListByInstance) scan reminder_sent dạng bool, vẫn an toàn vì bản mới chỉ ghi 0/1. Sàn rollback worker: cabbecf.
  Client API: status có thêm giá trị failed/cancelled. Chưa kiểm FE xử lý giá trị lạ thế nào, ghi vào API-48.

Follow-ups:
  PERF-49  index (reminder_sent, scheduled_date, milestone_id), cần migration -> PR-W6
  PERF-50  kiểm lại eligibility lúc dispatch -> PR-1b
  PERF-52  gauge backlog milestone/occurrence
  PERF-53  ghi vào plan: đã bỏ FOR UPDATE SKIP LOCKED (scanner chỉ đọc; idempotency dựa vào unique key)
  API-47   (tuỳ chọn) FE hiển thị thông điệp "workflow step_id must be unique"
  API-48   tài liệu enum trạng thái reminder pending|sent|failed|cancelled
```

## Rollout (chờ user cho phép từng bước)

1. Q-25 trên DEV: cách ly binary worker/api cũ, tắt 6 template QA chu kỳ hằng ngày, bật allowlist email, rotate `INTERNAL_REMINDER_TOKEN`.
2. Commit PR-1a.
3. Dark launch: deploy với `REMINDER_DISPATCH_ENABLED=false`, recreate worker, theo dõi `reminder_milestone_obsoleted_total` và số occurrence mới; sau đó bật dispatch.
4. Hạn chót: 2026-10-12. REM-06 sẽ bắt đầu chặn scanner quanh 13–14/10.
