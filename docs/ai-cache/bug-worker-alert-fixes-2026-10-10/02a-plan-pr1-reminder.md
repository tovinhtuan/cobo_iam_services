# PR-1 Reminder: kế hoạch triển khai chi tiết (wf-bugfix Phase 3–6, chỉ lập kế hoạch)

> **Tóm tắt (VI)**
> - **Hạn chót:** PR-1a (commit 1–5) phải deploy trước **2026-10-12**, để REM-06 không chặn scanner.
> - **Không đổi schema.** Migration `0153` để dành cho PR-W6.
> - **Thành phần dùng chung:**
>   - (A) Reader trạng thái bước runtime: đọc `workflow_instances` + `workflow_instance_step_states`, tính bằng `ComputeDeadlineSteps` theo giờ HCM.
>   - (B) Hàm quyết định thuần `Eligible | Wait | Obsolete`.
>   - (C) Các method repo mới: `EnsureOccurrence`, `MarkMilestoneObsolete`, `DeferOccurrence`/`ExpireOccurrence`, `RecordDispatchOutcome`.
> - **Không bao giờ ghi `reminder_sent=2`.** Milestone hết ý nghĩa ghi `reminder_sent=1` + `reminder_occurrence_id=NULL`.
> - **Quyết định mở (khuyến nghị):** grace 2 ngày; trễ quá 48h thì hết hạn; khung gửi 07–21 giờ HCM; REM-04 theo phương án B; tách PR-1a để kịp hạn.
>
> Nguồn: agent lập kế hoạch PR-1 (read-only, HEAD `a8acb51`). Main agent đã kiểm chứng các HIGH ở báo cáo audit.

**Baseline:**
- `go test ./internal/reminder/... ./internal/workflow/app/ ./internal/deadlinealerts/app/` pass ở HEAD.
- Fail có sẵn: `notification/app` `TestContract_VariableParity/workflow.approved`; 6 test template-import trong `disclosure/app`.

**Thành phần dùng chung**
- **(A) `WorkflowStepRuntimeReader`** tại `internal/reminder/infra/mysql/workflow_step_runtime_reader.go`:
  - Đọc `workflow_instances` (`status`, `workflow_source`, `t0_date`, `snapshot_json`; `WHERE company_id=? AND workflow_instance_id=?`) và `workflow_instance_step_states`.
  - Gọi `deadlinealertsapp.ComputeDeadlineSteps(..., asOf.In(HCM), "Asia/Ho_Chi_Minh", false)`. Truyền ngày HCM để không dính lỗi DLA-01.
  - Nguồn proposal dùng lại `WorkflowStepTaskStateReader`.
  - Không có import cycle: `go list -deps ./internal/deadlinealerts/app` không chứa package reminder.
- **(B) `decideWorkflowStepReminder(rt, stepID, offsetN, asOf)`** tại `internal/reminder/app/workflow_step_eligibility.go`: trả `Eligible | Wait | Obsolete(reason)` kèm `StepEnd` (đã cộng độ trễ).
- **(C)** Method repo mới: `EnsureOccurrence`, `MarkMilestoneObsolete`, `DeferOccurrence` / `ExpireOccurrence`, `RecordDispatchOutcome`.
- **Hằng số:** `reminderMaxLateness = 48h`, `reminderStepGraceDays = 2`.

---

### RMC-01 Bước 2..N không bao giờ được nhắc; bước 1 vẫn bị nhắc sau khi xong
- **Mức độ / nguồn:** HIGH, audit chức năng RMC-01; là nguyên nhân gốc của REM-06.
- **Nguyên nhân gốc:**
  - Điều kiện nhắc = "có `workflow_task` pending cho `step_code` này". Instance không phải proposal chỉ có task bước 1, và hoàn tất bước trên màn hình hạn chỉ ghi `workflow_instance_step_states`.
  - Bằng chứng:
    - `internal/reminder/app/workflow_step_reminder.go:48-56`; `infra/mysql/workflow_step_task_state.go:28-33`.
    - `internal/workflow/app/service.go:388-391` → `planLegacyCompleteIfNoPending` (`:498-524`), không tạo task tiếp theo.
    - `deadlinealerts/infra/mysql/steps_repository.go:99-173`.
  - Tầng worker/service. **Confirmed** (code + DEV: bước 2 = 0/125, bước 3 = 0/10; 323 instance có 323 task).
- **Test tái hiện (GATE R, unit)**, `internal/reminder/app/workflow_step_eligibility_test.go`:
  - `TestSeedOccurrences_LegacyStep2EligibleAfterStep1CompletedOnScreen`:
    - Scanner giả có `due_minus_1d` cho bước 1 và bước 2.
    - `fakeStepTaskStateReader{step-1:Pending, step-2:Absent}` (đúng hình dạng DEV).
    - Runtime reader giả: `global_template`, bước 1 có `CompletedAt`.
    - Kỳ vọng: chỉ seed bước 2, bước 1 bị obsolete. HEAD seed bước 1, nên **FAIL**.
  - `TestDecideWorkflowStepReminder` (bảng): proposal pending/absent/completed; legacy current/not-current/completed; instance approved; đã qua `step_end` + 2 ngày.
- **Thiết kế sửa:**
  - `SeedOccurrencesFromDueMilestones` (`service.go:1002`) dùng (B) thay cho `workflowStepReminderEligible`. Kiểm theo task cũ chỉ giữ làm fallback khi (A) chưa được gắn.
  - Quy tắc quyết định:
    - Instance không còn, hoặc status ∈ {`approved`, `rejected`, `cancelled`, `completed`} → Obsolete.
    - Bước không có trong snapshot → Obsolete.
    - `todayHCM > StepEnd + 2d` → Obsolete `STEP_EXPIRED`.
    - Bước đã hoàn tất → Obsolete.
    - Nguồn proposal (`workflowapp.IsProposalSnapshotFrozen`): task Pending → Eligible; Absent → Wait.
    - Nguồn khác: `CurrentStepCode == step` → Eligible; còn lại → Wait.
  - Ánh xạ `step_id` của milestone vào snapshot theo `StepID` hoặc `StepCode`.
  - Cache (A) theo instance trong mỗi tick.
  - Gắn ở `cmd/worker/main.go:209-214` và `internal/httpserver/server.go:613-616` qua `WithWorkflowStepRuntimeReader(...)`.
  - KHÔNG đổi: planner workflow, việc tạo task, `ComputeDeadlineSteps`, lỗi DLA-01 của màn hình hạn (sửa ở PR-2).
- **Contract / schema / FE:** không đổi. Hệ quả có chủ đích: email nhiều hơn vì bước 2..N giờ cũng được nhắc.
- **Dữ liệu:** không cần sửa. Ước lượng lượng email tăng trước khi deploy:
  ```sql
  SELECT COUNT(DISTINCT s.workflow_instance_id)
  FROM workflow_instance_step_states s
  JOIN workflow_instances wi USING (workflow_instance_id)
  WHERE s.completed_at IS NOT NULL
    AND wi.status = 'in_progress'
    AND COALESCE(wi.workflow_source, '') NOT LIKE 'proposal_snapshot%';
  ```
  DEV hiện `workflow_instance_step_states` có 0 dòng.
- **Rollout:**
  - Dark launch: đặt `REMINDER_DISPATCH_ENABLED=false` trong `.env`, rồi recreate worker bằng `up -d --no-deps worker` (`restart` không đọc lại `.env`, SEC-28).
  - Quan sát 2 tick rồi mới bật lại `true`.
  - Rollback về `a8acb51` tương thích dữ liệu, nhưng mở lại RMC-01.
- **Verify (GATE V):**
  - `go test ./internal/reminder/app/ -run 'TestSeedOccurrences_|TestDecideWorkflowStepReminder' -count=1`
  - `MYSQL_TEST_DSN=… go test ./internal/reminder/infra/mysql/ -run TestMySQLWorkflowStepRuntimeReader -count=1`
  - DEV: tỉ lệ đã seed/đến hạn theo `step_order` cho nguồn không phải proposal; bước ≥2 phải > 0 sau khi có bước được hoàn tất.
- **Phạm vi ảnh hưởng:**
  - Giả định "chỉ dựa vào task" còn ở policy task-assignee trong `recipient_policy_filter.go`, chỉ chạy khi bật `NOTIFICATION_RULES_CONSUMER_ENABLED` (→ RMC-07).
  - `StepTaskState` không có caller nào khác.
- **Phụ thuộc / quyết định:** không có mục chặn. Grace = 2 ngày (khuyến nghị).
- **Ước lượng:** M

### REM-06 Milestone chết chặn scanner (~13–14/10)
- **Mức độ / nguồn:** HIGH, risk review #1 (PERF-40 + API-35).
- **Nguyên nhân gốc:** dòng không đủ điều kiện chỉ `continue`, không ghi gì (`service.go:1002-1004`). Scanner lấy `reminder_sent=0 … ORDER BY scheduled_date LIMIT 200` (`milestone_scanner.go:25-33`), nên các dòng này chặn đầu hàng đợi. Tầng worker. **Confirmed** (DEV: 136 dòng, tăng khoảng 15/ngày).
- **Test tái hiện (unit)**, `workflow_step_reminder_seed_test.go`. Trước tiên sửa `fakeMilestoneScanner` để tôn trọng `limit`, thứ tự và cursor (hiện nó bỏ qua `limit`):
  - `TestSeedOccurrences_DeadMilestonesDoNotStarveBatch`: 200 dòng của instance đã đóng (D-10..D-1) + 1 dòng đủ điều kiện hôm nay. Sau 2 tick, dòng đủ điều kiện phải được seed và 200 dòng bị obsolete. HEAD → **FAIL**.
  - `TestSeedOccurrences_WaitRowsDoNotStarve`: 250 dòng Wait + 1 dòng đủ điều kiện → dòng đó được seed ngay tick 1.
- **Thiết kế sửa:**
  - `MilestoneScanner.MarkMilestoneObsolete(ctx, id)`:
    ```sql
    UPDATE workflow_step_milestones SET reminder_sent=1, reminder_occurrence_id=NULL
    WHERE milestone_id=? AND reminder_sent=0
    ```
    **Không bao giờ dùng giá trị 2**: `ListByInstance` scan cột này vào `bool` (`workflow/infra/mysql/milestones.go:67`).
  - Metric `reminder_milestone_obsoleted_total{reason}`.
  - Phân trang: `ListDueMilestones(ctx, asOf, after Cursor, limit)` với điều kiện `AND (scheduled_date > ? OR (scheduled_date = ? AND milestone_id > ?)) ORDER BY scheduled_date, milestone_id`. Mỗi tick service duyệt tối đa 5 trang (1000 dòng).
  - KHÔNG: thêm index (để `0153` sau: `(reminder_sent, scheduled_date)`; bảng hiện khoảng 7 nghìn dòng); không đụng SKIP LOCKED (REM-02).
- **Contract:** `reminder_sent` vẫn 0/1, nên API không thể trả 500. Dòng obsolete chỉ hiển thị "cancelled" khi có RMC-06, nên phải ship cùng RMC-06.
- **Sửa dữ liệu (bản nháp, chỉ dùng nếu deploy trễ quá 2026-10-12):**
  - Kiểm tra trước:
    ```sql
    SELECT COUNT(*) FROM workflow_step_milestones
    WHERE reminder_sent=0 AND milestone_type LIKE 'due_minus_%' AND scheduled_date < UTC_DATE();
    ```
    Kỳ vọng khoảng 136 vào 10/10 và khoảng 180 vào 13/10.
  - Sửa:
    ```sql
    CREATE TEMPORARY TABLE tmp_rem06 AS
    SELECT m.id FROM workflow_step_milestones m
    JOIN workflow_instances wi ON wi.workflow_instance_id = m.workflow_instance_id
    LEFT JOIN workflow_step_milestones e
      ON e.workflow_instance_id = m.workflow_instance_id
     AND e.step_id = m.step_id
     AND e.milestone_type = 'step_end'
    WHERE m.reminder_sent = 0
      AND m.milestone_type LIKE 'due_minus_%'
      AND m.scheduled_date < UTC_DATE()
      AND NOT EXISTS (SELECT 1 FROM workflow_instance_step_states s
                      WHERE s.workflow_instance_id = m.workflow_instance_id AND s.delay_days_applied > 0)
      AND (wi.status IN ('approved','rejected','cancelled')
           OR e.scheduled_date < DATE(CONVERT_TZ(UTC_TIMESTAMP(), '+00:00', '+07:00')) - INTERVAL 2 DAY);
    SELECT COUNT(*) FROM tmp_rem06;
    UPDATE workflow_step_milestones m JOIN tmp_rem06 t ON t.id = m.id
    SET m.reminder_sent = 1, m.reminder_occurrence_id = NULL
    WHERE m.reminder_sent = 0;
    ```
- **Rollout:** tick đầu tiên tự dọn. Rollback an toàn (binary cũ coi các dòng này là đã gửi). Tắt template QA hằng ngày (DATA-01) chỉ làm chậm tốc độ tăng.
- **Verify:**
  - `go test -race ./internal/reminder/app/ -run 'Starve' -count=1`
  - DEV: số dòng chết giảm từ 136+ xuống dưới khoảng 40 (chỉ còn các dòng trong thời gian grace).
  - `SELECT COUNT(*) … reminder_sent=1 AND reminder_occurrence_id IS NULL` xấp xỉ số dòng chết trước đó.
- **Phạm vi ảnh hưởng:** `reminder_occurrence_id` chỉ được dùng ở scanner và `ListByInstance`. `ListDispatchCandidates` chỉ join `wsm_end`.
- **Phụ thuộc:** (A) và (B) của RMC-01. Nếu RMC-01 trễ: hotfix một quy tắc obsolete chỉ dựa vào trạng thái instance, `step_end` + 2 ngày, và task Completed.
- **Ước lượng:** M

### REM-01 Upsert seed đặt lại SENT → PENDING
- **Mức độ / nguồn:** HIGH, có thể thành sự cố (khoảng 17.280 email/ngày cho mỗi key; hết hạn mức Gmail của mọi tenant). Risk review #3 (PERF-38, BES-37, API-36, PERF-47).
- **Nguyên nhân gốc:**
  - `SeedOccurrence` chạy `ON DUPLICATE KEY UPDATE … scheduled_at=VALUES(..), status=VALUES(status)` (`internal/reminder/infra/mysql/repository.go:286-300`).
  - Sau đó đọc lại theo `occurrence_id` mới nên ra 404 (`:305-311`, `:528`), milestone không được đánh dấu (`service.go:1016-1020`), và dispatch (`repository.go:444-450`) gửi lại ngay trong tick.
  - Điều kiện kích hoạt: `step_id` trùng.
    - Validate chỉ kiểm rỗng (`disclosure/app/workflow_override_validation.go:25-39`; `workflow/app/snapshot.go:169-174`).
    - CMS facade (plausible): ID tự sinh `%s-step-%d` có thể trùng ID khai báo (`cms_service.go:272-275`), và sau khi gán chỉ `step_key` được kiểm lại (`:118`).
  - Tầng DB/worker + validate. **Confirmed** (code). DEV: 0 nhóm trùng.
- **Test tái hiện:**
  - **Unit, sửa PERF-47:** thay `seedOccurrenceRepo` (báo lỗi khi trùng, nên đã che lỗi) bằng `mysqlLikeOccurrenceRepo`.
    - Mô phỏng `uq_reminder_occurrence_id` và `uq_reminder_occurrence_idempotency`.
    - `SeedOccurrence` theo đúng upsert ở HEAD: reset khi trùng key, rồi tìm theo `occurrence_id` ra 404.
    - `EnsureOccurrence` chỉ insert khi chưa có.
    - `TestSeedOccurrences_DuplicateMilestonesKeepSentOccurrence`: m1 và m2 trùng (inst, step, `due_minus_3d`); chạy tick, đánh dấu SENT, chạy tick. Kỳ vọng vẫn SENT, cả 2 milestone được đánh dấu, không có candidate. HEAD → **FAIL**.
  - **Source guard (unit):** `infra/mysql/occurrence_seed_guard_test.go` `TestEnsureOccurrence_DoesNotOverwriteState`, cùng kiểu với `workflow/infra/mysql/repository_atomic_materialization_test.go`: thân hàm không được chứa `status = VALUES(status)`.
  - **mysql-integration:** `TestMySQLEnsureOccurrence_DuplicateKeyReturnsExistingSent` (`MYSQL_TEST_DSN`, tên DB phải chứa "test").
  - **Validate:**
    - `workflow/app/snapshot_test.go` `TestValidateSnapshot_RejectsDuplicateStepIdentity`
    - `disclosure/app/workflow_override_validation_test.go` `TestValidateCompanyWorkflowOverrideSteps_RejectsDuplicateStepID` (400 `CodeWorkflowOverrideInvalid`, `field_errors["workflow[1].step_id"]`)
    - file mới `cms_workflow_facade_identity_test.go` `TestFacadeStepIdentities_GeneratedIDCollision` (`[{""},{"T-step-1"}]`)
    - `adhoc/app` `TestValidateFrozenProposalWorkflowForRuntime_DuplicateStepID`
- **Thiết kế sửa:**
  - Method mới `OccurrenceRepository.EnsureOccurrence`:
    ```sql
    INSERT … ON DUPLICATE KEY UPDATE id = id;
    SELECT … WHERE idempotency_key = ?;
    ```
    Không có dòng thì trả lỗi kèm metric.
  - Bridge (`service.go:1007`) dùng `EnsureOccurrence` và đánh dấu milestone theo occurrence tìm được.
  - Giữ `SeedOccurrence` cho route dev seed.
  - Repo in-memory và fake test mô phỏng cả 2 key.
  - Validate:
    - `ValidateSnapshot` trả `errs.ErrDuplicateWorkflowStep` (định danh = {StepID, StepCode}, so sánh không phân biệt hoa/thường).
    - Thêm kiểm trùng vào vòng lặp override.
    - CMS: chạy lại `rejectDuplicateIncomingIdentities(req.Steps)` sau `assignFacadeStepKeys`.
    - Ad-hoc: `runtime_materialize.go:23` kiểm `step.ID` trùng.
    - `createWorkflowInstance` trả thông điệp "workflow step_id must be unique".
  - KHÔNG: thêm UNIQUE cho milestone (API-36, để PR-W6); không đổi route dev seed.
- **Contract:** thêm phản hồi 400/422 khi `step_id` trùng ở override draft/approve/rebase, CMS upsert, ad-hoc finalize. FE cần kiểm editor override/CMS có hiển thị `field_errors` không (`mergeIncomingWorkflowSteps` ở web chưa kiểm).
- **Dữ liệu:** không cần sửa. Kiểm tra trước, kỳ vọng tất cả = 0:
  ```sql
  SELECT workflow_instance_id, step_id, milestone_type, COUNT(*) c
  FROM workflow_step_milestones
  WHERE milestone_type LIKE 'due_minus_%'
  GROUP BY 1, 2, 3 HAVING c > 1;

  -- xác minh cấu trúc workflow_json trước khi chạy
  SELECT v.override_id, v.version_no, j.sid, COUNT(*) c
  FROM company_template_workflow_override_versions v
  JOIN JSON_TABLE(v.workflow_json, '$[*]' COLUMNS (sid VARCHAR(128) PATH '$.step_id')) j
  GROUP BY 1, 2, 3 HAVING c > 1;

  SELECT wi.workflow_instance_id, j.sid
  FROM workflow_instances wi
  JOIN JSON_TABLE(wi.snapshot_json, '$[*]' COLUMNS (sid VARCHAR(128) PATH '$.step_id')) j
  GROUP BY 1, 2 HAVING COUNT(*) > 1;

  SELECT occurrence_id
  FROM reminder_delivery_attempts
  GROUP BY 1 HAVING SUM(status = 'SENT') > 1 OR COUNT(*) > 5;
  ```
  Nếu một override đang active đã có ID trùng: từ nay materialize periodic sẽ retry mỗi 60 giây (PER-03) thay vì gửi spam.
- **Rollout:** `deploy-be` (API + worker). Rollback an toàn nhưng mở lại vòng lặp.
- **Verify:**
  - `go test ./internal/reminder/... ./internal/workflow/app/ ./internal/adhoc/app/ -count=1`
  - `go test ./internal/disclosure/app/ -run 'WorkflowOverride|Facade' -count=1`
  - Integration test.
  - DEV: 2 query giám sát trả 0.
- **Phạm vi ảnh hưởng:**
  - `status = VALUES(status)` chỉ còn ở `iam/registrationmysql/register_public.go:62` (`role_permissions`, vô hại).
  - Caller của `ValidateSnapshot`: `workflow/app/service.go:82`, `disclosure/infra/workflow/bootstrap.go:41`, `adhoc/infra/disclosure/record_creator.go:213,235,260`.
- **Phụ thuộc:** không. Phải vào **trước** RMC-01, vì RMC-01 sẽ seed thêm bước.
- **Ước lượng:** M

### REM-05 Occurrence bị skip không bao giờ có trạng thái kết thúc
- **Mức độ / nguồn:** HIGH, risk review #4 (PERF-39, API-37).
- **Nguyên nhân gốc:** skip chỉ `continue` (`service.go:346-357`). Candidate lấy `ORDER BY scheduled_at LIMIT 50`, không có cận dưới (`repository.go:444-451`). **Confirmed** (code). DEV có 0 dòng loại này.
- **Test tái hiện (unit)**, `internal/reminder/app/dispatch_skip_defer_test.go`:
  - `TestDispatchDue_SkippedOccurrenceIsDeferred`: 60 candidate có template bị tắt. Mỗi dòng phải chuyển `RETRY_SCHEDULED`, `next_retry=now+1h`, `last_error_code=TEMPLATE_DISABLED`; tick sau xử lý 0 dòng. HEAD không update dòng nào → **FAIL**.
  - `TestDispatchDue_SkippedOlderThan48hExpires`.
- **Thiết kế sửa:**
  ```sql
  -- DeferOccurrence
  UPDATE reminder_occurrences
  SET status='RETRY_SCHEDULED', next_retry_at=?, last_error_code=?, updated_at=CURRENT_TIMESTAMP(3)
  WHERE occurrence_id=? AND status IN ('PENDING','RETRY_SCHEDULED');
  -- ExpireOccurrence: cùng điều kiện, status='FAILED', next_retry_at=NULL, ghi last_error_code/message
  ```
  - Khi skip: quá 48h → Expire `REMINDER_EXPIRED`; còn lại → Defer +1h. Không ghi attempt, không tăng `attempt_count`.
  - `infra/observe/db_collector.go:162`: loại `last_error_code IN ('REMINDER_EXPIRED','REMINDER_OBSOLETE')`, nếu không alert `cobo_reminder_failed_recent` (`deploy/monitoring/reminder_alert_rules.yml:15`) sẽ bật.
  - KHÔNG thêm giá trị ENUM (phải migrate trước, API-37).
- **Contract:** web không đọc status của occurrence (grep không thấy). API history vẫn chỉ trả các giá trị enum hiện có.
- **Dữ liệu:**
  - `SELECT status, COUNT(*) FROM reminder_occurrences GROUP BY 1;` → toàn SENT (241+).
  - Dòng `IN ('PENDING','RETRY_SCHEDULED') AND scheduled_at < UTC_TIMESTAMP() - INTERVAL 48 HOUR` → 0. Nếu khác 0, liệt kê các dòng đó trước, vì tick đầu sẽ chuyển chúng sang hết hạn.
- **Rollout:** worker. Rollback: các dòng đã defer lại bị skip mỗi tick (mở lại REM-05), không hỏng dữ liệu.
- **Verify:** `go test ./internal/reminder/app/ -run 'Skipped' -count=1`. DEV: không còn dòng `PENDING` có `scheduled_at` cũ hơn 1 giờ.
- **Phạm vi ảnh hưởng:**
  - `handleEvaluatorError` cùng kiểu lỗi (REM-04).
  - Skip `BINDING_RESOLUTION_OWNS` (flag mặc định tắt). Quyết định mở: có defer luôn không (khuyến nghị có).
- **Ước lượng:** S

### RMC-02 Gửi dồn, gửi ban đêm, không kiểm lại, sai câu chữ khi quá hạn
- **Mức độ / nguồn:** MEDIUM, có bằng chứng DEV.
- **Nguyên nhân gốc:**
  - `DispatchDueOccurrences` (`service.go:315-390`) gửi mọi dòng đến hạn, không có cận dưới, không có khung giờ gửi.
  - Điều kiện chỉ được kiểm lúc seed.
  - Template in ra số âm: "Số ngày còn lại: {{.remaining_days}} ngày" (`templates/reminder.workflow_step_due/vi/body.txt:8`, `body.html:22`).
  - **Confirmed** (DEV: 80 bản gửi lúc 01:07 giờ VN ngày 03/10, trễ tới 3 ngày).
- **Test tái hiện (unit)**, `internal/reminder/app/dispatch_window_test.go`:
  - `TestDispatchDue_OutsideSendWindowSendsNothing`: now = 18:07Z. HEAD gửi 1 → **FAIL**.
  - `TestSeedOccurrences_LateBurstSeedsOnlyNearestOffset`: mốc 3d và 1d cùng đến hạn → chỉ seed 1d, mốc 3d bị obsolete (`SUPERSEDED`).
  - `TestDispatchDue_RechecksStepEligibility`: bước đã hoàn tất → `ExpireOccurrence(REMINDER_OBSOLETE)`, không gửi.
  - `TestPrepareDispatch_OverdueWording`.
- **Thiết kế sửa:**
  - `WithSendWindow(7, 21)` theo giờ HCM: ngoài khung thì trả kết quả rỗng trước khi lấy candidate. Env `REMINDER_SEND_WINDOW` (mặc định "07-21", "off" để tắt) trong `config.go`, gắn ở worker.
  - Bỏ mốc cũ khi seed (supersede).
  - Kiểm lại lúc dispatch cho scope `WORKFLOW_STEP` bằng (A)/(B):
    - Obsolete → Expire.
    - Wait → Defer +1h `STEP_NOT_ACTIVE`.
    - Eligible → tiếp tục.
  - Mọi scope: trễ > 48h và `attempt_count=0` → Expire.
  - `prepareDispatch` thêm biến `remaining_text`:
    - > 0 → "Số ngày còn lại: N ngày"
    - = 0 → "Hạn chót là hôm nay"
    - < 0 → "Đã quá hạn N ngày"
  - Dùng biến này trong `body.txt`/`body.html` và khai báo required trong `meta.yaml`. Giữ biến `remaining_days`.
- **Contract:** chỉ thay đổi nội dung email; thêm 1 biến template embed.
- **Dữ liệu:** không.
- **Rollout:** bật mặc định. Tắt bằng `REMINDER_SEND_WINDOW=off` rồi recreate worker.
- **Verify:**
  - `go test ./internal/reminder/... -count=1`
  - `go test ./internal/notification/app/ -run 'TestContract_VariableParity/reminder' -count=1`
  - DEV: mọi giờ gửi phải nằm trong 7..20:
    ```sql
    SELECT HOUR(CONVERT_TZ(last_attempt_at, '+00:00', '+07:00')) h, COUNT(*)
    FROM reminder_occurrences
    WHERE last_attempt_at > UTC_TIMESTAMP() - INTERVAL 1 DAY
    GROUP BY h;
    ```
- **Phạm vi ảnh hưởng:** reminder scope DISCLOSURE cũng bị giới hạn khung giờ (có chủ đích). `/internal/reminders/dispatch` không bị giới hạn.
- **Phụ thuộc:** RMC-01 (A/B), REM-05. Quyết định: sản phẩm xác nhận 48h, grace 2 ngày, và "mỗi bước chỉ một thông báo quá hạn" (khuyến nghị).
- **Ước lượng:** M

### REM-03 Ghi attempt và cập nhật trạng thái không nguyên tử
- **Mức độ / nguồn:** MEDIUM (REM-03).
- **Nguyên nhân gốc:** gửi, `InsertAttempt` và `UpdateDispatchResult` là 3 lệnh autocommit riêng, trên ctx có thể bị huỷ (`service.go:631-658`; cả `:723-742`, `:769-787`).
  - Crash sau khi gửi để lại dòng `DISPATCHING`.
  - Reaper (`repository.go:252-268`) đưa dòng đó về hàng đợi.
  - Lần gửi lại đụng `uq_reminder_attempt_occurrence_no` (`0022`) và kẹt lần nữa: khoảng 288 email/ngày.
  - **Confirmed** (code).
- **Test tái hiện:**
  - Unit `internal/reminder/app/dispatch_outcome_test.go` `TestDispatch_ReplayAfterCrashConvergesToSent`: fake attempt có unique key; lần update đầu lỗi; chạy dispatch → reaper → dispatch hai lần. Kỳ vọng ≤2 lần gửi và cuối cùng là SENT. HEAD → **FAIL**.
  - mysql-integration `TestMySQLRecordDispatchOutcome_IdempotentAndGuarded`.
- **Thiết kế sửa:** interface tuỳ chọn `DispatchOutcomeRecorder.RecordDispatchOutcome(ctx, attempt, result) (applied bool, error)`, cài ở repo MySQL và in-memory:
  ```sql
  BEGIN;
  INSERT INTO reminder_delivery_attempts (…) VALUES (…) ON DUPLICATE KEY UPDATE id = id;
  UPDATE reminder_occurrences SET … WHERE occurrence_id = ? AND status = 'DISPATCHING';
  COMMIT;
  ```
  - Một helper dùng chung cho các đường sent, retry và permanent.
  - ctx: `context.WithTimeout(context.WithoutCancel(ctx), 10s)`.
  - `applied=false` → log WARN và tăng `reminder_dispatch_outcome_stale_total`.
  - Fake không cài interface thì fallback về 2 lệnh như cũ.
  - KHÔNG đổi reaper, không đổi thứ tự "gửi rồi mới ghi".
- **Contract:** không.
- **Dữ liệu:** cả 2 query kỳ vọng 0:
  - `SELECT COUNT(*) FROM reminder_occurrences WHERE status='DISPATCHING';`
  - `… GROUP BY occurrence_id HAVING MAX(a.attempt_no) > o.attempt_count`
- **Rollout:** worker. Rollback an toàn.
- **Verify:** `go test -race ./internal/reminder/... -count=1`.
- **Phạm vi ảnh hưởng:** đường email ở `notification/app` có cùng kiểu (OBX-03, để PR-W3).
- **Ước lượng:** S–M

### REM-04 Lỗi evaluator retry vô hạn
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** `handleEvaluatorError` (`service.go:585-618`) luôn gọi `failRetryable`, không có giới hạn (chỉ đường SMTP có, `:635`). Mỗi occurrence ghi khoảng 72 attempt/ngày với mã sai `EMAIL_PROVIDER_TEMPORARY_ERROR`. **Confirmed.**
- **Test tái hiện (unit):** `TestDispatchDue_EvaluatorErrorIsBounded`:
  - trễ 49h → FAILED `REMINDER_EXPIRED`;
  - `CompanyID==""` → FAILED `REMINDER_SOURCE_MISSING`.
  - HEAD → **FAIL**.
- **Thiết kế sửa (phương án B, khuyến nghị):**
  - Không claim, không ghi attempt.
  - Thiếu company/nguồn → Expire.
  - Trễ > 48h → Expire.
  - Còn lại → Defer +15 phút `REMINDER_EVALUATOR_ERROR`.
  - Phương án A (FAILED ở attempt thứ 5, khoảng 20 phút) sẽ làm mất reminder khi DB gián đoạn ngắn.
- **Contract / dữ liệu:** không. Kiểm `error_code='EMAIL_PROVIDER_TEMPORARY_ERROR' GROUP BY occurrence HAVING COUNT(*) > 5` → 0.
- **Rollout:** worker. Rollback an toàn.
- **Verify:** `go test ./internal/reminder/app/ -run Evaluator -count=1`.
- **Phụ thuộc:** REM-05.
- **Ước lượng:** S

### RMC-06 API reminder của workflow báo "sent" khi mới seed
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** `MarkMilestoneSent` chạy ngay lúc seed (`service.go:1021`), và `ListByInstance` ánh xạ `reminder_sent` thành "sent" (`workflow/infra/mysql/milestones.go:53-77`). **Confirmed.**
- **Test tái hiện:**
  - Unit `internal/workflow/infra/mysql/milestones_status_test.go` `TestReminderStatusFromRow`:
    - `(0)` → pending
    - `(1, occurrence NULL)` → cancelled
    - `(1, PENDING/RETRY/DISPATCHING)` → pending
    - `(1, SENT)` → sent
    - `(1, FAILED REMINDER_OBSOLETE/EXPIRED)` → cancelled
    - `(1, FAILED khác)` → failed
  - mysql-integration `TestMySQLListByInstance_StatusFromOccurrence`.
- **Thiết kế sửa:**
  ```sql
  SELECT m.milestone_id, m.workflow_instance_id, m.step_id, m.step_order, m.milestone_type,
         m.scheduled_date, m.reminder_sent, m.reminder_occurrence_id,
         o.status, o.last_error_code, o.last_attempt_at
  FROM workflow_step_milestones m
  LEFT JOIN reminder_occurrences o ON o.occurrence_id = m.reminder_occurrence_id
  WHERE m.company_id = ? AND m.workflow_instance_id = ?
  ORDER BY m.step_order, m.milestone_type
  ```
  - Scan `reminder_sent` vào `int` (chịu được cả giá trị 2, API-35).
  - Thêm `sent_at` (omitempty) khi SENT.
- **Contract / FE:** status vẫn nằm trong union của web `'pending'|'sent'|'failed'|'cancelled'|'dlq'`:
  - khai báo ở `cobo_web_design/src/types.ts:255`;
  - nhãn ở `DisclosureDetail.tsx:68-82`;
  - normalizer `workflowOverrideMappers.ts:306-322` đã đọc `sent_at`.
  - Không cần sửa FE. Mục contract doc là tuỳ chọn (API-42).
- **Rollout:** API. Rollback thì lại hiện "sent".
- **Verify:** DEV gọi `GET /api/v1/workflows/instances/{id}/reminders` bằng persona QA: milestone obsolete phải hiện `"cancelled"`.
- **Ước lượng:** S

### RMC-03 Template mặc định sai cho WORKFLOW_STEP
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** `repository.go:486` (và in-memory `:252`) gắn cứng `REMINDER_DISCLOSURE_DUE`. Khi không có row CMS, layer 2 giữ nguyên mã này (`dispatch_decision.go:232-269`), trong khi mặc định của CMS là `reminder.workflow_step_due` (`platformcms/app/alert_config_service.go:18`). **Confirmed.**
- **Test tái hiện (unit):** `TestPrepareDispatch_WorkflowStepDefaultTemplate` → kỳ vọng `reminder.workflow_step_due`. HEAD → **FAIL**.
- **Thiết kế sửa:**
  - Trong `prepareDispatch`: scope `WORKFLOW_STEP` mà mã là `REMINDER_DISCLOSURE_DUE` thì chuyển sang `DefaultWorkflowStepTemplateCode`.
  - Thêm case `reminder.workflow_step_due` vào renderer legacy (`infra/email/smtp_sender.go:202-251`), dùng lại text của disclosure. Legacy là mặc định của code (`config.go:267`); thiếu case này thì email FAIL vĩnh viễn.
  - Không đổi SQL của repo.
- **Contract:** chỉ nội dung email. Artifact dùng embed (`docker-compose.artifacts.yml:140,205`).
- **Verify:** `go test ./internal/reminder/... -count=1`; xem trên Mailpit ở local.
- **Ước lượng:** S

### RMC-04 Bước bị trễ nhưng reminder không dời theo
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** độ trễ chỉ được cộng ở màn hình hạn (`deadlinealerts/app/steps_algorithm.go:291-314`). Milestone chỉ được tạo một lần (`workflow/app/service.go:143-166`), và hạn lúc dispatch lấy từ `wsm_end` gốc (`reminder/infra/mysql/repository.go:416,438-441`). **Confirmed.**
- **Test tái hiện (unit):**
  - `TestDecideWorkflowStepReminder_DelayShiftWaits`: bước trước trễ 3 ngày → Wait cho tới `StepEnd − N`.
  - `TestDispatchDue_UsesShiftedStepEnd`: `due_date` trong payload là ngày đã dời.
- **Thiết kế sửa:**
  - Lúc seed: `ScheduledAt = max(m.ScheduledDate, StepEnd − N)`.
  - Lúc dispatch: `c.DeadlineAt = StepEnd`, `step_due_date` = StepEnd.
  - Không ghi lại milestone, không có job tính lại.
- **Rollout / verify:** đi cùng RMC-02. DEV: `SELECT COUNT(*) FROM workflow_instance_step_states WHERE delay_days_applied > 0;`
- **Phụ thuộc:** RMC-01 (A).
- **Ước lượng:** S

### REM-10 In-app không có khoá chống trùng
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** id sinh ngẫu nhiên bằng `newID()` (`inappnotification/app/service.go:53,108-110`) và INSERT thường (`infra/mysql/repository.go:21-33`). Mỗi lần gửi lại tạo thêm R dòng. **Confirmed.**
- **Test tái hiện (unit):** `internal/reminder/infra/inapp/bridge_test.go` `TestBridge_SameOccurrenceTwiceCreatesOneRowPerUser`, dùng `inappapp.NewService` thật và fake repo có kiểm PK. HEAD → **FAIL**.
- **Thiết kế sửa:**
  - Thêm `ReminderInAppRequest.DedupeKey`; bridge đặt bằng `c.OccurrenceID`.
  - id = `"uin_"+hex(sha256(key+"|"+userID))[:32]`, dài 36 ký tự, vừa `VARCHAR(64)` (migration `0085`).
  - MySQL `Create` dùng `ON DUPLICATE KEY UPDATE id = id`. Không dùng `INSERT IGNORE`, vì nó che lỗi cắt chuỗi của strict mode (AHA-04).
  - Repo in-memory làm tương tự.
- **Contract:** không. id trở nên đoán được, nhưng `MarkRead` lọc theo `user_id` (be-security cần xác nhận).
- **Dữ liệu:** in-app trùng đã hết từ sau 16/09; không cần sửa.
- **Verify:** `go test -race ./internal/reminder/infra/inapp/ ./internal/inappnotification/... -count=1`.
- **Ước lượng:** S

### RMC-09 "Hướng dẫn thực hiện" luôn là text mặc định
- **Mức độ / nguồn:** LOW.
- **Nguyên nhân gốc:**
  - `StepSnapshot` không có trường `instructions` (`workflow/app/contracts.go:119-138`); `MapEffectiveWorkflowToSnapshot` (`snapshot.go:97-108`) bỏ trường này.
  - Reader instance trả bước có instructions rỗng (`instance_step_reader.go:30,74`), nên không fallback sang reader global (`service.go:460-469`) và dùng text mặc định (`:533-535`).
  - **Confirmed.**
- **Test tái hiện (unit):**
  - `TestMapEffectiveWorkflowToSnapshot_KeepsInstructions`.
  - `TestPrepareDispatch_FallsBackToGlobalInstructionsWhenSnapshotEmpty`. HEAD → **FAIL**.
- **Thiết kế sửa:**
  - Thêm `Instructions string \`json:"instructions,omitempty"\``.
  - Copy trường này trong `MapEffectiveWorkflowToSnapshot`.
  - Khi tìm thấy bước nhưng instructions rỗng thì lấy từ `stepReader.GetStepByID`.
  - Không backfill.
- **Contract:** `snapshot_json` có thêm một key tuỳ chọn (chỉ instance mới). BES-30: `body.html` render bằng `text/template`, nên instructions do người dùng nhập không được escape. Báo be-security; sửa ở PR-W3.
- **Ước lượng:** S

---

## Thứ tự commit (mỗi commit gồm test tái hiện + bản sửa; GATE R chứng minh bằng cách chạy test mới trên commit cha)
1. `test(reminder)`: fake PERF-47 (repo occurrence có 2 unique key; scanner tôn trọng limit/thứ tự/cursor). Không đổi code production.
2. `fix(reminder)`: REM-01 `EnsureOccurrence` (repo, in-memory, bridge) + guard + integration test.
3. `fix(workflow,disclosure,adhoc)`: REM-01 validate định danh bước không trùng ở cả 4 chỗ.
4. `feat(reminder)`: (A)+(B), RMC-01 điều kiện nhắc, REM-06 obsolete + phân trang, RMC-04 ngày seed.
5. `fix(workflow)`: RMC-06 ánh xạ status. **Commit 1–5 = PR-1a**, hotfix phải deploy trước 2026-10-12.
6. `fix(reminder)`: REM-05 defer/expire + loại trừ trong collector; REM-04.
7. `fix(reminder)`: RMC-02 khung giờ gửi + supersede + kiểm lại lúc dispatch; RMC-04 hạn trong payload; template `remaining_text`.
8. `fix(reminder)`: REM-03 `RecordDispatchOutcome`.
9. `fix(reminder)`: RMC-03 template mặc định + case renderer legacy.
10. `fix(inapp)`: REM-10 id tất định.
11. `fix(workflow,reminder)`: RMC-09 instructions.
12. `docs(ai-cache)`: thư mục kết quả + `reusable-task-updates.md`.

## Reviewer (wf-bugfix Phase 6, mở song song trong một message)
- **perf-reliability:** commit 2, 4, 6, 7, 8. Kiểm guard SQL, chi phí phân trang, transaction, `WithoutCancel`, trường hợp ≥2 worker, race khi đánh dấu obsolete.
- **api-compat:** commit 3, 5, 7, 9. Kiểm phản hồi 400/422 mới ở override/CMS/ad-hoc, giá trị status reminder (union của web), biến template, `DispatchDueResult`.
- **be-security:** commit 4, 10, 11. Kiểm lọc `company_id` trong reader mới, id in-app đoán được so với phạm vi `MarkRead`, instructions chảy vào HTML `text/template` (BES-30). Việc chọn người nhận không đổi.
- Sau cùng: `premerge-system-review` bản rút gọn.

## Checklist verify tổng hợp
- `gofmt -l ./internal ./cmd` → rỗng. `go vet ./...` và `go build ./...` pass.
- `go test ./internal/reminder/... ./internal/workflow/... ./internal/inappnotification/... ./internal/adhoc/app/ ./internal/deadlinealerts/... -count=1`
- `go test -race ./internal/reminder/... ./internal/inappnotification/... -count=1`
- `go test ./internal/disclosure/app/ ./internal/notification/app/ -count=1`: chỉ được phép còn đúng các fail có sẵn (6 test template-import, `TestContract_VariableParity/workflow.approved`); `…/reminder.workflow_step_due` phải pass.
- `MYSQL_TEST_DSN=…test… go test ./internal/reminder/infra/mysql/ ./internal/workflow/infra/mysql/ -run MySQL -count=1`. Không có DSN thì ghi `BLOCKED: no MYSQL_TEST_DSN`.
- `docker compose -f docker-compose.dev.yml build api worker`.
- **DEV trước khi deploy** (`qa_ro`): các query kiểm tra của REM-01/05/03 trả 0; ghi lại số milestone chết của REM-06.
- **Deploy** bằng `deploy-be`, ban đầu với `REMINDER_DISPATCH_ENABLED=false` (recreate worker). Sau 2 tick:
  - số milestone chết < khoảng 40;
  - số dòng obsolete xấp xỉ số dòng chết trước đó;
  - không còn dòng `PENDING`/`RETRY_SCHEDULED` cũ hơn 48h;
  - rồi mới bật `REMINDER_DISPATCH_ENABLED=true`.
- **24h sau deploy:**
  - giờ gửi chỉ nằm trong 7..20 (giờ VN);
  - không occurrence nào có >1 attempt SENT hoặc >5 attempt;
  - không còn `DISPATCHING` cũ hơn 5 phút;
  - API reminder hiển thị đúng `cancelled`/`pending`/`sent`;
  - `cobo_reminder_failed_recent` không bị kích hoạt bởi `REMINDER_EXPIRED`/`REMINDER_OBSOLETE`.
- **Sàn rollback:** mọi binary ≥ `a8acb51` tương thích dữ liệu với PR-1. Worker không bao giờ được thấp hơn `cabbecf` (PER-01).

**Ngoài phạm vi PR-1:** REM-02, REM-07, REM-08, RMC-05, RMC-07/08, SEC-27. Lưu ý SEC-27: nên bật allowlist email trên DEV **trước khi** bước 2..N làm tăng lượng email.
