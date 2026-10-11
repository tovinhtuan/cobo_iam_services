# PR-2 Tính hạn + worker periodic: kế hoạch triển khai chi tiết (wf-bugfix Phase 3–6, chỉ lập kế hoạch)

> **Tóm tắt (VI)**
>
> Chia làm 2 phần:
> - **PR-2a (độ tin cậy, không đổi ngày pháp lý; ship được trước 2026-12-01):** PER-01, PERF-46, BES-38, PER-05, PER-04, PER-03, DUE-01, DUE-03, PER-06, DLA-01, DLA-05.
> - **PR-2b (ngày pháp lý, chỉ làm sau khi chốt Phase 0):** DUE-02, DUE-09, DUE-07, DUE-08, DUE-10, DLA-05 phần neo `created_at`.
>
> Migration duy nhất là `0153` (data-only, cho PER-01). Thứ tự chạy migration so với binary thế nào cũng được.
>
> **Lưu ý deploy:** `make deploy-be` copy thư mục `deploy-artifacts/backend/configs/`, không phải `configs/`. File ngày nghỉ mới phải đặt ở **cả hai** thư mục.
>
> Quyết định PR2-D1..D8: xem bảng ngay dưới; danh sách tổng hợp ở `02-implementation-plan.md`.
>
> Nguồn: agent lập kế hoạch PR-2 (read-only, HEAD `a8acb51`).

## Các điểm đã kiểm với code
- **Baseline:** `go test` pass trên holiday, deadlinealerts, personalops, `disclosure/infra/mysql`, `workflow/app`, `periodic_oneshot`. `internal/disclosure/app` có 6 test template-import đã fail từ trước.
- **Số migration kế tiếp:** `0153`.
- **3 nguồn loại ngày cho hạn:**
  - Worker (`periodic.go:206-215`): rules NULL và `deadlineDays>0` thì dùng WORKING.
  - Preview / fallback của alerts (`deadline_calculator.go:348-351`, `first_occurrence_preview.go:68-80`): dùng `cfg.deadline_duration_type`, nếu trống thì WORKING.
  - Shadow (`deadlineengine_shadow.go:185-191`): luôn là "working".
  - `deadlinealerts.dueDateFromTypeConfig` gọi calculator với config thô, nên vẫn rơi về WORKING ngay cả khi rules là calendar.
  - Migration 0097 chỉ đặt `calendar` cho type periodic global có rules. Type company/custom có rules NULL vẫn là WORKING.

## Phase 0 của PR-2: quyết định và dry-run read-only (GATE)
| ID | Quyết định | Phương án | Khuyến nghị |
|---|---|---|---|
| PR2-D1 (DUE-02) | Loại ngày khi tính hạn | — | **Đã định hướng theo Q-05:** theo đúng khai báo của template (`rules.deadline_day_type` → `deadline_config.deadline_duration_type`). Chỉ khi template không khai báo gì mới fallback CALENDAR, kèm validate bắt buộc khai báo. DEV: 12/12 template active đã khai báo (8 periodic + 1 ad-hoc ngày lịch, 3 periodic ngày làm việc), nên dự kiến không phải sửa dữ liệu |
| PR2-D2 (DUE-10) | Hạn theo ngày lịch rơi vào ngày nghỉ | — | **ĐÃ CHỐT (user 2026-10-10): không tự dời.** Tính theo quy định của template; ngày cụ thể (FIXED_DATE) là bắt buộc. DUE-10 đóng |
| PR2-D3 (DUE-07) | T0 của workflow khi submit thủ công | hôm nay / `cycle_start` / tính ngược từ hạn | **Hôm nay theo giờ HCM**, `t0_policy='system_date'` |
| PR2-D4 (DUE-08) | Timeline các bước vượt quá hạn | chỉ phát hiện / nén / tính lại | Chỉ phát hiện (log + trường bổ sung). Tính lại để sau, đi cùng PR-1 (RMC-04/REM-07) |
| PR2-D5 (DUE-09) | Cửa sổ tạo kỳ trước | `max(lead, open_days_before_t)` / validate ở CMS | Dùng `max` |
| PR2-D6 (DLA-05) | Hạn của Draft có `planned_date` NULL | neo vào `created_at` / ẩn dòng | Neo vào `created_at` |
| PR2-D7 (PER-03) | Lỗi 4xx nào là vĩnh viễn; giới hạn retry | — | 400/403/404/422 là vĩnh viễn; 402/409/429 thì retry; tối đa 20 lần, backoff 1→60 phút |
| PR2-D8 (DUE-03) | Danh sách ngày nghỉ | — | BA xác nhận các mục đánh dấu "TBC" |

---

### DUE-01 Upload lịch nghỉ lưu lùi 1 ngày
- **Mức độ / nguồn:** HIGH, audit DUE-01. Lỗi này chặn luôn cách sửa DUE-03/PER-06 bằng upload CMS.
- **Nguyên nhân gốc:**
  - `ReplaceCalendar` bind một `time.Time` 00:00 giờ HCM vào cột DATE. Driver chuyển sang `loc=UTC` (thành 17:00 ngày hôm trước), rồi MySQL cắt phần giờ, nên lưu ra ngày hôm trước.
  - Bằng chứng: `holiday/app/parser.go:77`, `service.go:91`, `infra/mysql/repository.go:99-102`; `loc=UTC` ở `docker-compose.dev.yml:70,143` và `docker-compose.artifacts.yml:106,192`.
  - Tầng infra/DB. Confirmed (code). Chưa từng chạy trên DEV (0 dòng).
- **Test tái hiện (GATE R):**
  - Unit `internal/holiday/infra/mysql/repository_test.go` `TestReplaceCalendar_BindsCivilDate`: dùng một fake `database/sql/driver` tối giản để bắt tham số `Exec`. Kỳ vọng tham số là chuỗi `YYYY-MM-DD`, hoặc `time.Time` có `.UTC()` cùng ngày.
    | Đầu vào | Kỳ vọng | HEAD |
    |---|---|---|
    | 2027-01-01 (+07) | "2027-01-01" | 2026-12-31 |
    | 2026-02-16 | "2026-02-16" | |
    | 2026-12-31 | "2026-12-31" | |
  - mysql-integration `TestReplaceCalendar_RoundTrip_MySQL` (`MYSQL_TEST_DSN`): `GetHolidayMapForYear(2027)` có 2027-01-01 và không có 2026-12-31.
- **Thiết kế sửa:**
  - `repository.go:102` bind `d.Date.Format("2006-01-02")` (dòng này đã ở giờ HCM).
  - `db_provider.go:59-61` không cache `err` nữa (một phần DUE-12).
  - Không đổi: parser, đường đọc (đúng với `loc=UTC`), DSN.
- **Contract / schema / FE:** không.
- **Dữ liệu:**
  - Kiểm tra: `SELECT calendar_year, COUNT(*), SUM(YEAR(holiday_date)<>calendar_year) FROM holiday_dates GROUP BY 1;` → DEV kỳ vọng 0 dòng.
  - Nếu có dòng: không dời bằng SQL; upload lại file XLSX sau khi deploy.
- **Rollout:** chỉ API, thứ tự nào cũng được. Rollback an toàn.
- **Verify (GATE V):**
  - `go test ./internal/holiday/... -count=1`
  - DEV, sau khi có lượt upload được cho phép: `SELECT MIN(holiday_date),MAX(holiday_date) FROM holiday_dates WHERE calendar_year=2027` nằm trong năm 2027.
- **Phạm vi ảnh hưởng:** các chỗ bind DATE khác đã format chuỗi: `UpsertPeriodicCycle` (`repository.go:2325-2337`) và `t0_date` của workflow (`workflow/infra/mysql/repository.go:74-75`). Reviewer grep `ExecContext` có bind `time.Time` vào DATE.
- **Phụ thuộc:** không. Phải vào **trước** khi có ai upload lịch 2027.
- **Ước lượng:** S

### DUE-02 / DLA-08 Ba nguồn loại ngày; rules NULL mặc định WORKING; TT96 dùng ngày lịch
> **Cập nhật 2026-10-10 theo Q-05:** nguyên tắc là "tính theo đúng loại ngày template khai báo".
> - Trên DEV, worker đã đúng, vì cả 11 template periodic active đều có `rules.deadline_day_type`.
> - Nơi còn sai là **preview CMS và fallback của danh sách hạn**: chúng đọc `deadline_duration_type` (trống) nên mặc định WORKING. Hệ quả là 8 template ngày lịch bị tính như ngày làm việc ở 2 nơi này.
> - Vì vậy resolver dùng chung là trọng tâm của bản sửa. Fallback CALENDAR chỉ dùng khi template không khai báo gì.
> - Thêm validate bắt buộc khai báo loại ngày khi tạo/publish template có `deadline_days>0`.
- **Mức độ / nguồn:** HIGH. BCTC Q3, N=20: hạn luật 20/10/2026, hệ thống tính 28/10.
- **Nguyên nhân gốc:**
  - Các nhánh fallback về WORKING nêu ở mục "Các điểm đã kiểm", không có resolver dùng chung.
  - Worker không đọc `$.deadline_duration_type`: `ListActivePeriodicTypes` (`repository.go:2257-2281`) không select trường này.
  - Bằng chứng: `periodic.go:213-215`, `deadline_calculator.go:348-351`, `first_occurrence_preview.go:68-80`, `deadlineengine_shadow.go:185-191`.
  - Tầng service. Confirmed.
- **Test tái hiện (GATE R, unit):**
  - File mới `internal/disclosure/app/periodic_daytype_test.go` `TestSeedPeriodicCycles_DayTypeResolution` (dùng lại `seedApplicableToRepo`, `noHolidaysProvider`):
    | Loại | Rules | now | Kỳ vọng | HEAD |
    |---|---|---|---|---|
    | quarterly N=20 | nil | 2026-10-05T03:00Z (T=2026-10-01) | **2026-10-20** | 2026-10-28 |
    | monthly N=10 | nil | 2026-11-03 (T = CN 2026-11-01) | **2026-11-10** | 2026-11-13 |
    | N=20 | `working` | như trên | 2026-10-28 | không đổi |
    | N=20 | day type `""` | như trên | 2026-10-20 | không đổi |
  - `TestPeriodicDayType_PreviewMatchesWorker`: rules {20, calendar}, không có profile. HEAD: preview 28/10, worker 20/10.
  - `deadlinealerts/app/service_test.go` `TestListDeadlineAlerts_NullPlannedDateUsesCalendarDefault`: quarterly N=20, now 2026-10-21T02:00Z → kỳ vọng 2026-10-20 OVERDUE (HEAD: 2026-10-28 UPCOMING).
- **Thiết kế sửa:**
  - File mới `disclosure/app/deadline_day_type.go`: `ResolvePeriodicDurationType(rules, cfgDT string) string`.
    - Thứ tự ưu tiên: `rules.DeadlineDayType` khai báo rõ → `cfgDT` đã chuẩn hoá (`WORKING_DAYS|working|CALENDAR_DAYS|calendar`) → `CALENDAR_DAYS`.
    - Comment trích TT96.
  - Thêm `PeriodicTypeRow.DeadlineDurationType` (`contracts.go:1260`); select `COALESCE(JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json,'$.deadline_duration_type')),'')` và scan.
  - Dùng resolver ở: `periodic.go:206-215`, `deadline_calculator.go:348`, `previewDurationType`, shadow (`DeadlineDayType` lấy từ resolver), `service.go:706`, `list_resolved_due.go:246`.
  - Không đổi: `applicability.ResolveDeadlineDurationType` (contract I-18), `deadlineengine.AddDays`, loại ngày của ad-hoc (0127), `periodic_oneshot/engine.go` (cố định WORKING, có guard REFUSE).
  - Cập nhật có review: `deadlineengine_shadow_test.go:338` (đang kỳ vọng "working").
- **Contract / schema / FE:** shape không đổi. Giá trị đổi: `deadline_summary.duration_type`, ngày hạn, câu mô tả quy tắc ("ngày dương lịch"). FE không cần sửa (chỉ hiển thị loại ngày ở màn hình preview import).
- **Dữ liệu (chỉ sau PR2-D1; không chạy):**
  - Kỳ mới tự đúng. Kỳ đã tồn tại giữ nguyên, vì upsert không ghi đè.
  - Dry-run lưu ở `migrations/dryrun_due02_calendar_default.sql`, **không** thêm vào runner:
    ```sql
    SET @today = DATE(CONVERT_TZ(UTC_TIMESTAMP(),'+00:00','+07:00'));
    SELECT pc.cycle_id, pc.company_id, pc.type_id, dtv.template_category, pc.cycle_label, pc.cycle_start,
           pc.due_date cur_due,
           DATE_ADD(pc.cycle_start, INTERVAL CAST(JSON_EXTRACT(dtv.deadline_config_json,'$.deadline_days') AS SIGNED)-1 DAY) cal_due,
           dr.record_id, dr.status, dr.planned_date,
           (SELECT COUNT(*) FROM reminder_configs rc WHERE rc.disclosure_id=dr.record_id AND rc.enabled=1) rem_cfgs
    FROM periodic_cycles pc
    JOIN disclosure_types dt ON dt.type_id=pc.type_id
    JOIN disclosure_type_versions dtv ON dtv.type_id=dt.type_id AND dtv.version_no=dt.active_version_no
    LEFT JOIN disclosure_records dr ON dr.record_id=pc.record_id
    WHERE (dtv.applicability_rules_json IS NULL OR JSON_TYPE(dtv.applicability_rules_json)='NULL')
      AND CAST(COALESCE(JSON_EXTRACT(dtv.deadline_config_json,'$.deadline_days'),0) AS SIGNED)>0
      AND UPPER(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json,'$.deadline_duration_type')),'')) NOT IN ('WORKING_DAYS','WORKING')
      AND (pc.record_id IS NULL OR (LOWER(TRIM(dr.status))='draft' AND dr.submitted_at IS NULL))
    HAVING cal_due<>cur_due ORDER BY cal_due;
    ```
  - Thêm một query thứ hai cho trường hợp rules có day type rỗng cộng `cfg=WORKING_DAYS`.
  - Báo cáo số dòng có `cal_due >= @today` và `< @today`.
  - Bản nháp sửa (sau khi PR2-D1 được duyệt):
    - backup sang `bak_due02_*`;
    - `UPDATE periodic_cycles pc JOIN tmp_due02 t USING(cycle_id) SET pc.due_date=t.cal_due WHERE pc.due_date=t.cur_due;`
    - `UPDATE disclosure_records dr JOIN tmp_due02 t USING(record_id) SET dr.planned_date=t.cal_due WHERE dr.planned_date=t.cur_due AND LOWER(TRIM(dr.status))='draft' AND dr.submitted_at IS NULL;`
  - Điều kiện tiên quyết: `rem_cfgs=0`. Nếu không, REM-07 sẽ gửi lại mọi mốc nhắc đã qua.
  - Milestone không đổi, vì T0 = `cycle_start`.
- **Rollout:** sau DUE-03 và PER-06. Worker + API cùng một lượt deploy. Chỉ sửa dữ liệu sau khi code đã chạy. Rollback: kỳ mới quay lại dùng WORKING.
- **Verify (GATE V):**
  - `go test ./internal/disclosure/... ./internal/deadlinealerts/... -count=1`
  - DEV: chạy lại dry-run thấy 0 dòng cho tập đã sửa; kỳ mới của type có rules NULL thỏa `due_date = cycle_start + N - 1`.
- **Phạm vi ảnh hưởng:** `dueDateFromTypeConfig` vẫn bỏ qua rules day type `working` cho dòng có `planned_date` NULL (hiếm); ghi lại trong tài liệu. Không đụng `periodic_oneshot/engine.go:216-220`.
- **Phụ thuộc:** PR2-D1 (BA/pháp chế); DUE-03 và PER-06 phải vào trước.
- **Ước lượng:** M (L nếu sửa dữ liệu)

### DUE-03 `2026.json` chỉ có 01/01: thêm lịch 2026, 2027 chính thức và kiểm tra CI
- **Mức độ / nguồn:** HIGH. DEV `holiday_dates` 0 dòng.
- **Nguyên nhân gốc:** file 2026 chỉ có 01/01, chưa có file 2027, không có gì kiểm tra. Tầng config. Confirmed.
- **Test tái hiện (GATE R, unit)**, file mới `internal/disclosure/app/holiday_calendar_files_test.go` đọc `../../../configs/non_trading_days`:
  - `TestNonTradingDayFiles_RequiredYears`:
    - Có file 2026 và 2027; trường `year` khớp tên file.
    - Mọi ngày parse được và thuộc đúng năm; không trùng.
    - Mỗi năm ≥9 mục.
    - HEAD fail (1 mục, thiếu 2027).
  - `TestNonTradingDayFiles_FixedSolarHolidays`: có 01-01, 04-30, 05-01, 09-02 nếu rơi vào ngày thường.
  - `TestNonTradingDayFiles_NextYearFromOctober`: khi `time.Now().Month()>=10` thì phải có file của năm sau. Cố ý phụ thuộc thời gian.
  - `TestNonTradingDayFiles_DeployArtifactsInSync`: bản ở `deploy-artifacts/backend/configs/non_trading_days/` giống hệt từng byte.
- **Thiết kế sửa:** thêm các ngày thường sau, `type` là `PUBLIC_HOLIDAY` hoặc `COMPENSATORY_DAY`, vào **cả hai** thư mục:
  - **2026:**
    - 01-01.
    - Tết: 02-16, 02-17, 02-18, 02-19, 02-20.
    - Giỗ Tổ rơi vào CN 04-26, nên nghỉ bù thứ Hai **04-27**.
    - 04-30, 05-01.
    - 09-02 + một ngày liền kề, 09-01 hoặc 09-03 (**TBC**).
    - 01-02 nếu có hoán đổi ngày làm việc (**TBC**).
    - 11-24 "Ngày Văn hoá Việt Nam" (**TBC**, chỉ khi đã được ban hành).
  - **2027:**
    - 01-01.
    - Tết (mùng 1 = thứ Bảy 02-06; 29 Chạp = 02-05): 02-05, 02-08, 02-09, bù 02-10, 02-11 (**TBC**: theo quyết định của Thủ tướng; phương án khác có 02-04).
    - Giỗ Tổ 10/3 âm lịch = thứ Sáu **04-16** (**TBC**: kiểm lại chuyển đổi âm lịch).
    - 04-30; 05-01 rơi vào thứ Bảy nên bù thứ Hai **05-03**.
    - 09-02 + 09-03 hoặc 09-01 (**TBC**).
    - 11-24 (**TBC**).
  - BA xác nhận quy tắc: ngày nghỉ chính thức + ngày nghỉ bù (BLLĐ Điều 111(3), 112), và có tính ngày làm việc hoán đổi không.
- **Contract / schema / FE:** không.
- **Dữ liệu:** không (hạn tính theo ngày lịch không phụ thuộc lịch nghỉ).
- **Rollout:** đi theo `deploy-be`, nhưng **chỉ qua bản trong `deploy-artifacts`**. Rollback an toàn.
- **Verify (GATE V):**
  - `go test ./internal/disclosure/app -run NonTradingDayFiles -count=1`
  - DEV: `ls /root/cobo_project/configs/non_trading_days/` thấy file 2026 và 2027.
- **Phạm vi ảnh hưởng:** test của `deadlinealerts` dựng provider với đường dẫn tương đối `"configs/non_trading_days"` tính từ package của nó, vốn không tồn tại. Giữ các test đó độc lập với file.
- **Phụ thuộc:** PR2-D8.
- **Ước lượng:** S (nhập liệu + BA duyệt)

### DLA-01 / DUE-04 "Hôm nay" lấy theo ngày UTC
- **Mức độ / nguồn:** MEDIUM. Mỗi ngày từ 00:00 tới 06:59 giờ VN:
  - mục quá hạn hiển thị "Sắp đến hạn";
  - mục đến hạn hôm nay hiển thị "Trong hạn";
  - bước bắt đầu hôm nay trả 409.
- **Nguyên nhân gốc:** lấy ngày từ một thời điểm mà chưa đổi sang giờ HCM.
  - `deadlinealerts/app/status.go:58` (đầu vào từ `service.go:138` `s.now().UTC()`).
  - `steps_algorithm.go:120` (đầu vào từ `time.Now`; container không đặt TZ).
  - Tầng service. Confirmed.
- **Test tái hiện (GATE R, unit):**
  - `status_test.go` `TestRemainingDaysFromDue_UTCInstantEarlyHCM`, now = 2026-10-19T18:30Z (= 20/10 01:30 giờ HCM):
    | Hạn | Kỳ vọng | HEAD |
    |---|---|---|
    | 2026-10-20 | 0 | 1 |
    | 2026-10-19 | -1 | 0 |
    Thêm biên 16:59:59Z và 17:00:00Z.
  - `steps_algorithm_test.go` `TestComputeDeadlineSteps_TodayFromUTCInstant`: T0 2026-10-20, today 2026-10-19T18:00Z → bước 0 phải là bước hiện tại (HEAD: khoá hoặc tương lai).
  - `personalops/app/mine_test.go` `TestRemainingDays_UTCInstantEarlyHCM`, cùng bộ dữ liệu.
- **Thiết kế sửa:**
  - `now = now.In(loc)` trước khi gọi `time.Date` trong `remainingDaysFromDue`.
  - `today = today.In(loc)` trong `ComputeDeadlineSteps`.
  - Sửa cùng một dòng như vậy ở `personalops/app/mine.go:49` (clock là UTC, `repository.go:76`).
  - Không đổi `reminder/infra/mysql/milestone_scanner.go:24` (`asOf.UTC()`): ngày UTC có nghĩa là gửi từ 07:00 giờ VN, và audit đã xác nhận điều đó là đúng.
- **Contract / schema / FE:** không; chỉ giá trị status thay đổi.
- **Dữ liệu:** không (tính lúc đọc).
- **Rollout:** API. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/deadlinealerts/... ./internal/personalops/... -count=1`. Tuỳ chọn: smoke DEV trong khung 00:00–07:00 giờ VN.
- **Phạm vi ảnh hưởng:**
  - Cùng loại lỗi: `disclosure/infra/workflow/record_status.go:27` (`PublishedDate = time.Now().UTC()`). Ứng viên sửa trong cùng commit.
  - `reminder/app/email_business_context.go:51` và `adhoc/app/proposal_due.go:55-80` format giá trị DATE (UTC midnight), đúng; reviewer xác nhận.
  - Đã đúng: `next_alerts.go:116`, `list_rows_membership.go:43`, `portaldashboard/domain/range.go:49`.
- **Phụ thuộc:** không.
- **Ước lượng:** S

### DLA-05 / DUE-05 `planned_date` NULL làm hạn "trôi"; một dòng lỗi làm danh sách và dashboard trả 500
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:**
  - Fallback tính hạn của kỳ **hiện tại** theo `now`.
  - Mọi lỗi của calculator đều bị trả ra cho cả danh sách: `deadlinealerts/app/service.go:188-190` (`return nil, err`) và `:438-447` (`dueDateFromTypeConfig(now)`).
  - Dashboard dùng chung `enrichDeadlineAlertRows`.
  - Tầng service. Confirmed.
- **Test tái hiện (GATE R, unit)**, `service_test.go`:
  - `TestListDeadlineAlerts_OneUnresolvableRowDoesNotFailList`: 2 dòng.
    - r-bad: không có `planned_date`, config periodic, holiday provider stub trả lỗi.
    - r-ok: 2026-09-01.
    - Kỳ vọng không lỗi, 1 item. HEAD trả lỗi.
  - Theo PR2-D6: `TestListDeadlineAlerts_NullPlannedDateAnchoredToCreatedAt`: monthly N=10, tạo 2026-07-05, now 2026-10-10 → hạn 2026-07-10 OVERDUE (HEAD: 2026-10-10 DUE_SOON).
- **Thiết kế sửa:**
  - Trong `enrichDeadlineAlertRows`: lỗi từ `resolveDueDateAndStatus` thì `slog.WarnContext("deadline alert row skipped: due unresolved", record_id, type_id, err)` rồi `continue`.
  - Theo PR2-D6:
    - Thêm `AlertRow.CreatedAt`: scan `dr.created_at` ở cả hai query ListRows (`repository.go` khoảng :102 và :506) và ở repo inmemory.
    - Truyền làm mốc thời gian tham chiếu cho `CalculateDeadlineSummary`.
    - Số ngày còn lại vẫn tính theo now thật.
  - Không đổi SQL membership.
- **Contract / schema / FE:** `total` có thể giảm đúng bằng số dòng bị bỏ qua (chồng với DLA-06). Ngoài ra không.
- **Dữ liệu:** chỉ đếm: `SELECT COUNT(*) FROM disclosure_records WHERE LOWER(TRIM(status))='draft' AND submitted_at IS NULL AND planned_date IS NULL;`
- **Rollout:** API. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/deadlinealerts/... ./internal/portaldashboard/... -count=1`. DEV: danh sách và dashboard trả 200 cho cả 4 company QA.
- **Phạm vi ảnh hưởng:** phần tính hạn ở personalops `mine.go`: reviewer kiểm có cùng kiểu `return err` không.
- **Phụ thuộc:** PR2-D6, chỉ cho phần neo.
- **Ước lượng:** S (M nếu làm cả PR2-D6)

### PER-06 / PERF-42 Worker không dùng lịch nghỉ trong DB; thiếu năm thì bỏ seed mà không báo
- **Mức độ / nguồn:** HIGH, hạn chót 2026-12-01.
- **Nguyên nhân gốc:**
  - Worker dựng `disclosureapp.NewService` không có `WithHolidayCalendarProvider` (`cmd/worker/main.go:132-140`), nên chỉ đọc file (`service.go:145`).
  - Lỗi "missing year" (`holiday_calendar_provider.go:63-67`) dẫn tới `return false` mà không log gì (`periodic.go:216-219`).
  - Lỗi upsert và lỗi profile cũng bị nuốt (`:162-164`, `:231-232`).
  - Tầng worker. Confirmed.
- **Test tái hiện (GATE R, unit):** `periodic_daytype_test.go` `TestSeedPeriodicCycles_MissingHolidayYearIsLogged`: type WORKING, yearly, T = 2027-01-01, provider trả lỗi. Bắt slog bằng handler test → kỳ vọng 1 dòng WARN có `reason=holiday_year_missing` và `type_id`. HEAD không log gì.
- **Thiết kế sửa:**
  - `cmd/worker/main.go`: dựng `holidaymysql.CompositeProvider{Repo, DB: NewDBProvider(repo), File}` giống `httpserver/server.go:426-436`, rồi truyền `WithHolidayCalendarProvider`.
  - Thêm TTL 5 phút cho `DBProvider` và `CompositeProvider.HasCalendarForYear`:
    - không có TTL thì mỗi ngày cần kiểm là một query, ở mỗi tick;
    - có TTL thì worker thấy lịch upload qua CMS trong vòng 5 phút.
  - Log WARN cho 3 lỗi đang bị nuốt, chống lặp theo (type, reason, year) mỗi giờ.
  - Collector DB phía API `internal/disclosure/infra/observe/periodic_collector.go` (theo mẫu `reminder/infra/observe/db_collector.go`, đăng ký cạnh nó ở `server.go:~806-822`):
    - `cobo_holiday_calendar_year_available{year="current|next"}`
    - `cobo_periodic_cycles{state}`
    - `cobo_periodic_cycles_claimed_stale`
  - Chính sách khi thiếu năm: vẫn bỏ qua nhưng phải ồn ào (log + metric). Không lặng lẽ coi như chỉ nghỉ cuối tuần, vì kỳ đã tạo thì không được tính lại.
- **Contract / schema / FE:** thêm series `/metrics`.
- **Dữ liệu:** không.
- **Rollout:** worker + API. Rollback an toàn.
- **Verify (GATE V):**
  - `go test ./internal/disclosure/... ./internal/holiday/... -count=1`
  - `curl :8080/metrics | grep cobo_holiday_calendar_year_available` = 1 cho năm hiện tại và năm sau (sau DUE-03).
  - Log worker không có `holiday_year_missing`.
- **Phạm vi ảnh hưởng:** chi phí seed mỗi tick (PERF-45); TTL giữ chi phí này không tăng.
- **Phụ thuộc:** DUE-01 (đường DB), DUE-03.
- **Ước lượng:** M

### DUE-10 Hạn theo ngày lịch rơi vào cuối tuần/ngày nghỉ không được dời
> **ĐÃ ĐÓNG theo quyết định Q-05 (user, 2026-10-10): không phải bug.**
> - Hạn tính theo quy định của template: ngày lịch thì giữ nguyên kết quả, kể cả khi rơi vào ngày nghỉ.
> - Ngày cụ thể (FIXED_DATE) là bắt buộc. Mặc định `WARN_ONLY_KEEP_DATE` đã đúng. `MOVE_TO_NEXT_WORKING_DAY` chỉ áp dụng khi người tạo template chọn tường minh trên CMS (`TemplateEditorScreen.tsx:1510-1529`).
> - **Không làm** flag `PERIODIC_DUE_ROLL_FORWARD_ENABLED` hay `resolvePeriodicDue` mô tả bên dưới. Phần đó chỉ giữ để tham khảo lịch sử.
> - **Chỉ thêm test khoá quyết định:**
>   - `TestSeedPeriodicCycles_CalendarDueOnWeekendIsKept` (T=2026-10-01, N=17, CALENDAR → 2026-10-17, thứ Bảy, không đổi);
>   - `TestFixedDate_DefaultKeepsDateOnNonTradingDay`;
>   - `TestFixedDate_MoveNextWorkingOnlyWhenTemplateSaysSo`.
- **Mức độ / nguồn:** LOW. DEV 96/318 kỳ, toàn bộ là template QA hằng ngày. Lỗi theo hướng sớm hơn hạn, không gây rủi ro tuân thủ.
- **Nguyên nhân gốc:** nhánh CALENDAR là `start.AddDate(0,0,N-1)`, không kiểm ngày không giao dịch (`deadline_calculator.go:247-248`). Tầng service. Confirmed.
- **Test tái hiện (GATE R, unit, khi bật flag):** `TestSeedPeriodicCycles_CalendarDueRollForward`:
  | T | N | Kỳ vọng | HEAD |
  |---|---|---|---|
  | 2026-10-01 | 17 | 2026-10-19 (T7 17/10 → dời) | 2026-10-17 |
  | 2026-05-01 | 1 | 2026-05-04 | |
  | 2026-04-01 | 27 (27/04 nghỉ bù) | 2026-04-28 | |
- **Thiết kế sửa:**
  - Hàm dùng chung `resolvePeriodicDue(ctx, calc, T, N, dt, rollForward)` cho worker, preview và fallback của alerts.
  - Env `PERIODIC_DUE_ROLL_FORWARD_ENABLED`, mặc định false.
  - Khi bật: lưu ngày đã dời và thêm câu "(dời sang ngày làm việc kế tiếp)" vào mô tả quy tắc. Khi tắt: chỉ hiện gợi ý (tuỳ chọn).
- **Contract / schema / FE:** ngày chỉ thay đổi khi bật flag.
- **Dữ liệu:** chỉ đếm, không sửa (kỳ đã tạo không bị tính lại; chỉ kỳ mới áp dụng):
  ```sql
  SELECT COUNT(*) FROM periodic_cycles
  WHERE DAYOFWEEK(due_date) IN (1,7)
    AND (record_id IS NULL OR materialization_state<>'COMPLETED');
  ```
  (Bản của agent thiếu ngoặc quanh điều kiện `OR`; main agent đã thêm.)
- **Rollout:** flag mặc định tắt. Chỉ bật sau DUE-03 và PER-06, vì cần đủ lịch nghỉ của mọi năm. Rollback = tắt flag.
- **Verify (GATE V):** `go test ./internal/disclosure/app -run RollForward -count=1`.
- **Phạm vi ảnh hưởng:** `NonTradingPolicyMoveNextWorking` đã có cho FIXED_DATE (`deadline_calculator.go:127`); dùng lại `getNextWorkingDay`.
- **Phụ thuộc:** PR2-D2.
- **Ước lượng:** M

### DUE-07 Submit thủ công lấy T0 = `planned_date`
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** `EnsureOnSubmit` đặt `t0 = rec.PlannedDate` (`disclosure/infra/workflow/bootstrap.go:44-49,60-61`), nên mọi bước kết thúc sau hạn. Tầng service. Confirmed; cần sản phẩm xác nhận ý đồ.
- **Test tái hiện (GATE R, unit):** `bootstrap_test.go` `TestEnsureOnSubmit_T0IsSubmitDateNotPlannedDate`: fake workflow service bắt request; `PlannedDate` = 2026-10-20; clock = 2026-10-10T02:00Z. Kỳ vọng `T0Date` 2026-10-10 và policy `system_date` (HEAD: 2026-10-20, `user_defined`).
- **Thiết kế sửa:**
  - Thêm clock có thể inject vào `Bootstrap`.
  - `t0` = ngày HCM của now, `T0Policy: "system_date"` (giá trị đã có trong enum FE, `cmsApi.ts:465`).
  - Không đổi đường periodic (dùng `cycle_start`).
- **Contract / schema / FE:** giá trị `t0_policy` thay đổi cho instance mới. api-compat xác nhận FE không rẽ nhánh theo giá trị này.
- **Dữ liệu:** không sửa (instance cũ giữ T0 đã đóng băng). Chỉ đếm:
  ```sql
  SELECT COUNT(*) FROM workflow_instances wi
  JOIN disclosure_records dr ON dr.record_id=wi.record_id
  WHERE wi.t0_date=dr.planned_date AND wi.status='in_progress';
  ```
- **Rollout:** API. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/disclosure/infra/workflow -count=1`.
- **Phạm vi ảnh hưởng:** đường finalize ad-hoc cũng gọi `EnsureOnSubmit` (AHA-07). Phối hợp với PR-3.
- **Phụ thuộc:** PR2-D3.
- **Ước lượng:** S

### DUE-08 Timeline các bước không bị giới hạn bởi hạn, không tính lại
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** `ComputeStepTimelines` cộng dồn `processing_days` từ T0 mà không có giới hạn (`disclosure/app/timeline.go:64-88`). Milestone không được tính lại khi `planned_date` đổi. Tầng service. Confirmed.
- **Test tái hiện (GATE R, unit, theo PR2-D4):** `timeline_test.go` `TestStepTimelines_ExceedsDueFlagged`: T0 2026-10-10, các bước 5+5+5, hạn 2026-10-20 → bước 3 (kết thúc 10-24) phải được đánh cờ. HEAD không có cờ.
- **Thiết kế sửa (khuyến nghị PR2-D4):**
  - Service các bước trả thêm `ExceedsDue bool` cho từng bước (trường bổ sung `exceeds_due`) và log WARN lúc tạo.
  - Không nén timeline. Việc tính lại milestone thuộc PR-1 (RMC-04/REM-07).
- **Contract / schema / FE:** trường bổ sung; api-compat review.
- **Dữ liệu:** chỉ đếm:
  ```sql
  SELECT COUNT(*) FROM (
    SELECT m.workflow_instance_id, MAX(m.scheduled_date) e, dr.planned_date p
    FROM workflow_step_milestones m
    JOIN workflow_instances wi ON wi.workflow_instance_id=m.workflow_instance_id
    JOIN disclosure_records dr ON dr.record_id=wi.record_id
    WHERE m.milestone_type='step_end' AND wi.status='in_progress'
    GROUP BY 1,3 HAVING e>p) x;
  ```
- **Rollout:** API. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/deadlinealerts/... ./internal/disclosure/app -run Timeline -count=1`.
- **Phạm vi ảnh hưởng:** reminder dùng ngày kết thúc bước (PR-1).
- **Phụ thuộc:** PR2-D4, DUE-07.
- **Ước lượng:** S (chỉ phát hiện) / L (tính lại, để sau)

### DUE-09 `open_days_before_t` bị bỏ qua khi lớn hơn `lead`
- **Mức độ / nguồn:** MEDIUM. Ví dụ: preview báo mở 26/09 nhưng record tạo ngày 01/10.
- **Nguyên nhân gốc:** chỉ tạo trước kỳ tương lai khi `lead>0`, vào ngày `T−lead` (`periodic.go:86-89,130`). Ngày mở `T−open_days` (`:220`) có thể đến trước khi kỳ được tạo. Tầng worker. Confirmed.
- **Test tái hiện (GATE R, unit):** `periodic_future_pregen_test.go` `TestFuturePregen_OpenDaysBeforeTExceedsLead`, monthly T=2026-10-01:
  | open | lead | now (HCM) | Kỳ vọng | HEAD |
  |---|---|---|---|---|
  | 5 | 0 | 2026-09-26 | tạo kỳ 2026-10, `open_at` 09-26 | không tạo |
  | 5 | 2 | 2026-09-26 | tạo kỳ | không tạo |
  | 5 | 7 | 2026-09-24 | tạo kỳ | tạo kỳ (không đổi) |
- **Thiết kế sửa:** `pregen := max(t.PeriodicCycleGenerationLeadDays, t.OpenDaysBeforeT); if pregen<=0 {continue}; generateAt := GenerateAtDate(futureT, pregen)`.
- **Contract / schema / FE:** không. Record xuất hiện sớm hơn, đúng như preview đã nói.
- **Dữ liệu:** không sửa. Liệt kê type bị ảnh hưởng:
  ```sql
  SELECT dt.type_id FROM disclosure_types dt
  JOIN disclosure_type_versions dtv ON dtv.type_id=dt.type_id AND dtv.version_no=dt.active_version_no
  WHERE dt.status='active'
    AND CAST(COALESCE(JSON_EXTRACT(dtv.deadline_config_json,'$.open_days_before_t'),0) AS SIGNED)
      > CAST(COALESCE(JSON_EXTRACT(dtv.deadline_config_json,'$.periodic_cycle_generation_lead_days'),0) AS SIGNED);
  ```
- **Rollout:** worker. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/disclosure/app -run FuturePregen -count=1`.
- **Phạm vi ảnh hưởng:** `first_occurrence_preview.go:174` (đã dùng `T−open`).
- **Phụ thuộc:** PR2-D5.
- **Ước lượng:** S

### PER-01 `MarkPeriodicCycleFailed` đặt `materialized_at = NULL`
- **Mức độ / nguồn:** HIGH. Theo API-38, phải vào **trước** PER-03/PER-04.
- **Nguyên nhân gốc:** chuyển sang FAILED xoá dấu claim cũ (`disclosure/infra/mysql/repository.go:2556-2560`). Binary trước `cabbecf` chọn `record_id IS NULL AND materialized_at IS NULL`, nên sẽ nhặt lại dòng FAILED và tạo Draft mỗi tick. Tầng DB/worker. Confirmed.
- **Test tái hiện (GATE R):**
  - Unit, source guard: `repository_periodic_test.go` `TestMarkPeriodicCycleFailedKeepsLegacyClaimFence` (dùng `extractFunc`). Thân hàm không được chứa `materialized_at = NULL` và phải chứa `COALESCE(materialized_at, NOW(3))`.
  - mysql-integration (`MYSQL_TEST_DSN`): claim → fail → selector của binary cũ trả 0 dòng.
- **Thiết kế sửa:**
  - Đổi thành `SET materialized_at = COALESCE(materialized_at, NOW(3))`.
  - Migration data `0153_periodic_cycles_failed_claim_fence.up.sql`:
    ```sql
    UPDATE periodic_cycles SET materialized_at=NOW(3)
    WHERE materialization_state='FAILED' AND materialized_at IS NULL AND record_id IS NULL;
    ```
  - Down: forward-only (`SELECT 1`), comment rằng revert sẽ khởi động lại vòng lặp.
  - Thêm `0153_periodic_cycles_failed_claim_fence.up.sql` vào `MIGRATIONS=` của `migrations/run_dev_migrations.sh`, sau dòng 0152 (dòng 155).
- **Contract / schema / FE:** chỉ dữ liệu.
- **Dữ liệu:** kiểm `SELECT materialization_state, COUNT(*), SUM(materialized_at IS NULL) FROM periodic_cycles GROUP BY 1;` → DEV kỳ vọng FAILED = 0.
- **Rollout:**
  - Code và 0153 không phụ thuộc thứ tự (`make deploy-dev MODE=all`, hoặc `migrate` rồi `be`).
  - Vận hành (cần cho phép): cách ly 6 binary worker cũ hơn `cabbecf`.
  - Runbook: sàn rollback worker = `cabbecf`.
- **Verify (GATE V):**
  - `go test ./internal/disclosure/infra/mysql -count=1`
  - `bash -n migrations/run_dev_migrations.sh && grep -c 0153 migrations/run_dev_migrations.sh`
  - DEV: `SELECT COUNT(*) FROM periodic_cycles WHERE record_id IS NULL AND materialized_at IS NULL AND materialization_state<>'PENDING';` = 0.
- **Phạm vi ảnh hưởng:** `ReleasePeriodicCycleClaim` (xem PER-07); Retry và Complete đã đặt `NOW(3)`.
- **Phụ thuộc:** không; làm đầu tiên.
- **Ước lượng:** S

### PER-03 RETRY không có giới hạn; lỗi 4xx không chuyển FAILED
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:** mọi lỗi (trừ workflow rỗng) đều thành RETRY sau 1 phút, mãi mãi (`periodic.go:280`; `repository.go:2530-2554`). Tầng worker. Confirmed.
- **Test tái hiện (GATE R, unit):** `periodic_materialize_test.go`:
  - `TestMaterializePeriodic_PermanentGateErrorMarksFailed`: creator trả `perr.NewHTTPError(403,…)` (đã wrap) → FAILED `GATE_403` (HEAD: RETRY).
  - `TestMaterializePeriodic_RetryableGateKeepsRetry`: 409/429.
  - SQL guard `TestMarkPeriodicCycleRetryCapsAttempts`: phải có `IF(attempt_count >= `.
- **Thiết kế sửa:**
  - Phân loại bằng `perr.AsHTTPError`: 400/403/404/422 → `MarkPeriodicCycleFailed(code "GATE_<status>")`.
  - SQL retry:
    ```sql
    materialization_state = IF(attempt_count>=20,'FAILED','RETRY'),
    next_attempt_at = IF(attempt_count>=20, NULL,
        DATE_ADD(NOW(3), INTERVAL LEAST(3600, 60*POW(2, LEAST(attempt_count-1,6))) SECOND)),
    last_error_code = IF(attempt_count>=20,'RETRY_EXHAUSTED',NULLIF(?,'')),
    materialized_at = NOW(3)
    ```
    `attempt_count` không bị gán trong câu lệnh này, nên thứ tự đánh giá các phép SET của MySQL không ảnh hưởng kết quả.
- **Contract / schema / FE:** không; sẽ có nhiều dòng FAILED hơn (có chủ đích).
- **Dữ liệu:**
  - Kiểm `SELECT materialization_state, COUNT(*), MAX(attempt_count), MIN(next_attempt_at) FROM periodic_cycles GROUP BY 1;` → DEV RETRY = 0.
  - Runbook để đưa một dòng FAILED quay lại hàng đợi bằng tay:
    ```sql
    UPDATE periodic_cycles SET materialization_state='RETRY', next_attempt_at=NOW(3), attempt_count=0
    WHERE cycle_id=? AND materialization_state='FAILED' AND record_id IS NULL;
    ```
- **Rollout:** sau PER-01, worker. Rollback an toàn (FAILED vẫn giữ dấu claim).
- **Verify (GATE V):** `go test -race ./internal/disclosure/app -run Materialize -count=1`.
- **Phạm vi ảnh hưởng:** `errPeriodicWorkflowUnavailable` vẫn được retry.
- **Phụ thuộc:** PER-01, PR2-D7.
- **Ước lượng:** S

### PER-04 / PERF-44 CLAIMED không có lease; SIGTERM huỷ ctx giữa chừng
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:**
  - `tick(runCtx…)` (`cmd/worker/main.go:126,231`) huỷ các lệnh DB đang chạy.
  - Cycle khi đó kẹt CLAIMED vĩnh viễn, vì `ListPendingCycles` chỉ lấy PENDING/RETRY (`repository.go:2456-2459`).
  - Không có reaper; compose không đặt `stop_grace_period`.
  - Tầng worker. Confirmed.
- **Test tái hiện (GATE R, unit):** `periodic_materialize_test.go`:
  - `TestMaterializePeriodic_CancelAfterClaimStillFinalizes`: creator huỷ ctx cha rồi trả thành công. Kỳ vọng `UpdateCycleRecord` được gọi với ctx còn sống (fake kiểm `ctx.Err()==nil`). HEAD fail.
  - `TestMaterializePeriodic_ReapsStaleClaims`: fake reaper được gọi đúng 1 lần mỗi lượt chạy.
  - SQL guard cho câu reaper.
- **Thiết kế sửa:**
  - Method repo mới `RequeueStalePeriodicClaims(ctx, olderThanSec int) (int64, error)`, thêm vào interface, repo mysql, repo inmemory và fake:
    ```sql
    UPDATE periodic_cycles
    SET materialization_state='RETRY', next_attempt_at=NOW(3), materialized_at=NOW(3),
        last_error_code='CLAIM_EXPIRED'
    WHERE record_id IS NULL AND materialization_state='CLAIMED'
      AND materialized_at < NOW(3) - INTERVAL ? SECOND
    ```
    So sánh hoàn toàn trong SQL, không dùng thời gian truyền từ Go.
  - Gọi reaper ở đầu `materializePeriodicDisclosures`. Lease theo `PERIODIC_CLAIM_VISIBILITY_TIMEOUT` (mặc định 10 phút, truyền qua service option).
  - Trong vòng lặp:
    - Kiểm `if ctx.Err()!=nil {return}` trước mỗi lần claim.
    - Sau khi claim thành công: `itemCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)`, dùng cho create và mark/complete.
  - Thêm `stop_grace_period: 60s` cho service worker trong `docker-compose.dev.yml` và `docker-compose.artifacts.yml`.
- **Contract / schema / FE:** chỉ compose.
- **Dữ liệu:** kiểm `SELECT cycle_id, cycle_label, due_date FROM periodic_cycles WHERE materialization_state='CLAIMED' AND materialized_at < NOW(3)-INTERVAL 10 MINUTE;` → DEV kỳ vọng 0. Phải xem trước khi deploy, vì reaper sẽ tự chạy lại các kỳ cũ.
- **Rollout:**
  - Sau PER-01; worker.
  - Thay đổi compose cần copy lại file artifacts lên server (`deploy-init`; bước này không tự chạy khi deploy).
  - Rollback an toàn.
- **Verify (GATE V):**
  - `go test -race ./internal/disclosure/app ./internal/disclosure/infra/mysql -count=1`
  - `docker compose -f docker-compose.dev.yml config | grep stop_grace_period`
  - DEV: `docker stop` worker giữa một tick → sau 10 phút không còn dòng CLAIMED kẹt.
- **Phạm vi ảnh hưởng:** reminder và outbox đã có reaper; thay đổi này theo cùng mẫu.
- **Phụ thuộc:** PER-01.
- **Ước lượng:** M

### PER-05 `_ = fmt.Errorf` nuốt lỗi insert milestone
- **Mức độ / nguồn:** MEDIUM. DEV có 1 instance do worker tạo mà không có milestone nào.
- **Nguyên nhân gốc:** lỗi build hoặc insert milestone bị bỏ đi, không log, không retry (`workflow/app/service.go:146-149,159-162`). Tầng service. Confirmed.
- **Test tái hiện (GATE R, unit):** `workflow/app/service_test.go` `TestCreateWorkflowInstance_MilestoneInsertFailureIsLogged`: fake milestone repo trả lỗi → bắt slog, kỳ vọng 1 dòng ERROR có `workflow_instance_id`. HEAD không có.
- **Thiết kế sửa:**
  - `slog.ErrorContext(ctx, "workflow milestones not seeded", stage build|insert, ids…, err)`.
  - Vẫn không làm hỏng request. Lý do: nếu trả lỗi thì sẽ sinh RETRY, RETRY lại tiếp tục với instance đã có, và vẫn không bao giờ seed milestone.
  - Sửa tương tự ở `workflow/infra/mysql/repository.go:263`.
  - Tuỳ chọn: gauge "instance đang chạy mà không có milestone" trong collector của PER-06.
- **Contract / schema / FE:** không.
- **Dữ liệu:** chỉ liệt kê (seed lại để làm sau; milestone chưa có unique key, xem REM-01):
  ```sql
  SELECT wi.workflow_instance_id FROM workflow_instances wi
  WHERE wi.status='in_progress' AND wi.t0_date IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM workflow_step_milestones m
                    WHERE m.workflow_instance_id=wi.workflow_instance_id);
  ```
  DEV kỳ vọng ≥ 1.
- **Rollout:** thứ tự nào cũng được. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/workflow/... -count=1`.
- **Phạm vi ảnh hưởng:** `grep -rn "_ = fmt.Errorf"` ra đúng 3 chỗ nêu trên.
- **Phụ thuộc:** không.
- **Ước lượng:** S

### PERF-46 / PER-07 Release của công cụ one-shot làm cycle mắc kẹt
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:**
  - `ReleasePeriodicCycleClaim` đặt PENDING kèm `materialized_at=NOW(3)`, nên không khớp selector nào (`repository.go:2514-2522`; selector ở `:2456-2459`).
  - Engine one-shot dùng record ID ngẫu nhiên (`periodic_oneshot/engine.go:125-143` gọi `CreateAndSubmitRecordWithPlannedDate`).
  - Tầng tool/DB. Confirmed.
- **Test tái hiện (GATE R, unit):**
  - SQL guard `TestReleasePeriodicCycleClaimRequeuesAsRetry`: thân hàm đặt `materialization_state = 'RETRY'` và `next_attempt_at = NOW(3)`.
  - `periodic_oneshot/engine_test.go` `TestEngine_UsesDeterministicRecordID`: `MemoryDomain` ghi nhận creator theo cycle đã được gọi.
- **Thiết kế sửa:**
  - Release → `state='RETRY', next_attempt_at=NOW(3), materialized_at=NOW(3)`.
  - `ProductionDomain` gọi `Creator.CreateAndSubmitPeriodicRecord(cycleID, …)`.
  - Không đổi các guard REFUSE.
- **Contract / schema / FE:** không.
- **Dữ liệu:**
  - Kiểm: `SELECT cycle_id, cycle_label FROM periodic_cycles WHERE record_id IS NULL AND materialization_state='PENDING' AND materialized_at IS NOT NULL;`
  - Bản nháp sửa (cần cho phép):
    ```sql
    UPDATE periodic_cycles SET materialization_state='RETRY', next_attempt_at=NOW(3)
    WHERE cycle_id IN (…) AND record_id IS NULL
      AND materialization_state='PENDING' AND materialized_at IS NOT NULL;
    ```
- **Rollout:** tool + repo. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/disclosure/app/periodic_oneshot ./internal/disclosure/infra/mysql -count=1`.
- **Phạm vi ảnh hưởng:** release là caller duy nhất.
- **Phụ thuộc:** PER-01. Tốt hơn nữa: gỡ bỏ công cụ này, vì phạm vi cố định của nó (2026-07) đã qua.
- **Ước lượng:** S

### BES-38 / PER-02 Type periodic phạm vi company bị seed cho mọi company; `ListNextAlertCycles` không lọc chủ sở hữu
- **Mức độ / nguồn:** MEDIUM (bảo mật / cô lập tenant). DEV có 0 dòng.
- **Nguyên nhân gốc:**
  - Seed lặp qua mọi company, chỉ kiểm `t.IsGlobal && !IsApplicable` (`periodic.go:79-80,104-114,166`).
  - `ListActivePeriodicTypes` không select chủ sở hữu.
  - `ListPendingCycles` và `ListNextAlertCycles` (`deadlinealerts repository.go:783-807`) không lọc theo chủ sở hữu, nên tenant khác thấy tên type và có thể có Draft được tạo.
  - Tầng service/SQL. Confirmed.
- **Test tái hiện (GATE R, unit):** `periodic_applicable_to_seed_test.go` `TestSeedPeriodicCycles_CompanyTypeOnlyForOwner`: type `IsGlobal=false` của c1, có 2 company c1 và c2 → chỉ upsert cho c1 (HEAD upsert cả hai).
  - Test cần trường `OwnerCompanyID` chưa có ở HEAD, nên HEAD fail ở bước biên dịch (vẫn tính là fail). Cách không cần trường: kiểm `upsertCountFor(c2)==0`.
  - Thêm SQL guard cho điều kiện chủ sở hữu ở cả 2 query list.
- **Thiết kế sửa:**
  - Thêm `PeriodicTypeRow.OwnerCompanyID`; select `COALESCE(dt.company_id,'')`.
  - Guard `if !t.IsGlobal && t.OwnerCompanyID != companyID {skip}` ở đầu cả hai vòng lặp company và trong `seedOneCompanySlot`.
  - Thêm `AND (dt.company_id IS NULL OR dt.company_id = pc.company_id)` vào `ListPendingCycles` và `ListNextAlertCycles`.
- **Contract / schema / FE:** không.
- **Dữ liệu:**
  - Kiểm:
    ```sql
    SELECT pc.cycle_id, pc.company_id, dt.company_id, pc.record_id
    FROM periodic_cycles pc JOIN disclosure_types dt ON dt.type_id=pc.type_id
    WHERE dt.company_id IS NOT NULL AND dt.company_id<>pc.company_id;
    ```
    Kỳ vọng 0.
  - Bản nháp sửa: `DELETE … WHERE cycle_id IN (…) AND record_id IS NULL`.
  - Nếu đã có record "ma" được materialize thì xử lý như sự cố bảo mật.
- **Rollout:** worker + API. Rollback an toàn.
- **Verify (GATE V):** `go test ./internal/disclosure/... ./internal/deadlinealerts/... -count=1`.
- **Phạm vi ảnh hưởng:** `ListPortalListCycleDues` lọc theo `pc.company_id`, nên an toàn sau khi seed được sửa.
- **Phụ thuộc:** không.
- **Ước lượng:** S

---

## Thứ tự commit

Mỗi commit: viết test tái hiện fail trước, rồi mới sửa.

**PR-2a (độ tin cậy, không đổi ngày pháp lý; ship được trước 2026-12-01)**
1. `fix(periodic): FAILED keeps legacy claim fence (PER-01)` + 0153 + runner.
2. `fix(periodic): one-shot release→RETRY, deterministic id (PERF-46/PER-07)`.
3. `fix(periodic): company types seed only for owner (BES-38/PER-02)`.
4. `fix(workflow): log milestone seeding failures (PER-05)`.
5. `fix(periodic): stale-claim reaper + WithoutCancel + stop_grace_period (PER-04/PERF-44)`.
6. `fix(periodic): capped backoff; permanent 4xx→FAILED (PER-03)`.
7. `fix(holiday): store civil dates; don't cache load errors (DUE-01)`.
8. `feat(holiday): 2026/2027 calendars + CI guard, both config dirs (DUE-03)` (cần PR2-D8).
9. `fix(worker): composite holiday provider + TTL, seed logging, API gauges (PER-06)`.
10. `fix(alerts): today from HCM date (+personalops) (DLA-01/DUE-04)`.
11. `fix(alerts): skip unresolvable rows (DLA-05/DUE-05)`.

**PR-2b (ngày pháp lý, sau khi chốt Phase 0)**

12. `fix(deadline): single day-type resolver, CALENDAR default (DUE-02/DLA-08)` + `dryrun_due02_*.sql`.
13. `fix(periodic): pre-seed at max(lead, open_days) (DUE-09)`.
14. Phụ thuộc quyết định, mỗi mục một commit: DUE-07 (T0), DUE-08 (phát hiện), DLA-05 phần neo (PR2-D6).
15. `test(deadline): lock Q-05`. Hạn ngày lịch không bị dời; FIXED_DATE giữ ngày trừ khi template chọn `MOVE_TO_NEXT_WORKING_DAY`. Chỉ thêm test, không đổi code (DUE-10 đã đóng).
16. Trong commit 12, thêm validate bắt buộc khai báo loại ngày khi tạo/publish template có `deadline_days>0` (CMS + import). Import hiện đã chuẩn hoá về CALENDAR khi không khai báo (`template_import_normalizer.go:239-244`).

## Reviewer (mở song song trong một message)
- **perf-reliability:** commit 1–6 và 9. Kiểm tính idempotent, lease của reaper so với timeout của từng item, trường hợp 2 worker, SIGTERM, chi phí query mỗi tick khi có TTL.
- **api-compat:**
  - migration 0153 và runner;
  - thay đổi `PeriodicTypeRow` và SQL;
  - giá trị `deadline_summary` thay đổi;
  - giá trị `t0_policy`;
  - ngữ nghĩa `total`;
  - trường tuỳ chọn `exceeds_due`;
  - thay đổi compose;
  - sàn rollback (worker ≥ `cabbecf`).
- **be-security:** commit 3 (cô lập tenant).
- **Tự kiểm theo `cobo-domain-cbtt`** (đối chiếu `references/tt96-reference.md`) cho DUE-02/03/07/09/10:
  - quy tắc ngày lịch;
  - T được tính là ngày 1;
  - dời hạn theo TT96 Điều 7 / BLDS Điều 148;
  - danh sách ngày nghỉ.
  - Chữ ký duyệt của BA/pháp chế ghi vào `00-report.md`.

## Checklist verify tổng hợp
- [ ] Mọi test tái hiện fail trên HEAD và pass sau khi sửa (kiểm bằng cách revert bản sửa).
- [ ] `go test ./internal/holiday/... ./internal/deadlinealerts/... ./internal/personalops/... ./internal/disclosure/... ./internal/workflow/... ./internal/portaldashboard/... -count=1`. Chỉ được còn 6 test template-import đã fail từ trước.
- [ ] `go test -race ./internal/disclosure/app ./internal/workflow/app -count=1`
- [ ] `go vet ./...` (chỉ còn cảnh báo đã biết của `workflowfulfillment`).
- [ ] `MYSQL_TEST_DSN=… go test -run 'MySQL|Integration' ./internal/holiday/infra/mysql ./internal/disclosure/infra/mysql -count=1`, hoặc ghi `BLOCKED: no MYSQL_TEST_DSN`.
- [ ] `docker compose -f docker-compose.dev.yml build api worker`, hoặc ghi `BLOCKED:`.
- [ ] `grep 0153 migrations/run_dev_migrations.sh`; `diff -r configs/non_trading_days deploy-artifacts/backend/configs/non_trading_days`.
- [ ] DEV trước deploy (read-only, `qa_ro`):
  - chạy mọi dry-run ở trên, ghi lại số đếm;
  - CLAIMED / mắc kẹt / "ma" đều = 0, hoặc đã được xem xét;
  - đếm `reminder_configs` cho các dòng của DUE-02.
- [ ] DEV sau deploy (cần cho phép):
  - `/healthz`, `/readyz` = 200;
  - `cobo_holiday_calendar_year_available` = 1 cho cả 2 năm;
  - `cobo_periodic_cycles{state="CLAIMED"}` ổn định;
  - log worker không có `holiday_year_missing`;
  - danh sách hạn và dashboard trả 200 cho 4 company QA;
  - sau PR-2b: kỳ mới thỏa `due = cycle_start + N - 1`.
- [ ] Runbook: sàn rollback worker là `cabbecf`; cách ly binary cũ (cần cho phép); ghi tóm tắt vào `docs/ai-cache/reusable-task-updates.md`.
