# Bug worker/alert: nguyên nhân gốc (wf-bugfix Phase 2)

Mỗi bug có một câu nguyên nhân, kèm bằng chứng (file:line ở HEAD `a8acb51` và/hoặc dữ liệu DEV), tầng gây lỗi, và mức xác nhận.
- **C** = confirmed: đã đọc đúng đường code, hoặc thấy trên dữ liệu.
- **S** = suspected / plausible.

Chi tiết sửa nằm ở `02a`–`02d`.

## HIGH

| ID | Nguyên nhân gốc (1 câu) | Bằng chứng | Tầng | Mức |
|---|---|---|---|---|
| RMC-01 | Điều kiện nhắc của bước chỉ dựa vào `workflow_tasks`, nhưng workflow `global_template`/`company_override` chỉ có task cho bước 1; tiến độ bước nằm ở `workflow_instance_step_states` | `reminder/infra/mysql/workflow_step_task_state.go:28-33`; `workflow/app/service.go:389-391` (`planLegacyCompleteIfNoPending` không tạo task kế tiếp); DEV: bước 2 = 0/125, bước 3 = 0/10, 323 instance/323 task | worker/service | C |
| REM-06 | Milestone không đủ điều kiện chỉ `continue`, không ghi gì, trong khi scanner luôn lấy 200 dòng cũ nhất, nên dòng chết chặn đầu hàng đợi | `reminder/app/service.go:1002-1004`; `milestone_scanner.go:25-33`; DEV: 136 dòng, +15/ngày | worker | C |
| REM-01 | Upsert seed `ON DUPLICATE KEY UPDATE status=VALUES(status)` đè lên occurrence đã SENT khi milestone trùng `(instance, step, type)`; đọc lại theo `occurrence_id` mới thì ra 404, nên milestone không bao giờ được đánh dấu | `reminder/infra/mysql/repository.go:286-311`; `service.go:1016-1020`; nguồn trùng: `disclosure/app/workflow_override_validation.go:25-39`, `workflow/app/snapshot.go:169-174` chỉ kiểm rỗng | DB/worker + validate | C (code); DEV có 0 nhóm trùng |
| REM-05 | Occurrence bị skip giữ nguyên PENDING; query dispatch `LIMIT 50` không có cận dưới, nên 50 dòng skip là chặn mọi tenant | `reminder/app/service.go:346-357`; `repository.go:444-451` | worker | C |
| AHA-01 | Finalize ad-hoc gọi `SubmitRecord` (PendingReview + `submitted_at`), trong khi danh sách hạn chỉ nhận Draft chưa submit | `adhoc/infra/disclosure/record_creator.go:136-143`; `disclosure/app/service.go:327-337`; `deadlinealerts/infra/mysql/list_rows_membership.go:19-21`; DEV khớp | API read model | C |
| AHA-07 | `SubmitRecord` gọi `EnsureOnSubmit` tạo instance template trước instance proposal; mọi nơi đọc chọn instance có UUID nhỏ nhất | `disclosure/app/service.go:342-346`; `bootstrap.go:26-67`; `disclosure/infra/mysql/repository.go:100-106`; `deadlinealerts/.../repository.go:110-116`; DEV có 2 instance | service + SQL | C |
| AHA-02 | Side effect hoàn tất (duyệt record, gửi email) gate theo hành động của task (`terminalStatus=="approved"`) thay vì trạng thái instance | `workflow/app/service.go:401-407` (instance approved khi hết bước); `:526-537` (gate theo action) | service | C |
| AHA-03 | Finalize chỉ lấy ngày chốt từ phiếu cuối (`LastFinal*`); upsert phiếu xoá giá trị cũ; hạn hiển thị dùng `proposed_t0_date` | `adhoc/infra/mysql/repository.go:489-494,509-519`; `adhoc/app/service.go:575-586`; `proposal_due.go:58-75` | service + SQL | C |
| DLA-02 | Helper enrich lọc `dr.status <> 'draft'` trong khi membership của danh sách chỉ nhận Draft, nên bước/phòng ban/người phụ trách luôn rỗng và `AllowsRow` loại dòng | `deadlinealerts/infra/mysql/repository.go:252,390`; `list_rows_membership.go:20`; DEV: 308/308 | read model | C |
| DLA-03 | Mọi preset dashboard đặt `to=today`, và service lọc item theo `[from,to]` trước khi chia nhóm | `portaldashboard/domain/range.go:55-67`; `app/service.go:136`; DEV: 26 UPCOMING + 6 OVERDUE bị bỏ mỗi company | service | C |
| DLA-04 | KPI hoàn thành dựa vào `completed_at`, cột này chỉ được ghi khi record sang trạng thái kết thúc, trong khi danh sách chỉ có Draft; severity đúng hạn dùng thang của tỉ lệ quá hạn | `disclosure/app/completion.go:20-31`; `portaldashboard/infra/mysql/completed_at.go:44-50`; `aggregate.go:318-338`; DEV `completed_at` = 0 | service | C |
| DUE-02 | Không có resolver loại ngày dùng chung; rules NULL thì mặc định WORKING ở worker, preview và fallback, trái TT96 (ngày lịch) | `disclosure/app/periodic.go:206-215`; `deadline_calculator.go:348-351`; `first_occurrence_preview.go:68-80`; `deadlineengine_shadow.go:185-191` | service | C (code); cách hiểu TT96 cần BA xác nhận |
| DUE-03 | File ngày nghỉ 2026 chỉ có 01/01, thiếu file 2027, không có kiểm tra | `configs/non_trading_days/2026.json`; `deploy-artifacts/backend/configs/...`; DEV `holiday_dates` = 0 | config | C |
| DUE-01 | Upload lịch nghỉ bind `time.Time` (00:00 HCM) vào cột DATE với `loc=UTC`, nên lưu thành ngày hôm trước | `holiday/app/parser.go:77`; `service.go:91`; `holiday/infra/mysql/repository.go:99-102`; DSN `loc=UTC` | infra/DB | C (code); chưa chạy trên DEV |
| PER-01 | `MarkPeriodicCycleFailed` đặt `materialized_at=NULL`, làm cycle FAILED khớp selector của binary trước `cabbecf` | `disclosure/infra/mysql/repository.go:2556-2560`; `git show cabbecf^:...:2455`; DEV còn 6 binary cũ | DB/worker + vận hành | C |
| PER-06 | Worker không có holiday provider DB; thiếu năm thì seed `return false`, không log | `cmd/worker/main.go:132-140`; `periodic.go:216-219`; `holiday_calendar_provider.go:63-67` | worker | C |
| H1 / BES-36 | 5 route reminder-config chỉ kiểm token: không `authorize`, không lọc company, không giới hạn input | `reminder/transport/http/handler.go:28-32,39-170`; `reminder/app/service.go:177-270`; `repository.go:42-72` | API | C |
| SEC-27 | DEV gửi SMTP thật mà không có allowlist/redirect | compose `SMTP_HOST`; `.env` Gmail | config/vận hành | C (code) |
| SEC-26 | `/internal/reminders/dispatch` nhận người nhận từ request; build DEV có route seed dev | `reminder/app/service.go:272-280`; `build_dev_seed_default.go:5` | API/build | S (token H22 chưa kiểm) |
| H19 / API-34 | `deploy-be` không chạy migration | `Makefile:147-158` | deploy | C |

## MEDIUM / LOW (tóm tắt; chi tiết ở `02a`–`02d`)

| ID | Nguyên nhân gốc | Tầng | Mức |
|---|---|---|---|
| RMC-02 | Dispatch không có cận dưới / khung giờ / kiểm lại; template in số âm. DEV: 80 bản gửi lúc 01:07 giờ VN | worker | C |
| REM-03 | Gửi → attempt → status là 3 lệnh autocommit riêng, trên ctx huỷ được | worker | C |
| REM-04 | Lỗi evaluator gọi `failRetryable` không giới hạn | worker | C |
| RMC-03 | Template mặc định gắn cứng `REMINDER_DISCLOSURE_DUE` cho cả WORKFLOW_STEP | worker | C |
| RMC-04 | Độ trễ bước chỉ được cộng ở màn hình hạn; milestone/hạn trong email không dời theo | worker | C |
| RMC-06 | `MarkMilestoneSent` chạy lúc seed; API ánh xạ `reminder_sent` thành "sent" | API | C |
| REM-10 | In-app có id ngẫu nhiên + INSERT thường, không có khoá chống trùng | service | C |
| RMC-09 | `StepSnapshot` không có `instructions` | service | C |
| DLA-01 / DUE-04 | Lấy ngày từ `now.UTC()` rồi gắn `loc` mà không `.In(loc)` | service | C |
| DLA-05 / DUE-05 | Fallback tính hạn theo kỳ hiện tại; một dòng lỗi thì trả lỗi cho cả danh sách | service | C |
| DLA-06 | `total` = `COUNT(*)` SQL, còn item bị lọc thêm ở app | service | C |
| DLA-07 | `ORDER BY created_at` không có khoá phá hoà (DEV: 47 nhóm) | SQL | C |
| DLA-09 | Cycle đã qua `open_at` mà chưa materialize thì không hiện ở đâu | read model | C |
| DLA-11 | Confirm lần hai ghi đè người xác nhận | SQL | C |
| DLA-12 | FE map status lạ thành "Trong hạn"; nhãn DUE_SOON sai nghĩa | FE | C |
| DUE-07 | `EnsureOnSubmit` lấy T0 = `planned_date` | service | C (ý đồ cần xác nhận) |
| DUE-08 | Timeline các bước không bị giới hạn bởi hạn | service | C |
| DUE-09 | Chỉ tạo kỳ trước theo `lead`, bỏ qua `open_days_before_t` | worker | C |
| DUE-10 | ~~Nhánh CALENDAR không dời hạn rơi vào ngày nghỉ~~. **Không phải bug** theo Q-05 (2026-10-10): hạn tính đúng quy định của template, không tự dời | service | Đã đóng |
| PER-03 | RETRY +1 phút mãi mãi, lỗi 4xx không phân loại | worker | C |
| PER-04 | Không có lease cho CLAIMED; tick dùng `runCtx` bị huỷ khi SIGTERM | worker | C |
| PER-05 | `_ = fmt.Errorf` nuốt lỗi milestone (DEV có 1 instance không có milestone) | service | C |
| PERF-46 / PER-07 | Release của one-shot đặt PENDING + `materialized_at`, không khớp selector nào | tool/DB | C |
| BES-38 / PER-02 | Seed type thuộc company cho mọi company; `ListNextAlertCycles` không lọc chủ sở hữu | service/SQL | C (DEV 0 dòng) |
| AHA-04 | `shortNote` cắt theo byte, UTF-8 hỏng, MySQL strict từ chối | notifier | C |
| AHA-05 | Không kiểm lại reviewer/quyền người tạo khi submit; `PrepareV3Materialization` không có caller | service/authz | C |
| AHA-06 | Reject/Cancel thắng cuộc đua với finalize, không có bù trừ | service | C |
| AHA-09 | `LookupCreatorEmail` join `users` bằng membership id; chuyển bước không thông báo | notifier | C |
| AHA-12 | Patch chỉ T0 làm `legacyDeadline` rỗng | service | C |
| AHA-13 | Script fixture sửa thẳng `workflow_tasks` | dữ liệu/vận hành | C |
| RMC-05 / AHA-11 | Reminder không đọc `companies.status`; seed chỉ lấy `status='active'` | worker/SQL | C |
| RMC-07 / AHA-08 | Chỉ đánh giá kênh email; in-app đi theo email; thiếu row prefs thì skip | worker | C (tiềm ẩn, sau flag) |
| RMC-08 | Lịch company [30,7,3,0] gate offset bước [3,1]; `department_focal` dùng phòng ban global | worker | C (tiềm ẩn, sau flag) |

## Nhận xét chung về gốc rễ
1. **Giả định cũ còn sót sau khi đổi mô hình:**
   - danh sách chuyển sang "chỉ Draft", nhưng helper và KPI vẫn giả định "không phải Draft" (DLA-02/04, AHA-01);
   - tiến độ bước chuyển sang `step_states`, nhưng reminder vẫn đọc task (RMC-01).
2. **Thiếu một nguồn sự thật chung:** chọn instance (AHA-07), loại ngày (DUE-02), định nghĩa "hôm nay" (DLA-01), tập kênh (RMC-07).
3. **Worker thiếu trạng thái kết thúc và lease:** REM-05, REM-06, PER-03, PER-04, REM-03.
4. **Fake test không mô phỏng ràng buộc MySQL:** PERF-47 (fake reminder không có 2 unique key), fake scanner bỏ qua `limit`. Đây là lý do các lỗi trên lọt qua test.
