# Rollout, rollback, kill switch và sửa dữ liệu (bản nháp, chưa chạy)

> Mọi deploy, restart và thao tác ghi DB/server chỉ được làm khi user **yêu cầu rõ ràng** (CLAUDE.md, `deploy-dev-release`).
> Credential lấy qua `scripts/devqa/devqa_env.py`: đọc dùng `qa_ro`, ghi dùng `qa_rw` (chỉ khi được duyệt). Không bao giờ fallback sang root.

## 1. Sự thật về deploy cần nhớ
- `make deploy-be` (`Makefile:147-158`):
  - copy binary và `deploy-artifacts/backend/configs/` (không phải `configs/`);
  - recreate `api` + `worker` với `--no-deps`;
  - **không chạy migration** (H19).
- Vì vậy migration phải chạy bằng `MODE=migrate`/`all`. Các bản sửa chọn hướng không đổi schema; ngoại lệ duy nhất là `0153` (data-only, chạy trước hay sau binary đều được).
- Đổi env trong `.env` phải `up -d --force-recreate --no-deps worker`; `make dev-restart` không đọc lại env (SEC-28).
- Các biến `WORKER_TICK_INTERVAL`, `WORKFLOW_REMINDERS_ENABLED`, `WORKFLOW_TIMELINE_ENABLED` được ghi cứng trong compose `environment:`, nên `.env` không ghi đè được.
- Đổi compose (PER-04 `stop_grace_period`) cần copy lại `docker-compose.artifacts.yml` lên server (`deploy-init`).
- **Sàn rollback:**
  - Worker ≥ `cabbecf`. Không bao giờ dùng 6 binary `worker.bak*` / `worker.rollback.20261002T*`.
  - API ≥ `608c0258` (2026-07-17).
  - Sau khi PR-x đã deploy, rollback về trước PR-x chỉ mở lại lỗi PR-x đã sửa, không làm hỏng dữ liệu. Ngoại lệ: nếu sau này có ghi `reminder_sent=2` (kế hoạch này **không** ghi giá trị đó).

## 2. PR-1a (deadline 2026-10-12): trình tự deploy
1. **Trước deploy (read-only):**
   - Ghi lại số milestone chết (≈136+).
   - REM-01: kiểm tra trước trả 0 (milestone trùng, override trùng, gửi lặp).
   - REM-05: không có PENDING/RETRY cũ hơn 48h.
   - Kiểm `REMINDER_DISPATCH_ENABLED` hiện tại (mặc định true).
2. **Vận hành (cần cho phép, Q-25):**
   - Cách ly binary cũ: `mkdir bin/quarantine && mv bin/worker.bak* bin/worker.rollback.20261002T* bin/api.bak* bin/api_new bin/api.new.* bin/quarantine/`. Chỉ di chuyển, không xoá.
   - Bật allowlist email (hoặc tạm chuyển `SMTP_HOST` sang Mailpit đã bind 127.0.0.1), vì từ nay bước 2..N cũng được nhắc.
   - Tuỳ chọn: tắt 6 template QA chu kỳ hằng ngày (DATA-01).
3. **Dark launch:**
   - Đặt `REMINDER_DISPATCH_ENABLED=false` trong `.env`.
   - `make deploy-be` (hoặc `deploy-dev.sh be`).
   - `docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps worker`.
4. **Quan sát 2 tick:**
   - Số milestone chết giảm; số dòng obsolete (`reminder_sent=1 AND reminder_occurrence_id IS NULL`) xấp xỉ số chết trước đó.
   - Log không có ERROR.
5. **Bật gửi:** `REMINDER_DISPATCH_ENABLED=true` rồi recreate worker. Theo dõi 24h theo `03` §5.
6. **Rollback:** deploy lại binary `≥ a8acb51` trước đó. Dữ liệu tương thích; bị mở lại RMC-01, REM-06, REM-01.

**Kế hoạch dự phòng nếu PR-1a trễ quá 2026-10-12:** chạy SQL sửa REM-06 ở §6.1 (cần cho phép) để giải phóng scanner.

## 3. PR-2a: trình tự deploy
1. **Trước deploy:**
   - `periodic_cycles` theo state (FAILED/CLAIMED/RETRY = 0);
   - `holiday_dates` theo năm;
   - dry-run BES-38 (kỳ "ma" = 0);
   - PERF-46 (cycle mắc kẹt).
2. **Deploy:** `make deploy-dev MODE=all`, hoặc `migrate` rồi `be`. Đảm bảo:
   - 0153 đã chạy;
   - `deploy-artifacts/backend/configs/non_trading_days/{2026,2027}.json` có trên server;
   - compose mới có `stop_grace_period` (`deploy-init`).
3. **Sau deploy:**
   - `/metrics` có `cobo_holiday_calendar_year_available`;
   - log worker không có `holiday_year_missing`;
   - danh sách hạn + dashboard trả 200 cho 4 company QA.
4. **Rollback:** binary trước đó (≥ `cabbecf`); 0153 forward-only, vô hại.

## 4. PR-3: trình tự deploy và sửa dữ liệu
1. Deploy BE.
2. Sửa dữ liệu theo đúng thứ tự (mỗi bước: query kiểm tra trước → UPDATE trong transaction → query kiểm tra sau → ghi kết quả vào `05-completion-pr3.md`):
   1. AHA-13: instance fixture (§6.3).
   2. AHA-07: gắn lại proposal vào instance proposal; huỷ instance template lạc (§6.4).
   3. AHA-02: record đã có instance approved nhưng còn Draft/PendingReview (§6.5).
   4. AHA-06: workflow mồ côi (§6.6).
3. **Rollback BE:** an toàn. Dữ liệu đã sửa vẫn hợp lệ với binary cũ; khi đó binary cũ hiển thị instance template đã `rejected`.

## 5. PR-4, PR-1b, PR-5, PR-2b
- **PR-4:** BE trước (không đổi schema), WEB sau (`deploy-fe`). FE cũ chạy được với BE mới.
- **PR-1b:** worker. Bật khung giờ gửi mặc định; tắt bằng `REMINDER_SEND_WINDOW=off`.
- **PR-5:**
  - Ghi lại `NOTIFICATION_RULES_CONSUMER_ENABLED` và phân bố `companies.status` trước khi deploy.
  - Khi flag còn tắt: không đổi hành vi email/in-app.
  - Nếu bật flag sau này: company không có row prefs sẽ bắt đầu nhận reminder theo mặc định; ghi vào release notes.
- **PR-2b:**
  1. Deploy code.
  2. Chạy dry-run DUE-02, gửi BA.
  3. Sửa Draft đang mở theo §6.7 sau khi BA duyệt.
  - Điều kiện: `reminder_configs` của các dòng đó = 0; nếu không, REM-07 sẽ gửi lại các mốc nhắc đã qua.

## 6. SQL sửa dữ liệu DEV (BẢN NHÁP; chỉ chạy khi được duyệt từng mục)

### 6.1 REM-06: giải phóng milestone chết (chỉ khi PR-1a trễ)
Câu lệnh đầy đủ ở `02a` (tạo bảng tạm `tmp_rem06`, `SELECT COUNT(*)`, rồi `UPDATE ... SET reminder_sent=1, reminder_occurrence_id=NULL`).
- Kiểm tra trước: số dòng ≈ 136–180.
- Sau khi chạy: còn lại < 40.
- Rollback: bảng tạm giữ danh sách id; khôi phục bằng `UPDATE … SET reminder_sent=0 WHERE id IN (tmp)`.

### 6.2 PER-01: dấu claim của FAILED
Nằm trong migration `0153`, không chạy tay. DEV kỳ vọng 0 dòng bị ảnh hưởng.

### 6.3 AHA-13: instance bị fixture làm kẹt
Kiểm tra trước, kỳ vọng:
- instance `01a0ba9c-5cf3-774b-b253-164511cf6c4b`: `in_progress`, 1 task `confirmed`, record `01a0ba9c-5c81-7a19-b75a-ac524480fbf7`;
- record có đúng 1 instance;
- không có `step_states` hay `deadline_alert_confirmations`.

Sửa:
```sql
START TRANSACTION;
UPDATE workflow_tasks SET status='pending'
 WHERE workflow_instance_id='01a0ba9c-5cf3-774b-b253-164511cf6c4b' AND status='confirmed';
UPDATE workflow_step_milestones SET reminder_sent=1
 WHERE workflow_instance_id='01a0ba9c-5cf3-774b-b253-164511cf6c4b' AND reminder_sent=0
   AND scheduled_date < DATE(CONVERT_TZ(UTC_TIMESTAMP(),'+00:00','+07:00'));
COMMIT;
```
Sau đó `git rm deploy-artifacts/_tmp_fixture_complete_ready.sql` (trong PR-3).

### 6.4 AHA-07: hai instance của record ad-hoc
Kiểm tra trước: query ở `02c`. DEV kỳ vọng 1 proposal, 2 instance.

Sửa:
```sql
START TRANSACTION;
UPDATE ad_hoc_proposals p
JOIN workflow_instances wi ON wi.company_id=p.company_id AND wi.record_id=p.record_id
                         AND wi.workflow_source LIKE 'proposal_snapshot%'
SET p.workflow_instance_id=wi.workflow_instance_id
WHERE p.status='approved' AND p.workflow_instance_id<>wi.workflow_instance_id;
-- Với các instance không phải proposal, non_pending=0 và step_states=0 (liệt kê id sau bước kiểm tra trước):
UPDATE workflow_tasks SET status='cancelled' WHERE status='pending' AND workflow_instance_id IN (/*ids*/);
UPDATE workflow_instances SET status='rejected' WHERE status='in_progress' AND workflow_instance_id IN (/*ids*/);
COMMIT;
```

### 6.5 AHA-02: record kẹt sau khi instance đã approved
Kiểm tra trước: instance ưu tiên `approved`, record ∈ {`Draft`, `PendingReview`, `In Progress`}, trừ các instance template lạc.

Sửa (không gửi bù email):
```sql
UPDATE disclosure_records dr JOIN workflow_instances wi ON ...
SET dr.status='Approved', dr.published_date=COALESCE(dr.published_date, DATE(wi.updated_at))
WHERE ...;
```

### 6.6 AHA-06: workflow mồ côi
Kiểm tra trước: query ở `02c`. Sửa bằng cách huỷ task pending và đặt instance thành `rejected` (giống 6.4).

### 6.7 DUE-02: Draft đang mở sang ngày lịch (sau Q-04)
1. Chạy dry-run `migrations/dryrun_due02_calendar_default.sql` (không đưa vào runner).
2. Backup sang `bak_due02_*`.
3. `UPDATE periodic_cycles` / `disclosure_records` với điều kiện `due_date=cur_due`, chỉ Draft chưa submit và `rem_cfgs=0`.
4. Liệt kê riêng các dòng có hạn mới < hôm nay để BA quyết.

### 6.8 PERF-46: cycle mắc kẹt do one-shot
```sql
UPDATE periodic_cycles SET materialization_state='RETRY', next_attempt_at=NOW(3)
WHERE cycle_id IN (/*kiểm tra trước*/) AND record_id IS NULL
  AND materialization_state='PENDING' AND materialized_at IS NOT NULL;
```

## 7. Kill switch (tham chiếu SEC-28)
| Muốn | Cách | Lưu ý |
|---|---|---|
| Dừng gửi reminder (worker) | `REMINDER_DISPATCH_ENABLED=false` + recreate worker | Seed/materialize vẫn chạy. Nếu chưa có PR-1b, bật lại sẽ gửi dồn backlog |
| Dừng tạo kỳ CBTT | `PERIODIC_SEEDING_ENABLED=false` + recreate | |
| Tắt khung giờ gửi (sau PR-1b) | `REMINDER_SEND_WINDOW=off` | |
| Dừng mọi email | Chưa có công tắc chung (SEC-28): chuyển `SMTP_HOST` sang Mailpit (bind 127.0.0.1), hoặc dùng allowlist (PR-W5) | Endpoint `/internal/reminders/dispatch` không chịu `REMINDER_DISPATCH_ENABLED` |
| (Đã bỏ) Dời hạn sang ngày làm việc | Không có flag: theo Q-05, hạn tính đúng quy định của template, không tự dời | FIXED_DATE chỉ dời khi template chọn `MOVE_TO_NEXT_WORKING_DAY` |

## 8. Theo dõi sau deploy (gắn alert khi có PERF-43)
- Milestone chết ≥150 → cảnh báo.
- Occurrence `PENDING` hoặc `RETRY_SCHEDULED` cũ hơn 48h > 0.
- `reminder_delivery_attempts` có occurrence SENT > 1 lần.
- Cycle `CLAIMED` cũ hơn 10 phút > 0.
- Cycle `RETRY` có `attempt_count` > 20.
- `holiday_calendar_year_available{year="next"}` = 0 kể từ tháng 10.
- Giờ gửi email nằm ngoài 07–21 giờ VN.
