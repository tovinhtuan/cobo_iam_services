# Audit worker và alert: trùng lặp, spam, bản ghi rác (2026-10-10)

> **Đính chính (risk review lần 5, `../risk-review-2026-10-10-3/10-risk-report.md`):**
> - REM-01: ad-hoc proposal **không** sinh được step ID trùng, vì `NormalizeProposalWorkflowSteps` lọc trùng. Đường kích hoạt thật là override workflow của company: `ValidateCompanyWorkflowOverrideSteps` và `ValidateSnapshot` chỉ kiểm rỗng.
> - REM-06: **không** dùng `reminder_sent=2`, vì `ListByInstance` scan cột này vào `bool` nên API sẽ trả 500. Dùng `reminder_sent=1` + `reminder_occurrence_id=NULL`, và chỉ đánh dấu khi milestone đã hết ý nghĩa.
> - Mức độ: REM-03, REM-04, OBX-01, OBX-02 hạ xuống MEDIUM; PER-06 nâng lên HIGH (hạn 2026-12-01).

- Task type: phân tích read-only. Không sửa code, không ghi DB, không deploy.
- Skill chain: `cobo-task-workflow` → `backend-worker-idempotent` → `backend-performance-reliability` → `deadline-reminder-holiday` + `notification-delivery` (+ `observability-metrics`).
- Phạm vi:
  - iam HEAD `a8acb51`.
  - 3 agent đọc code song song: reminder, periodic seeding, outbox/notification.
  - Main agent kiểm chứng mọi mục HIGH bằng code.
  - Main agent query DEV chỉ bằng `qa_ro` (SELECT). Script tự dừng nếu thiếu `QA_DB_USER`, để không rơi về root.
  - Main agent dùng `ls` qua SSH để xem binary worker trên DEV.

## Trả lời ngắn

1. **Sự cố 4 triệu bản ghi đã rõ nguyên nhân và đã được sửa ở `cabbecf` (2026-10-03).**
   - Nguyên nhân: job materialize kỳ định kỳ insert `disclosure_records` (ID ngẫu nhiên) **trước** khi tạo workflow. Workflow lỗi thì claim được nhả và ID bị bỏ, nên mỗi tick 5 giây lại có một Draft mới. Mỗi cycle lỗi tạo 17.280 Draft/ngày.
   - Bản sửa: record ID tất định theo `cycle_id`, state machine PENDING/CLAIMED/RETRY/FAILED/COMPLETED (migration 0151), instance và task đầu tiên chung một transaction.
   - Dấu vết còn lại trên DEV:
     - `disclosure_records` còn 330 dòng nhưng có khoảng **2,7 GB `data_free`** (đã insert rồi xoá hàng loạt).
     - `reminder_occurrences` có `AUTO_INCREMENT` ≈ **1,42 triệu** nhưng chỉ 241 dòng. Id đã bị đốt trước 2026-09-20, do `INSERT IGNORE`/upsert chạy lại mỗi tick (REM-08). Đây không phải gửi lặp: `reminder_delivery_attempts` mới tới id 683.
2. **Hiện tại không có vòng lặp nào đang tạo bản ghi trên DEV.** Đã đo:
   - `AUTO_INCREMENT` đứng yên trong 2 phút;
   - không có cycle hay milestone trùng;
   - mỗi occurrence chỉ có 1 attempt;
   - outbox không có dòng pending/processing/failed;
   - in-app trùng sát nhau chỉ xảy ra từ 05/09 đến 16/09, sau đó hết.
3. **Code vẫn còn nhiều đường có thể tái tạo kiểu sự cố này hoặc gây spam email.** Nguy hiểm nhất:
   - **PER-01:** trên DEV vẫn còn binary worker cũ có lỗi.
   - **REM-01:** upsert seed đặt lại SENT → PENDING mỗi tick.
   - **REM-06:** hàng đợi milestone sắp bị chặn trong khoảng 4-5 ngày.
   - **DATA-01:** template QA chu kỳ hằng ngày sinh dữ liệu và email thật mỗi ngày.

## Bằng chứng DEV (read-only, 2026-10-10 ~15:00 UTC)

| Đo | Kết quả |
|---|---|
| Bảng lớn nhất | `audit_logs` ~9,2k; `workflow_step_milestones` ~7k; `outbox_events` ~6,4k; `login_attempts` ~4,2k; `sessions` ~4k. Không bảng nào gần 4 triệu |
| `disclosure_records` | 330 dòng, `data_free` 2.743 MB, bảng được rebuild lúc 2026-09-24 07:30 |
| `reminder_occurrences` | 241 dòng (đều SENT), `AUTO_INCREMENT` 1.416.779, không đổi sau 2 phút; id liên tục từ 2026-09-20 |
| `periodic_cycles` | 318 dòng: COMPLETED 308, PENDING 10. Không trùng `(type, company, label)`, không có RETRY/FAILED/CLAIMED |
| Template QA chu kỳ hằng ngày | 6 type × 4 company: **24 cycle + 24 record + 24 workflow instance + ~548 milestone mỗi ngày**, cộng **20 email reminder thật/ngày** (DEV gửi qua Gmail SMTP) |
| Milestone `due_minus_%` chết (`reminder_sent=0`, đã qua ngày) | **136**, tăng **108 trong 7 ngày gần nhất** |
| `workflow_instances` trùng `(company, record)` | 1 nhóm (2 instance cùng giây, `m_102`, 2026-09-20): double-submit, H13 |
| `user_in_app_notifications` | 698 dòng; 320 dòng `reminder.workflow_step_due` có `resource_id` NULL. Cặp trùng nội dung trong 60 giây: ~20/ngày từ 05/09 đến 16/09, sau đó 0 |
| `outbox_events` | 6.399 dòng, tất cả `processed`, `retry_count` 0. Không có cleanup |
| `sessions` | 3.978 dòng, 1.131 còn hiệu lực, 2.822 đã hết hạn nhưng không bị xoá |
| Binary worker trên DEV | **Còn ít nhất 6 binary có từ trước `cabbecf`**: `worker.rollback.20261002T180641Z`, `worker.rollback.20261002T180340Z`, `worker.bak-phase127`, `worker.bak.20260602`, `worker.bak.b2b_20260609_1449`, `worker.bak.deploy_20260610_152045`. `.env` đang bật `PERIODIC_SEEDING_ENABLED=true` |

## Findings

### HIGH

#### PER-01 Rollback worker về binary trước `cabbecf` sẽ chạy lại vòng lặp 4 triệu Draft
- **Confirmed:** main agent đã đọc code và `ls` trên DEV.
- Code:
  - Binary cũ chọn `WHERE pc.record_id IS NULL AND pc.materialized_at IS NULL` (`git show cabbecf^:internal/disclosure/infra/mysql/repository.go:2455`).
  - Code mới set `materialized_at = NULL` khi chuyển sang FAILED (`internal/disclosure/infra/mysql/repository.go:2559`).
  - Hệ quả: cycle FAILED, vốn là cycle chắc chắn lỗi, lại hợp lệ với binary cũ. Mỗi tick, mỗi cycle sinh một Draft ngẫu nhiên. Cycle PENDING mới cũng bị binary cũ xử lý với lỗi cũ.
- Điều kiện: có người chạy một trong các file `worker.rollback.20261002T*` hoặc `worker.bak*` (tên file gây hiểu nhầm là "an toàn để rollback").
- Fix:
  - Xoá hoặc đổi tên các binary worker cũ hơn `cabbecf` trên DEV.
  - Ghi vào runbook: "không rollback worker qua `cabbecf` khi `PERIODIC_SEEDING_ENABLED=true`".
  - Đổi `MarkPeriodicCycleFailed` thành `materialized_at = NOW(3)`.

#### REM-01 Upsert seed đặt lại occurrence đã SENT về PENDING mỗi tick khi milestone bị trùng
- **Confirmed** (code), main agent đã đọc `internal/reminder/infra/mysql/repository.go:286-311` và `internal/reminder/app/service.go:1006-1020`.
- Chuỗi lỗi:
  1. Hai milestone m1, m2 có cùng `(instance, step, due_minus_Nd)`.
  2. Khi seed m2, `idempotency_key` trùng nên `ON DUPLICATE KEY UPDATE ... status = VALUES(status)` đặt **row của m1 về PENDING**.
  3. `SELECT ... WHERE occurrence_id = m2` không thấy dòng nào, nên báo lỗi và `continue`. m2 không được đánh dấu.
  4. Tick sau lặp lại y hệt.
- Hệ quả mỗi tick, với mỗi key: 1 email, 1 dòng attempt, R dòng in-app. Tức 17.280 email/ngày/key, kéo dài tới khi step xong.
- Điều kiện kích hoạt: **confirmed có đường vào.**
  - `adhoc/app/runtime_materialize.go:40-60` chỉ chặn `order` trùng, **không chặn `step.ID` trùng**.
  - Gọi API với 2 step cùng ID là sinh được milestone trùng (bảng chỉ unique trên `milestone_id` ngẫu nhiên).
  - DEV hiện có 0 nhóm trùng.
- Fix:
  - Upsert không ghi đè `status`/`scheduled_at` (`ON DUPLICATE KEY UPDATE updated_at = updated_at`).
  - Đọc lại theo `idempotency_key` và đánh dấu milestone theo occurrence tìm được.
  - Validate `step.ID` duy nhất.
  - Migration dedupe + UNIQUE `(workflow_instance_id, step_id, milestone_type)`.

#### REM-06 Milestone không đủ điều kiện không bao giờ được đánh dấu, nên hàng đợi seed sắp bị chặn
- **Confirmed** (code + DEV).
- Scanner lấy `reminder_sent=0 AND scheduled_date<=today AND milestone_type LIKE 'due_minus_%' ORDER BY scheduled_date LIMIT 200` (`internal/reminder/infra/mysql/milestone_scanner.go:25-33`).
- Milestone mà step đã xong hoặc không còn là step hiện tại sẽ bị bỏ qua (`service.go:1002-1004`) nhưng không được đánh dấu.
- DEV: **136 dòng chết, +108 trong 7 ngày (~15/ngày)**. Sau khoảng 4-5 ngày (~14-15/10), cả batch 200 toàn dòng chết, nên **reminder bước workflow mới ngừng hẳn mà không có lỗi nào**.
- Mỗi tick còn tốn ~136-200 query `StepTaskState`, tức ~2-3,5 triệu query/ngày.
- Fix: đánh dấu milestone là đã xử lý (giá trị riêng, ví dụ `reminder_sent=2`) khi step đã xong; index `(reminder_sent, milestone_type, scheduled_date)`.

#### REM-03 Lệch attempt_count tạo vòng gửi lại mỗi 5 phút
- **Confirmed** (code; `service.go:621-657`).
- Thứ tự là gửi SMTP → `InsertAttempt` → `UpdateDispatchResult`, không trong transaction.
- Nếu bước cuối lỗi (SIGTERM khi deploy, deadlock): row kẹt DISPATCHING, rồi reaper đưa về PENDING. Lần sau gửi lại, `InsertAttempt` đụng unique `(occurrence_id, attempt_no)`, row lại kẹt. Kết quả: 288 email/ngày cho mỗi occurrence bị ảnh hưởng.
- Fix: attempt + status trong một transaction có guard `WHERE status='DISPATCHING'`; attempt insert idempotent.

#### REM-04 Lỗi evaluator/resolver retry vô hạn
- **Confirmed** (code; `service.go:599-608`).
- Đường evaluator gọi `failRetryable` mà không có giới hạn; giới hạn `attemptNo < 5` chỉ áp cho lỗi SMTP (`:635`).
- Occurrence mồ côi hoặc snapshot hỏng: khoảng 72 dòng attempt/ngày, mãi mãi.
- Fix: FAILED khi `attemptNo >= 5`; company trống hoặc không còn nguồn thì FAILED luôn.

#### REM-05 Occurrence bị skip không bao giờ có trạng thái kết thúc
- **Confirmed** (code; `service.go:346-357`).
- `skip` để nguyên PENDING; query dispatch `ORDER BY scheduled_at LIMIT 50`, không có cận dưới.
- Hệ quả:
  - Khi có ≥50 dòng skip vĩnh viễn, hàng đợi bị chặn và log skip chạy mỗi tick.
  - Khi điều kiện skip hết, backlog cũ được gửi dồn một lượt, kể cả cho disclosure đã nộp hoặc config đã tắt.
- Fix: trạng thái SKIPPED/EXPIRED; bỏ qua `scheduled_at` quá cũ (> 2 ngày).

#### OBX-01 (= H9) Reaper outbox dùng `available_at` làm lease, nên event chạy 2 lần
- **Confirmed** (code; `platform/outbox/mysql/repository.go:90-93,189-193`).
- Event đã quá hạn hơn 5 phút (backlog, worker vừa sống lại) có thể bị requeue ngay khi đang xử lý. SIGTERM giữa batch cũng gây requeue.
- `auth.*` không chống trùng, nên user nhận email reset/mời 2 lần.
- DEV chạy 1 worker, nên thực tế chỉ khoảng 1 email trùng mỗi lần deploy trùng lúc đang gửi.
- Fix: claim set `available_at = NOW()` (hoặc thêm `claimed_at`); `Mark*` có `AND status='processing'`.

#### OBX-02 (= H8) SMTP không timeout ở 3/4 sender
- **Confirmed** (code): `cmd/worker/main.go:392`, `notification/infra/smtp/adapter.go:42`, `binding_mailer.go:35`.
- SMTP treo thì cả goroutine tick treo, và reminder, outbox, periodic dừng hết. Gây ngừng dịch vụ, không phải spam.
- Fix: dùng `boundedSendMail` của `reminder/infra/email/smtp_sender.go:114-132`.

#### REM-09 (= H1, rộng hơn) Config reminder không kiểm tenant và không giới hạn kích thước
- **Confirmed** (code; `reminder/transport/http/handler.go:39-61`, `reminder/app/service.go:177-198`).
- Bất kỳ token hợp lệ nào cũng ghi được config cho disclosure bất kỳ, với `daysBefore` hàng nghìn phần tử và email bên ngoài. Kết quả là hàng nghìn occurrence và email: một đường spam relay.
- Fix:
  - kiểm company + quyền;
  - giới hạn mảng (≤10, giá trị 0..365);
  - người nhận chỉ là email của member active.

### MEDIUM
- **DATA-01 Template QA chu kỳ hằng ngày sinh dữ liệu và email thật mỗi ngày.** Confirmed qua DEV.
  - 6 type QA (`qa-def001-…`, `qa-def006-…`, `qa-final-focused-…`, `qa-final-mail-…`, `qa-final-mail2-…`, `qa-resmoke-periodic-20260904a`) × 4 company.
  - Mỗi ngày tạo ~620 dòng (cycle, record, instance, task, milestone) và 20 email gửi tới hộp thư thật.
  - Chu kỳ `daily` được validate cho phép (`template_validation.go:19,73`), không có giới hạn hay retention.
  - Đây cũng là nguồn chính của REM-06.
  - Fix: archive/tắt periodic cho các type QA này (cần quyết định); tuỳ chọn: không cho `daily` trên môi trường thật.
- **PER-02 Template periodic phạm vi company bị seed cycle cho mọi company** (`disclosure/app/periodic.go:79-80`). Kéo theo cycle "ma" retry mãi và lộ tên template sang tenant khác qua alert. DEV: 0 dòng. Fix: bỏ qua khi `owner_company_id != company_id`.
- **PER-03** Cycle RETRY không có giới hạn: 60 giây/lần mãi mãi; lỗi gate 4xx không chuyển FAILED (`repository.go:2538`).
- **PER-04** CLAIMED không có lease; SIGTERM giữa claim và complete làm cycle kẹt vĩnh viễn (`periodic.go:256-305`).
- **PER-05** Lỗi insert milestone bị nuốt (`workflow/app/service.go:159-162`), nên instance không có milestone và không bao giờ nhắc. DEV có 1 instance của worker không có milestone.
- **PER-06 (= CACHE-08)** Worker không có holiday provider như API; `configs/non_trading_days/` chỉ có `2026.json`, nên cycle ngày làm việc sang 2027 bị bỏ im lặng.
- **REM-02** `FOR UPDATE SKIP LOCKED` chạy ngoài transaction nên vô tác dụng (`milestone_scanner.go:25-33`). Chỉ nguy hiểm khi có ≥2 worker; DEV hiện chạy 1.
- **REM-07 (= H11)** Key có `recipient_hash` và không có cận dưới thời gian, nên mỗi lần sửa người nhận sinh K occurrence mới và gửi ngay các mốc đã qua.
- **REM-08** `INSERT IGNORE ... SELECT` chạy lại mỗi tick và đốt `AUTO_INCREMENT` (đây là nguồn 1,42 triệu id); cộng thêm việc recompute `JSON_TABLE` mỗi 5 giây. Fix: anti-join `NOT EXISTS` + cận dưới.
- **REM-10 / OBX-05** In-app không có khoá chống trùng (`migrations/0085`, PK là UUID). Mỗi lần gửi lại tạo thêm R dòng. Adhoc vẫn notify khi replay (`applied=false`). Dòng reminder cũ thiếu `resource_id`.
- **OBX-03** `email.dispatch` có thể gửi 2 lần: `MarkSending` không có điều kiện, attempt được ghi sau khi gửi.
- **OBX-04** Không có retention cho `outbox_events` (payload còn chứa link reset/OTP), `audit_logs`, `login_attempts`, `sessions`, `idempotency_keys`, `user_in_app_notifications`.

### LOW
- PER-07 (one-shot tool vẫn dùng ID ngẫu nhiên), PER-08 (seed no-op + log Info mỗi tick).
- REM-11 (lỗi QUIT sau DATA bị coi là tạm thời, nên gửi lại).
- OBX-06 (goroutine dùng request ctx, mất in-app), OBX-07 (lỗi `Mark*` bị nuốt; cắt `last_error` theo byte), OBX-08 (phạm vi dedupe key adhoc/workflow sai), OBX-09 (WARN mỗi tick; log email người nhận).

## Đã kiểm, không có vấn đề
- Key của `periodic_cycles` cố định theo slot. Claim cycle là một UPDATE có điều kiện, chỉ một bên thắng. Đường worker luôn dùng ID tất định. Retry không tạo instance/task/milestone con trùng.
- Không handler outbox nào phát thêm event, nên không có vòng phản hồi. Outbox retry tối đa 10; email attempt tối đa 5.
- `ClaimForDispatch` dùng `FOR UPDATE` + kiểm status. Binding tick claim nguyên tử và mặc định tắt.
- Không có `time.Now` hay giá trị ngẫu nhiên nào trong key của occurrence.
- Đường đọc của deadline-alerts không ghi gì.

## SQL giám sát (read-only, nên chạy định kỳ hoặc gắn alert)
```sql
-- Bản ghi tăng bất thường theo ngày
SELECT DATE(created_at) d, created_by, COUNT(*) FROM disclosure_records GROUP BY 1,2 ORDER BY d DESC LIMIT 14;
-- Id bị đốt so với số dòng thật
SET SESSION information_schema_stats_expiry=0;
SELECT table_name, table_rows, auto_increment FROM information_schema.tables WHERE table_schema='cobo_iam' AND auto_increment > 10*GREATEST(table_rows,1000);
-- REM-01 trigger và vòng lặp đang chạy
SELECT workflow_instance_id, step_id, milestone_type, COUNT(*) c FROM workflow_step_milestones WHERE milestone_type LIKE 'due_minus_%' GROUP BY 1,2,3 HAVING c>1;
SELECT occurrence_id, COUNT(*) n, SUM(status='SENT') sent FROM reminder_delivery_attempts GROUP BY 1 HAVING sent>1 OR n>5;
-- REM-06 sắp chặn hàng đợi (báo động khi >= 150)
SELECT COUNT(*) FROM workflow_step_milestones WHERE reminder_sent=0 AND scheduled_date<UTC_DATE() AND milestone_type LIKE 'due_minus_%';
-- Periodic kẹt/retry
SELECT materialization_state, COUNT(*), MAX(attempt_count) FROM periodic_cycles GROUP BY 1;
-- Outbox tồn đọng
SELECT status, COUNT(*), MIN(available_at) FROM outbox_events WHERE status <> 'processed' GROUP BY 1;
```

## Kế hoạch sửa đề xuất (chưa làm; chờ user yêu cầu)
1. **Ngay, hạ tầng, cần user cho phép:**
   - PER-01: xoá hoặc đổi tên 6 binary worker cũ hơn `cabbecf` trên DEV; thêm cảnh báo vào runbook rollback.
   - DATA-01: quyết định tắt/archive 6 type QA chu kỳ hằng ngày.
2. **iam PR-W1 (reminder, ưu tiên 1):**
   - REM-06 (đánh dấu milestone chết + index);
   - REM-01 (upsert không reset status + validate `step.ID` + unique milestone);
   - REM-03, REM-04, REM-05;
   - REM-10 (dedupe key in-app);
   - test chạy lặp, crash sau side effect, 2 worker.
3. **iam PR-W2 (periodic):** PER-01 (`MarkPeriodicCycleFailed`), PER-03 (cap + phân loại 4xx), PER-04 (reaper CLAIMED + `WithoutCancel`), PER-05, PER-02, PER-06/CACHE-08.
4. **iam PR-W3 (outbox/SMTP):** OBX-02/H8 (timeout), OBX-01/H9 (lease), OBX-03, OBX-07; metric `cobo_outbox_pending`, `cobo_reminder_dead_milestones`.
5. **iam PR-W4 (retention):** OBX-04, job dọn theo lô có `LIMIT` cho outbox đã xử lý, session hết hạn, idempotency hết hạn, in-app cũ.
6. **Bảo mật (đã có trong risk review):** REM-09/H1.

## Not checked / BLOCKED
- Không chạy được integration test MySQL (không có `MYSQL_TEST_DSN`). Chưa tái hiện REM-01/03 trên DB thật; chỉ có bằng chứng code + dữ liệu DEV.
- Chưa đọc log worker trên DEV, nên chưa đo được tần suất WARN mỗi tick.
- Nguồn của ~20 cặp in-app trùng nội dung mỗi ngày (05/09–16/09) chưa xác định chắc (`resource_id` NULL nên không truy được occurrence). Đã hết từ 17/09.
- Theo ghi chú `cabbecf`, sự cố 4 triệu thuộc `disclosure_records` (khớp với `data_free` ~2,7 GB). Không có log hay số đếm gốc để đối chiếu đúng con số 4 triệu.
