# PR-1a: Completion report (wf-bugfix), 2026-10-11

Trạng thái: **đã commit (`0a63b2d` code, `683f65d` docs; chưa push) và đã deploy lên DEV ngày 2026-10-11.** User đồng ý Q-25, commit và deploy ("đồng ý 1, 2, 3"). Nhật ký ở cuối file.

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

## Nhật ký rollout DEV 2026-10-11 (UTC)

Trước khi thao tác (qa_ro, 01:58):
- 1300 milestone đến hạn chưa gửi, nhưng scanner chỉ lấy `due_minus_%`: 232 dòng (101 `due_minus_1d`, 131 `due_minus_3d`). 673 `step_start` và 395 `step_end` không thuộc phạm vi scanner.
- Ước lượng bằng SQL theo cùng thứ tự quyết định: 0 eligible, 214 wait (bước trước chưa xong), 18 obsolete. Kết quả thật sau deploy khớp đúng.
- 0 occurrence PENDING/RETRY đến hạn. Mốc id: occurrence max id 1416798, delivery max id 702.

1. **Q-25(b) DATA-01 (02:01):** archive 6 template QA chu kỳ hằng ngày qua `POST /api/v1/platform/cms/templates/{type_id}/archive`, dùng persona `CMS` (có `cms.template.archive`), lý do có ghi audit. Kết quả: cả 6 đều `archived`, `active_version_no` 0, `archived_from_version_no` 1; không còn template daily nào active. Khôi phục: `POST .../restore`.
   - `qa_rw` chỉ có quyền ghi trên `membership_direct_permissions` và `role_permissions`, không UPDATE được `disclosure_types`, nên mới phải dùng API. Không dùng root.
2. **Q-25(a) (02:02):** sao lưu binary đang chạy thành `bin/{api,worker}.rollback.20261011T020205Z` (mốc rollback); chuyển 26 binary cũ (`worker.bak*`, `worker.rollback.20261002T*`, `api.bak*`, `api_new`, `api.new.*`) vào `bin/quarantine/` và đặt quyền 000. Chỉ di chuyển, không xoá.
3. **Q-25(d) (02:02):** sao lưu `.env` thành `.env.bak-q25-20261011T020205Z` (quyền 600). Rotate `INTERNAL_REMINDER_TOKEN` bằng `openssl rand -hex 32`, giá trị không in ra đâu. Token cũ trùng với giá trị đã commit trong `deploy-artifacts/_tmp_seed_reminder_smoke.py` (xác nhận SEC-26/H22). Sau deploy, `/metrics` với token cũ trả 401. Prometheus scrape qua mạng nội bộ Docker, không dùng token, nên không bị ảnh hưởng.
4. **Q-25(c) allowlist email: KHÔNG làm được bằng cấu hình.** Code chưa có allowlist (SEC-27 nằm ở PR-W5). Chuyển `SMTP_HOST` sang Mailpit thì sẽ lộ link reset qua cổng :8025 public (C6). Thay vào đó dùng chốt chặn theo số liệu: dark launch, đếm occurrence sẽ gửi, chỉ bật lại gửi khi hàng đợi gần như trống.
5. **Dark launch (02:03):** `REMINDER_DISPATCH_ENABLED=false`, chạy `deploy-dev.ps1 -Mode be`. Hash sha256 của binary trên server khớp bản build local; `/healthz` và `/readyz` OK; env của worker và token trong api khớp `.env`.
   - Sau 1 phút: obsolete 0 → 18; wait 214; 0 occurrence mới; 0 delivery; log worker chỉ có INFO.
6. **Bật gửi (02:05):** `REMINDER_DISPATCH_ENABLED=true`, recreate worker. Lúc 02:06: 0 ERROR/WARN; 0 occurrence đến hạn; 0 delivery mới; 0 occurrence gửi lặp.

Rollback (nếu cần): copy `bin/{api,worker}.rollback.20261011T020205Z` đè lên `bin/{api,worker}`, rồi `docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker`. Dữ liệu tương thích (chỉ ghi `reminder_sent` 0/1).

Follow-up mới phát sinh khi rollout:
- PERF-54: cursor phân trang đặt lại mỗi tick. Nếu số dòng wait vượt 1000 (5×200) thì các dòng phía sau bị bỏ đói. DEV hiện có 214 và DATA-01 đã tắt nên không còn tăng nhanh. Hướng sửa: cursor xoay vòng giữa các tick.
- PERF-55: worker không có endpoint `/metrics`, nên `reminder_milestone_obsoleted_total` (đếm trong worker) không scrape được. Tạm theo dõi bằng SQL `reminder_sent=1 AND reminder_occurrence_id IS NULL`. Gộp vào PERF-52 (gauge từ MySQL ở phía API).
- SEC-29: xoá literal token (đã chết) khỏi `deploy-artifacts/_tmp_seed_reminder_smoke.py`, chuyển sang đọc env.
- SEC-27 (allowlist email) vẫn là điều kiện trước khi lượng email tăng theo RMC-01.
- Theo dõi 24h theo `03` §5 và `04` §8.

## Sửa tiếp sau rollout (2026-10-11, chưa commit, chưa deploy)

- **PERF-54 (đã sửa):** `SeedOccurrencesFromDueMilestones` giữ cursor giữa các tick (`seedCursor`, có `seedMu`). Tick sau đọc tiếp từ chỗ tick trước dừng; tới cuối hàng thì quay về đầu một lần và dừng khi gặp lại điểm bắt đầu (`milestoneBeforeCursor`). Scanner không phân trang vẫn chạy như cũ.
  - Tái hiện: `TestSeedOccurrences_WaitRowsBeyondTickBudgetDoNotStarve` (1300 dòng chờ + 1 dòng đủ điều kiện). Fail trên `141a242`, pass sau khi sửa.
  - Test bảo vệ: `TestSeedOccurrences_SeedCursorWrapsAround` (dòng nằm trước cursor được xử lý sau khi quay vòng).
- **SEC-29 (đã sửa):** `deploy-artifacts/_tmp_seed_reminder_smoke.py` đọc `DEV_API_BASE_URL` và `INTERNAL_REMINDER_TOKEN` từ env; đã bỏ token, IP và email gắn cứng (lịch sử git vẫn còn token cũ, nhưng token đó đã bị rotate).
- Verify: `go build ./...` OK; `go test ./...` 16 fail đúng baseline; vet chỉ còn copylocks có sẵn; gofmt (bản LF) sạch; `go test -race ./internal/reminder/...` OK; Docker build api và compile `cmd/api`, `cmd/worker` trong image OK.
