# Risk review 2026-10-10 (lần 5) - scope: rủi ro worker/alert (iam `a8acb51`)

Chỉ đọc, không sửa code, không ghi DB, không thao tác server. Phạm vi xem `00-scope.md`. Đầu vào là `../worker-alert-spam-audit-2026-10-10/00-report.md`.

## Cách chạy
- 4 reviewer chạy song song: perf-reliability, be-security, api-compat, secrets-cors. Mỗi reviewer chạy dưới dạng agent `general-purpose` với file `.claude/agents/<name>.md`.
- be-security lần 1 bị bộ lọc an toàn dừng giữa chừng (đang viết kịch bản khai thác) và đã được chạy lại theo hướng chỉ nêu cách sửa.
- Main agent tự đọc code mọi mục HIGH. Ngoài ra main agent đo thêm trên DEV bằng `qa_ro` (SELECT) và `ls` qua SSH:
  - MySQL 8.0.46, strict mode;
  - 0 milestone trùng hoặc có type rỗng;
  - có 7 binary `api.*` cũ hơn 2026-07-17.
- Không chọn fe-security, admin-role, cache-versioning vì ngoài phạm vi. Hai nhóm Security Frontend và Cache & Versioning ghi "Not checked".

Bảng đếm mọi finding trong phạm vi này sau khi gộp trùng, bao gồm cả mục cũ được kiểm lại như H1, H8, H9, BES-30.

| Group | Status | Critical | High | Medium | Low |
|---|---|---|---|---|---|
| Security Frontend | Not checked (ngoài phạm vi) | - | - | - | - |
| Security Backend | Issues | 0 | 1 | 4 | 3 |
| Backend Performance & Reliability | Issues | 0 | 5 | 16 | 3 |
| Cache & Versioning | Not checked (ngoài phạm vi) | - | - | - | - |
| API Compatibility | Issues | 0 | 1 | 6 | 1 |
| CORS & Credentials | Issues | 0 | 2 | 0 | 1 |

Gộp trùng:
- **REM-01** = PERF-38 + BES-37 (đường kích hoạt) + API-36 (migration) + PERF-47 (test fake).
- **REM-06** = PERF-40 + API-35 (cách sửa phá API).
- **REM-05** = PERF-39 + API-37 (ENUM).
- **PER-01** = PERF-41 + API-33 + SEC-25 + API-38 (thứ tự sửa).
- **PER-06** = PERF-42.
- **H1** = REM-09 = BES-36.
- **H19** = API-34.
- **OBX-04** = SEC-29.
- **OBX-09** = BES-40 = SEC-30 (+ BES-34).
- **PER-07** = PERF-46 = API-39.
- **PER-02** = BES-38.
- **OBX-05/06/08** = BES-41.

## Đính chính so với báo cáo audit
- **REM-01, đường kích hoạt.**
  - Ad-hoc proposal **không** sinh được step ID trùng qua API: `NormalizeProposalWorkflowSteps` cấp lại và lọc trùng ID (`internal/adhoc/app/workflow_v2.go:145`).
  - Đường có thật là **override workflow của company**: `ValidateCompanyWorkflowOverrideSteps` chỉ kiểm rỗng (`internal/disclosure/app/workflow_override_validation.go:25-39`), `ValidateSnapshot` cũng chỉ kiểm rỗng (`internal/workflow/app/snapshot.go:169-174`).
  - Quyền tối thiểu: `template.workflow.override.write` + `.approve` (hoặc publish).
  - CMS facade có khả năng trùng giữa ID tự sinh và ID khai báo (plausible).
- **REM-06, cách sửa.** Không được dùng `reminder_sent=2`.
  - `ListByInstance` scan cột này vào `bool` (`internal/workflow/infra/mysql/milestones.go:55-71`). Gặp giá trị 2, `database/sql` báo lỗi, nên `GET /api/v1/workflows/instances/{id}/reminders` trả 500.
  - Khi đó FE (`DisclosureDetail.tsx:169-173`) xoá cả khối workflow.
  - Dùng `reminder_sent=1` + `reminder_occurrence_id=NULL`, hoặc mở rộng API trước.
  - Thêm nữa, step `Absent` hoặc step bị mở lại vẫn có thể trở lại đủ điều kiện, nên chỉ được đánh dấu khi milestone đã thật sự hết ý nghĩa.
- **Mức độ đã điều chỉnh:**
  - REM-03: MEDIUM, cần lỗi trong cửa sổ vài ms hoặc ctx bị huỷ.
  - REM-04: MEDIUM, chỉ tạo attempt, không gửi email.
  - OBX-01/H9: MEDIUM, cần ≥2 worker hoặc `Mark*` bị nuốt lỗi.
  - OBX-02/H8: MEDIUM, peer chết thì dừng trong khoảng 2 phút; chỉ treo vĩnh viễn khi peer im lặng.
  - PER-06: lên HIGH vì có ngày cụ thể.

## Critical / High (fix first)

Không có CRITICAL. 9 mục HIGH, xếp theo độ gấp:

### 1. REM-06 Milestone chết sẽ chặn scanner reminder quanh 13–14/10. Confirmed (code + DEV)
- **Code:** scanner `reminder_sent=0 AND scheduled_date<=? AND milestone_type LIKE 'due_minus_%' ORDER BY scheduled_date LIMIT 200` (`internal/reminder/infra/mysql/milestone_scanner.go:25-33`). Milestone không đủ điều kiện chỉ `continue`, không ghi gì (`internal/reminder/app/service.go:1002-1004`).
- **DEV:** 136 dòng chết, tăng khoảng 15/ngày, chủ yếu từ 6 template QA chu kỳ hằng ngày (DATA-01). Khoảng 4,2 ngày nữa (UTC 2026-10-14 ±1, có thể sớm hơn 1 ngày vì không có cột phá hoà trong ORDER), cả batch 200 sẽ toàn dòng chết. Từ đó reminder bước workflow mới **ngừng hẳn mà không có lỗi nào**.
- **Chi phí:** 136-200 query `StepTaskState` mỗi tick (2,3-3,5 triệu/ngày), full scan + filesort.
- **Fix:**
  - Khi milestone không đủ điều kiện **và** đã hết ý nghĩa (`scheduled_date + N < today` theo giờ HCM, hoặc instance đã kết thúc): `UPDATE ... SET reminder_sent=1, reminder_occurrence_id=NULL WHERE milestone_id=? AND reminder_sent=0`.
  - API hiển thị "hết hạn" khi `occurrence_id IS NULL`.
  - Index `(reminder_sent, scheduled_date)`.
  - Tick đầu tiên tự dọn 136 dòng.
- **Tạm thời:** tắt DATA-01 chỉ dừng tăng, không dọn được dòng đã có.

### 2. PER-01 Rollback worker về binary trước `cabbecf` chạy lại vòng lặp tạo Draft (nguồn sự cố 4 triệu). Confirmed (code + `ls` DEV)
- **Code:**
  - Binary cũ chọn `record_id IS NULL AND materialized_at IS NULL` (`git show cabbecf^:internal/disclosure/infra/mysql/repository.go:2455`).
  - `MarkPeriodicCycleFailed` mới lại đặt `materialized_at = NULL` (`repository.go:2556-2560`), nên cycle FAILED (vốn chắc chắn lỗi) lại hợp lệ với binary cũ.
  - Ghi chú `cabbecf` ("failed rows retain the legacy claim marker") sai với FAILED.
- **Binary bị ảnh hưởng:**
  - Mọi build từ `deb180e` (2026-05-20) tới trước `cabbecf`.
  - Hai file `worker.rollback.20261002T18*` còn mang lỗi SQL 1052 (đã sửa cùng đợt deploy `cabbecf`). Mọi cycle tới hạn tạo Draft mỗi tick; riêng DATA-01 đã khoảng 414 nghìn Draft/ngày.
- **DEV:**
  - 6 worker cũ trong `/root/cobo_project/bin`.
  - Deploy script không tự tạo backup, cũng không dọn; tên file "rollback" gây hiểu nhầm là an toàn.
  - Không có kiểm phiên bản lúc khởi động.
  - Binary cũ vẫn chạy được trên schema 0151 vì cột mới có default.
- **Fix:**
  - Ops: cách ly hoặc xoá 6 worker cũ và các `api.*` cũ hơn 2026-07-17 (xem SEC-26).
  - Runbook: "sàn rollback worker = `cabbecf`".
  - Code: `materialized_at = COALESCE(materialized_at, NOW(3))` khi FAILED, kèm data migration cho dòng FAILED cũ (DEV hiện có 0 dòng).
  - Log `vcs.revision` khi khởi động; kiểm schema version.
  - Phải làm **trước** PER-03/PER-04 (API-38).

### 3. REM-01 Upsert seed đặt lại occurrence đã gửi về PENDING mỗi tick khi milestone trùng. Confirmed (code); kích hoạt được qua override workflow
- **Chuỗi lỗi** (main agent đọc toàn bộ):
  1. `SeedOccurrence` dùng `ON DUPLICATE KEY UPDATE ... status = VALUES(status), scheduled_at = ...` (`internal/reminder/infra/mysql/repository.go:286-311`).
  2. SELECT theo `occurrence_id` mới không có dòng, nên `scanOccurrenceRow` trả 404 (`:519-531`).
  3. Hàm gọi `continue`, milestone không được đánh dấu (`service.go:1016-1019`).
  4. Query dispatch lấy `status='PENDING' AND scheduled_at<=?` (`repository.go:444-450`). Seed chạy trước dispatch trong cùng tick, nên email được gửi lại ngay tick đó.
- **Hệ quả:** khoảng 17.280 email/ngày cho mỗi key, cộng attempt và R dòng in-app. Trên DEV (Gmail), hạn mức gửi khoảng 500/ngày hết sau ~40 phút; sau đó email reset/OTP/mời của **mọi tenant** fail tới 24h và outbox chuyển chúng sang `failed_permanent`. Upsert này cũng đưa FAILED/RETRY về PENDING.
- **Đường kích hoạt:** override workflow có `step_id` trùng (xem Đính chính). DEV hiện có 0 nhóm trùng.
- **Fix (không cần migration):**
  - Seed riêng `INSERT ... ON DUPLICATE KEY UPDATE id=id`, đọc lại theo `idempotency_key`, đánh dấu milestone theo occurrence tìm được.
  - Chặn `step_id`/`step_code` trùng trong `ValidateSnapshot` (điểm chung mọi nguồn) và ở mọi chỗ ghi workflow (override, CMS facade sau `assignFacadeStepKeys`, ad-hoc runtime gate).
  - Sửa test fake để mô phỏng cả 2 unique key (PERF-47; đây là lý do lỗi lọt qua test).
  - Sau đó mới thêm UNIQUE milestone (API-36: backup dòng loại, giữ dòng có `milestone_id` = occurrence, `ALGORITHM=INPLACE, LOCK=NONE`).

### 4. REM-05 Occurrence bị skip không bao giờ kết thúc; đủ 50 dòng là chặn reminder mọi tenant. Confirmed
- **Code:** skip chỉ `continue`, không ghi gì (`service.go:346-357`). Query dispatch `ORDER BY scheduled_at LIMIT 50`.
- **Nguồn skip là thao tác thường ngày:** `TEMPLATE_DISABLED`, `RECIPIENT_EMPTY`, thiếu rule.
- **Hệ quả:**
  - Mỗi dòng sinh log, metric và cảnh báo SLA ở mỗi tick.
  - Đủ 50 dòng thì reminder của mọi company dừng.
  - Khi điều kiện skip hết, backlog cũ được gửi dồn một lượt.
- **DEV:** 0 dòng skip.
- **Fix (không cần migration):**
  - Khi skip: `UPDATE ... SET status='RETRY_SCHEDULED', next_retry_at=NOW()+1h, last_error_code=<reason> WHERE ... AND status='PENDING'`.
  - Quá 48h thì FAILED `REMINDER_EXPIRED`.
  - Nếu sau này thêm SKIPPED/EXPIRED: `status` là ENUM, phải thêm vào cuối danh sách bằng `ALGORITHM=INSTANT` và migrate trước binary (API-37).

### 5. H1 / REM-09 / BES-36 Config reminder không có authorization, tenant check hay giới hạn input. Confirmed, rộng hơn ghi nhận cũ
- **Code:** 5 route chỉ `subjectFromToken` (`internal/reminder/transport/http/handler.go:39-170`). Service không `authorize` (`service.go:177-270`); repository không có điều kiện company (`repository.go:42-72,107-113`).
- **Thiếu:**
  - giới hạn mảng `DaysBefore`/`SpecificDates`/`Recipients`/`Departments`;
  - miền giá trị;
  - giới hạn người nhận là member;
  - `MaxBytesReader` (chỉ có nginx 5 MB).
- **Người nhận:** `recipients_json` được đưa thẳng vào `RecipientEmails`, bỏ qua resolver. Bộ lọc policy chỉ chạy khi `NOTIFICATION_RULES_CONSUMER_ENABLED`, mà flag này mặc định tắt.
- **Tác động:** user đã đăng nhập của tenant A đọc/sửa lịch nhắc và danh sách người nhận của tenant B, và khiến SMTP của Cobo gửi email tới địa chỉ tuỳ ý.
- **Fix:**
  - `FindByID(company từ token)` + `authorize` (`disclosure.update` cho PUT, `disclosure.view` cho GET).
  - Kiểm `stepId` theo snapshot.
  - Giới hạn: ≤10 `daysBefore` (0..365), ≤10 ngày, ≤20 người nhận là member active, ≤10 phòng ban.
  - Body 16 KB.
  - Lúc dispatch, loại người nhận không phải member bất kể flag.

### 6. PER-06 Từ năm 2027, mọi cycle tính theo ngày làm việc bị bỏ im lặng; worker không dùng lịch nghỉ trong DB. Confirmed
- **Code:**
  - Worker không có `WithHolidayCalendarProvider`; chỉ API có (`internal/httpserver/server.go:435`).
  - `configs/non_trading_days/` chỉ có `2026.json`. Provider báo `missing holiday config for year` (`holiday_calendar_provider.go:63-66`), và seed `return false` không log (`periodic.go:216-219`).
- **Mốc thời gian:**
  - Cycle tháng 12 vắt sang tháng 1 hỏng từ khoảng 2026-12-01 (lead days có thể sớm hơn).
  - Từ 2027-01-01 hỏng toàn bộ cycle tính theo ngày làm việc (mặc định khi `deadlineDays>0`).
  - Lịch nghỉ tải lên qua CMS không được worker dùng, nên worker và API có thể tính ra hạn khác nhau.
- **Fix:**
  - Gắn composite provider cho worker.
  - Thêm `2027.json`.
  - Log + metric `cobo_periodic_seed_errors_total{reason}`.
  - Chọn chính sách rõ ràng khi thiếu năm.
- **Hạn chót:** 2026-12-01.

### 7. SEC-27 DEV gửi email thật mà không có allowlist hay redirect. Confirmed (code)
- **Cấu hình:** compose `SMTP_HOST: ${SMTP_HOST:-mailpit}`, nhưng `.env` trỏ Gmail. Ngoài adhoc shadow, không sender nào có allowlist hay redirect.
- **Tác động:**
  - DATA-01 đang gửi 20 email thật mỗi ngày.
  - Vòng lặp REM-01/REM-03 sẽ làm khoá tài khoản Gmail, kéo theo email reset/mời trên DEV.
  - `To:` chứa toàn bộ người nhận.
  - `.env.example` có dòng host Gmail chưa comment.
- **Fix:**
  - Một wrapper sender chung kiểm `EMAIL_RECIPIENT_ALLOWLIST`/`EMAIL_REDIRECT_TO`; khi `ENV=development` thì từ chối host SMTP thật không nằm trong allowlist.
  - Chỉ chuyển sang Mailpit **sau khi** sửa C6 (bind 127.0.0.1), nếu không link reset sẽ public qua :8025.
  - Gửi từng người hoặc dùng Bcc.

### 8. SEC-26 Endpoint internal reminder trên DEV có thể bị dùng làm relay gửi mail qua Gmail. Plausible
Mắt xích chưa kiểm: token trên server có còn đúng giá trị đã commit ở H22 không.
- **Code:**
  - `POST /internal/reminders/dispatch` nhận `recipient_emails` và `TemplatePayload` từ request (`internal/reminder/app/service.go:272-280`).
  - Build DEV không có `-tags prod`, nên còn route `/internal/dev/reminders/seed-occurrence` (`build_dev_seed_default.go:5`; `Makefile:64` build không tag).
  - Port :8080 public (H23).
  - `REMINDER_DISPATCH_ENABLED=false` không chặn được đường này (chạy trong API).
- **Binary API cũ:** DEV còn khoảng 7 file `api.*` cũ hơn `608c0258` (2026-07-17), ví dụ `api.bak.20260603_2128`, `api.bak.a2`, `api_new`, `api.new.deadlines`. Theo reviewer, các binary đó đăng ký route dispatch với token rỗng (plausible).
- **Fix:**
  - Rotate `INTERNAL_REMINDER_TOKEN`.
  - Build DEV với `-tags prod` hoặc tách flag cho route seed.
  - Lấy người nhận ở server.
  - Đóng :8080.
  - Cách ly binary api cũ.

### 9. H19 / API-34 `make deploy-be` không chạy migration, nên bản sửa nào thêm cột sẽ làm dừng luồng chính. Confirmed
- `Makefile:147-158` dùng `--no-deps api worker`, không migrate. Go không kiểm `schema_migrations`.
- Bản sửa thêm cột mà deploy bằng `be` sẽ gặp lỗi 1054:
  - `outbox_events.claimed_at`: outbox dừng (reset, mời);
  - `user_in_app_notifications.dedupe_key`: mọi insert in-app lỗi;
  - cột mới cho milestone: scanner dừng.
- Lỗi chỉ hiện thành WARN mỗi tick.
- **Fix:** ưu tiên thiết kế không đổi schema (xem ma trận). `deploy-be` phải từ chối chạy khi còn migration chưa apply.

## Medium / Low

### Backend Performance & Reliability
- **[M] PERF-43 Metric của worker không bao giờ được scrape.**
  - Worker đăng ký vào default registry nhưng không mở `/metrics` (`cmd/worker/main.go:200`, không có HTTP listener).
  - Không quan sát được REM-01/05/06 và PER-03/04.
  - Fix: thêm gauge từ DB vào collector của API (milestone chết, cycle CLAIMED >10 phút, cycle RETRY `attempt_count>20`, heartbeat `worker_last_tick`) kèm alert rule.
- **[M] PERF-44 (PER-04, gây REM-03)** SIGTERM huỷ ctx của tick đang chạy (`cmd/worker/main.go:126,231`). Cycle kẹt CLAIMED; row kẹt DISPATCHING rồi bị gửi lại. Compose không có `stop_grace_period`. Fix: `WithoutCancel` + timeout cho từng item; `stop_grace_period: 60s`; reaper CLAIMED.
- **[M] PERF-45 (PER-08, OBX-02)** Một goroutine chạy tuần tự mọi bước, nên bước chậm chặn các bước sau. Seed làm O(type × company) query mỗi 5 giây (100 × 30 ≈ 6 nghìn query/tick). Khi SMTP treo, reminder mất khoảng 8 phút/tick. Fix: seed tối đa 1 lần/phút; ngân sách thời gian cho từng bước, hoặc tách outbox ra vòng lặp riêng.
- **[M] PERF-46 / API-39 (PER-07)** Release của công cụ one-shot đặt `materialized_at=NOW(3), state='PENDING'`, không khớp selector nào, nên cycle mất vĩnh viễn (`repository.go:2514-2522`). Fix: release về RETRY.
- **[M] PER-03** RETRY mỗi 60 giây, không giới hạn; lỗi gate 4xx không chuyển FAILED. Chỉ làm **sau** PER-01.
- **[M] PER-05** `_ = fmt.Errorf("seed milestones...")` là lệnh không làm gì, nên lỗi không được log và milestone không bao giờ được tạo lại (`internal/workflow/app/service.go:148,161`).
- **[M] REM-02** `FOR UPDATE SKIP LOCKED` chạy ngoài transaction. Khi có 2 worker, mọi milestone có thể bị gửi 2 lần qua đường reset của REM-01.
- **[M] REM-03** Gửi → `InsertAttempt` → `UpdateDispatchResult` không trong transaction, nên có thể lặp mỗi 5 phút (288 email/ngày).
- **[M] REM-04** Lỗi evaluator retry vô hạn: khoảng 72 attempt/ngày, không có email.
- **[M] REM-07 (H11)** Sửa người nhận hoặc `planned_date` thì gửi lại mọi mốc đã qua.
- **[M] REM-08** `INSERT IGNORE…SELECT` mỗi tick, khoá S trên `disclosure_records`, đốt id (BIGINT, vô hại). Đang không hoạt động trên DEV (0 config).
- **[M] REM-10** In-app không có khoá chống trùng; nhân số dòng khi REM-01 xảy ra. Fix ưu tiên: `id` tất định theo `occurrence:user`, không cần migration.
- **[M] OBX-01 (H9)** Lease theo `available_at`. Cần ≥2 worker hoặc `Mark*` bị nuốt lỗi. Fix: set `available_at=NOW()` khi claim, không đổi schema.
- **[M] OBX-02 (H8)** 3/4 sender không timeout. Fix: dùng lại `boundedSendMail`.
- **[M] OBX-03** `MarkSending` không có điều kiện, nên `email.dispatch` có thể gửi 2 lần.
- **[M] DATA-01** 6 template QA chu kỳ hằng ngày × 4 company: khoảng 620 dòng và 20 email thật mỗi ngày; là nguồn chính của REM-06.
- **[M] SEC-28 Kill switch thiếu và không đổi được từ `.env`.**
  - Compose `environment:` ghi đè `env_file`: `WORKER_TICK_INTERVAL`, `WORKFLOW_REMINDERS_ENABLED`, `WORKFLOW_TIMELINE_ENABLED` là giá trị cứng.
  - `make dev-restart` không đọc lại `.env`.
  - `REMINDER_DISPATCH_ENABLED` (mặc định true) không có trong tài liệu nào và chỉ chặn đường reminder của worker. Bật lại thì backlog được gửi dồn.
  - Không có công tắc email chung.
- **[L]** REM-11 (lỗi QUIT sau DATA bị coi là tạm thời, gửi lại), OBX-07 (cắt `last_error` theo byte, `MarkRetry` lỗi, retry vô hạn), PER-08.

### Security Backend
- **[M] BES-38 (PER-02)** Template periodic phạm vi company được seed cho mọi company. `ListNextAlertCycles` không lọc theo chủ sở hữu, nên tenant khác thấy `type_id`, tên và tần suất (`internal/disclosure/app/periodic.go:79-83`; `internal/deadlinealerts/infra/mysql/repository.go:799-807`). Cần platform operator tạo type này; DEV có 0 dòng.
- **[M] BES-30 (còn mở)** `body.html` render bằng `text/template`. Trường do user điều khiển: tiêu đề/nội dung/lý do của adhoc, `disclosure_title`, `step_name`, `implementation_guide`. `recipient_name` lấy từ chuỗi người nhận đầu tiên, nên qua H1 ai cũng điều khiển được.
- **[M] BES-42 Quên mật khẩu không có giới hạn tần suất** (`internal/iam/app/service.go:470-509`). Bất kỳ ai, không cần đăng nhập, khiến Cobo gửi email reset không giới hạn tới mọi địa chỉ đã đăng ký; chỉ có rate limit nginx theo IP. Fix: tối đa 3/giờ cho mỗi user, cooldown 60 giây, phản hồi chung.
- **[M] OBX-04 / SEC-29** `outbox_events.payload_json` giữ vĩnh viễn link reset/mời và OTP (`MarkProcessed` chỉ UPDATE status). `qa_ro` và MySQL đang public (H20) đều đọc được. Fix: xoá `body` trong cùng UPDATE với `MarkProcessed`; thêm retention (loại trừ các bảng nêu ở API-41).
- **[L]** BES-39 (header ghép chuỗi; stdlib đã chặn CRLF), BES-40/SEC-30 (email người nhận trong `To:` chung, log, `last_error`), BES-41 (OBX-05 notify khi replay; key `workflow.approved` theo actor; OBX-06 ctx).

### API Compatibility
- **[M] API-35 (thuộc REM-06)** Muốn dùng giá trị 2 thì API phải mở rộng trước (scan int, 2 → `"cancelled"`). Từ đó sàn rollback API là bản mở rộng đó.
- **[M] API-36 (thuộc REM-01)** Migration UNIQUE có thể lỗi 1062 (dòng trùng, type rỗng, race với API). Runner exit 1 thì api/worker không khởi động khi `compose up`. Trình tự: validate trước → kiểm tra trước → backup → giữ dòng đúng → ALTER idempotent. DEV: 0 trùng, 0 rỗng.
- **[M] API-37 (thuộc REM-05)** ENUM: thêm vào cuối, migrate trước binary. Down migration phải remap trước. Metric cần gauge mới.
- **[M] API-38 (thuộc PER-01)** PER-03/PER-04 tạo thêm dòng FAILED/RETRY, nên chỉ an toàn sau khi đã sửa PER-01. Không bao giờ đưa FAILED về `PENDING/NULL`.
- **[M] API-40 (H18)** 0038/0099 thiếu trong runner nhưng có đường đọc từ worker. DEV có cột `instructions` và materialize vẫn chạy dù không có membership `m_system_worker`, nên rủi ro thuộc môi trường dựng mới.
- **[M] API-41** Retention: không được xoá `reminder_*`, `periodic_cycles`, milestone (chúng là sổ chống trùng; xoá đi thì tick sau gửi lại) hay `audit_logs` (có API và yêu cầu tuân thủ). An toàn để xoá: outbox đã xử lý, session hết hạn + thời gian đệm, idempotency hết hạn, in-app đã đọc và cũ.
- **[M] API-43** Runbook không có rollback từng process hay vệ sinh binary. Cần bảng sàn rollback: worker ≥ `cabbecf`; API ≥ bản mở rộng của REM-06 khi đã ghi giá trị 2.
- **[L] API-42** Endpoint reminder history, workflow reminders và in-app không có trong contract doc nào. Doc web ghi sai path unread-count.
- **[I] API-44** Company suspended (`5715d6c`) không còn được seed cycle (`repository.go:2606 status='active'`). Cần quyết định sản phẩm.

### CORS & Credentials
- **[L] SEC-31** Công cụ one-shot không so host/port với DSN và nhận DSN root (`cmd/periodic-materialize-one/main.go:30-31,58-70`).
- Các hit của secret scan trong phạm vi đều là false positive: tên hàm, enum `TokenKind`, nonce SHA-256, secret HMAC đọc từ env.

## Ma trận rollout (rút gọn từ api-compat)

| Fix | Đổi schema | Thứ tự deploy | Rollback |
|---|---|---|---|
| PER-01 | data-only | code + data cùng lúc | an toàn nếu worker ≥ `cabbecf` |
| REM-01 upsert + validate step ID | không | bất kỳ | an toàn |
| REM-01 UNIQUE | dedupe + ADD UNIQUE INPLACE | sau validate | down = DROP INDEX |
| REM-05 deferral (RETRY +1h) | không | worker | an toàn |
| REM-05 SKIPPED enum | ENUM append INSTANT | migration → worker | down phải remap |
| REM-06 (`=1` + occurrence NULL) | không (+ index tuỳ chọn) | worker (API tolerant) | an toàn |
| REM-10 (id tất định) | không | bất kỳ | an toàn |
| OBX-01 (`available_at` khi claim) | không | worker | an toàn |
| OBX-04 retention | index tuỳ chọn | job sau flag | an toàn (theo loại trừ API-41) |

## Quick wins (≤1h each)
1. **REM-06:** đánh dấu milestone đã hết ý nghĩa (`reminder_sent=1`, occurrence NULL) + test 200 dòng chết. **Làm trước 2026-10-13.**
2. **REM-01:** `ValidateSnapshot` và `ValidateCompanyWorkflowOverrideSteps` từ chối step ID trùng.
3. **REM-01:** seed upsert không ghi đè `status`/`scheduled_at`, đọc lại theo `idempotency_key`.
4. **REM-05:** skip → `RETRY_SCHEDULED` +1h; quá 48h → FAILED.
5. **PER-01:** `MarkPeriodicCycleFailed` dùng `COALESCE(materialized_at, NOW(3))`.
6. **PER-06:** thêm `configs/non_trading_days/2027.json` + log WARN khi seed thất bại.
7. **PERF-46:** release one-shot → RETRY.
8. **OBX-01:** set `available_at=NOW()` khi claim.
9. **OBX-02:** dùng `boundedSendMail` cho 3 sender còn lại.
10. **BES-30:** `html/template` cho `body.html`.
11. **BES-42:** giới hạn tần suất quên mật khẩu.
12. **PER-05:** log/return lỗi seed milestone thay vì `_ = fmt.Errorf`.

## Decisions needed from the team
- **Thao tác trên DEV (cần cho phép rõ ràng):**
  - cách ly 6 worker + khoảng 7 api cũ;
  - tắt hoặc archive 6 template QA hằng ngày;
  - có dọn 136 milestone chết bằng SQL ngay không, hay chờ bản sửa REM-06.
- **Email trên DEV:** allowlist/redirect (khuyến nghị) hay Mailpit (chỉ sau C6). Rotate `INTERNAL_REMINDER_TOKEN`; build DEV với `-tags prod`.
- **REM-06:** `reminder_sent=1` + NULL (không đổi API; UI hiện "Đã gửi"/"hết hạn") hay giá trị 2 (mở rộng API trước).
- **REM-05:** dùng RETRY tạm (không cần migration) hay thêm enum SKIPPED/EXPIRED. Chọn ngưỡng hết hạn 48h hay khác.
- **Retention:** chính sách cho `audit_logs`, in-app, outbox.
- **API-44:** company suspended có còn được tạo kỳ CBTT không.
- **Phạm vi tối đa:** người nhận reminder chỉ là member, hay vẫn cho email ngoài có duyệt.

## Not checked / BLOCKED
- **Ngoài phạm vi:** Security Frontend, Cache & Versioning.
- **MySQL integration test:** không có `MYSQL_TEST_DSN`. REM-01/03/06 chưa tái hiện trên DB thật; chỉ có bằng chứng code + dữ liệu DEV.
- **DEV chưa kiểm:**
  - `.env` (`NOTIFICATION_RULES_CONSUMER_ENABLED`, token thật);
  - log worker và tần suất WARN;
  - `go version -m` trên các binary;
  - `configs/non_trading_days` trên server;
  - loại tài khoản và hạn mức gửi của Gmail;
  - lịch sử git của `.env.example`.
- **FE:** chưa kiểm cách sinh step ID của override/CMS facade, và cách `mergeIncomingWorkflowSteps` xử lý ID trùng.
- **`govulncheck`, Docker build:** không chạy (read-only, không đổi code).
- **Đã chạy (reviewer):**
  - `go test` + `-race` trên reminder, outbox, adhoc/app, workflow/app, deadlinealerts, notification/infra/smtp: pass;
  - `notification/app` có 1 test fail từ trước (`TestContract_VariableParity/workflow.approved`);
  - `go vet` trên các package trong phạm vi: pass.

## Suggested fix plan (grouped by repo and PR)
1. **iam PR-W1 (reminder, gấp, trước 2026-10-13, không migration):**
   - REM-06, REM-01 (upsert + chặn step ID trùng ở `ValidateSnapshot`/override/CMS), REM-05 deferral, REM-03 (transaction + guard), REM-04 (giới hạn), REM-10 (id tất định), PERF-47 (fake).
   - Test: chạy lặp, crash sau side effect, 2 worker (`-race`).
2. **iam PR-W2 (periodic):** PER-01 (code + data), sau đó PER-03, PER-04/PERF-44 (reaper + `WithoutCancel` + `stop_grace_period`), PER-05, PERF-46, PER-06 (provider + 2027.json + metric), BES-38.
3. **iam PR-W3 (outbox/SMTP/email):** OBX-02, OBX-01, OBX-03, OBX-07, BES-30, BES-39/40, BES-42, REM-11.
4. **iam PR-W4 (H1):** BES-36 đầy đủ (authorize, company, giới hạn, người nhận là member, `MaxBytesReader`).
5. **iam PR-W5 (vận hành/deploy):** preflight migration cho `deploy-be` (H19/API-34), H18, runbook sàn rollback + vệ sinh binary (API-43), runbook kill switch + compose `${VAR:-default}` (SEC-28), gauge worker (PERF-43), wrapper allowlist email (SEC-27), SEC-26.
6. **iam PR-W6 (sau):** retention (OBX-04/SEC-29, loại trừ theo API-41), migration UNIQUE milestone (API-36), contract docs (API-42), SEC-31.
7. **Hạ tầng (cần quyền server):** cách ly binary cũ, rotate `INTERNAL_REMINDER_TOKEN`, tắt QA daily (DATA-01), C6/H20/H23 như các báo cáo trước.
