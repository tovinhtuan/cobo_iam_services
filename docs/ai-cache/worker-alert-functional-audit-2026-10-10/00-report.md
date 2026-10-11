# Audit tính đúng chức năng của flow worker và alert (2026-10-10)

- Task type: phân tích read-only (domain CBTT / deadline / notification). Không sửa code, không ghi DB, không thao tác server.
- Workflow: `cobo-task-workflow` → class "analysis + domain". Chuỗi skill: `cobo-domain-cbtt` → `deadline-reminder-holiday` + `notification-delivery` + `backend-workflow-engine`, cộng phương pháp `bug-triage-debug` (mỗi giả thuyết phải có bằng chứng code hoặc dữ liệu).
  - Không dùng `wf-risk-review`: 6 nhóm của nó không bao phủ lỗi chức năng, và phần spam/duplicate đã review ở `../risk-review-2026-10-10-3/`.
  - Không dùng `wf-bugfix`: chưa có triệu chứng cụ thể. Bug nào đã xác nhận sẽ chuyển sang `wf-bugfix` khi user yêu cầu.
- Phạm vi: iam HEAD `a8acb51`; web `453a3494`/`79853d9d` (chỉ đọc phần hiển thị).
- 4 agent đọc code song song:
  - DLA: deadline alerts + dashboard;
  - DUE: tính hạn + lịch định kỳ;
  - RMC: reminder;
  - AHA: ad-hoc, in-app, prefs, workflow notify.
- Main agent tự đọc code mọi mục HIGH, và query DEV bằng `qa_ro` (SELECT) để tìm bất thường thật.

## Kết luận ngắn
**Có, nhiều bug chức năng thật.** 11 HIGH, tất cả đã được main agent đọc code xác nhận; 8 mục có thêm dữ liệu DEV khớp. Nhóm nặng nhất:
1. **Nhắc việc bỏ sót:** bước 2..N của workflow định kỳ không bao giờ được nhắc (RMC-01). Đây cũng là nguyên nhân gốc của 136 milestone chết ở REM-06.
2. **Ad-hoc (CBTT bất thường 24h) đã duyệt bị lạc:** không xuất hiện ở tab hạn (AHA-01); màn hình hiển thị nhầm workflow (AHA-07); ngày chốt của người duyệt bị mất (AHA-03); hoàn tất bước cuối không chuyển trạng thái record (AHA-02).
3. **Hạn pháp lý có thể bị tính trễ:** mặc định tính ngày làm việc thay vì ngày lịch (DUE-02). File ngày nghỉ 2026 chỉ có 01/01 (DUE-03). Upload lịch nghỉ lưu lùi 1 ngày (DUE-01).
4. **Màn hình cảnh báo/dashboard sai số liệu:** cột bước/phòng ban/người phụ trách luôn trống và nhân viên theo phòng ban không thấy cảnh báo của mình (DLA-02); dashboard không đếm mục sắp đến hạn và làm rơi mục quá hạn cũ (DLA-03); KPI hoàn thành luôn bằng 0 và mức đúng hạn bị đảo màu (DLA-04).

## Bằng chứng DEV (read-only, ~15:00-16:00 UTC 2026-10-10)
| Đo | Kết quả | Liên quan |
|---|---|---|
| Milestone `due_minus` đã đến hạn của workflow `global_template`, theo bước | bước 1: 224/225 được seed; bước 2: **0/125**; bước 3: **0/10** | RMC-01, REM-06 (136 = 125+10+1) |
| Instance `global_template` | 323 instance, **323 task** (1 task/instance, snapshot tới 6 bước) | RMC-01 |
| Record Draft có task mở | 308 (77/company × 4); **`department_id='general'` cho cả 308** | DLA-02 |
| Nhóm Draft trùng `created_at` | 47 nhóm, lớn nhất 14 dòng | DLA-07 |
| Dashboard preset 7 ngày, mỗi company | 26 mục sắp đến hạn không được đếm, 6 mục quá hạn cũ bị rơi | DLA-03 |
| `completed_at` | 0 trên mọi record | DLA-04 |
| Ad-hoc đã duyệt | record `PendingReview` + `submitted_at`, nên bị loại khỏi tab hạn. Có 2 instance: `global_template` (ID nhỏ hơn, được hiển thị) và `proposal_snapshot_v3` (gắn với proposal) | AHA-01, AHA-07 |
| `holiday_calendars` / `holiday_dates` | 0 dòng. File `configs/non_trading_days/2026.json` chỉ có 01/01 | DUE-03, DUE-01 |
| Reminder gửi dồn | 02/10 18:07-18:11 UTC (**01:07 sáng giờ VN**): 80 reminder của 29/09-02/10, trễ tới 3 ngày, ngay sau khi worker chạy lại | RMC-02 |
| Hạn rơi vào cuối tuần | 96/318 cycle, đều từ template QA daily (ngày lịch) | DUE-10 |
| Instance `01a0ba9c-…` kẹt (task `confirmed`, instance `in_progress`) | Do script fixture `_tmp_fixture_complete_ready.sql` sửa thẳng DB | AHA-13 |
| In-app `resource_id` NULL | 320 dòng tháng 8-9; tháng 10 = 0 (đã sửa ở `7cf7711`) | — |
| Người nhận không phải member / member inactive | 0 | — |

## HIGH (main agent đã đọc code xác nhận)

### RMC-01 Bước 2..N của workflow định kỳ/tuỳ chỉnh không bao giờ được nhắc; bước 1 vẫn bị nhắc sau khi đã xong
- **Code:**
  - Điều kiện nhắc chỉ tra `SELECT status FROM workflow_tasks WHERE ... step_code=?` (`internal/reminder/infra/mysql/workflow_step_task_state.go:28-33`).
  - Instance `global_template`/`company_override` đi qua `planLegacyCompleteIfNoPending` (`internal/workflow/app/service.go:389-391`). Planner này không tạo task cho bước sau, nên chỉ bước 1 có task.
  - Hoàn tất bước trên màn hình hạn chỉ ghi `workflow_instance_step_states` (`deadlinealerts/infra/mysql/steps_repository.go:99-170`), task vẫn `pending`.
- **Hệ quả:**
  - Mọi workflow nhiều bước chỉ nhắc bước 1.
  - Milestone của bước sau thành dòng chết (REM-06).
  - Bước 1 hoàn tất trên màn hình hạn vẫn tiếp tục bị nhắc.
- **Fix:** với nguồn không phải proposal, coi là đủ điều kiện khi "đây là bước hiện tại và chưa hoàn tất" (tính bằng `ComputeDeadlineSteps` trên `workflow_instance_step_states`); giữ kiểm task cho proposal v2/v3. Kiểm lại lúc dispatch.
- **Lưu ý cho REM-06:** sửa RMC-01 không tự dọn được dòng chết, vì bước 2 chưa tới lượt thì vẫn chưa đủ điều kiện. Vẫn cần đánh dấu hết ý nghĩa khi `step_end` đã qua hoặc instance đã kết thúc.

### AHA-01 Ad-hoc (CBTT bất thường) đã duyệt không bao giờ xuất hiện ở tab hạn
- **Code:**
  - Finalize gọi `SubmitRecord` (`internal/adhoc/infra/disclosure/record_creator.go:136-143`), không có `SkipCompanySubmit`. Periodic thì có (`:55,69`).
  - Record chuyển sang `PendingReview` + `submitted_at`.
  - Danh sách chỉ lấy `status='draft' AND submitted_at IS NULL` (`deadlinealerts/infra/mysql/list_rows_membership.go:19-21`).
  - Nhánh enrich hạn ad-hoc và PENDING_CONFIRM/DONE vì vậy không bao giờ chạy.
- **DEV:** proposal đã duyệt có record `PendingReview`.
- **Hệ quả:** nghĩa vụ 24h không hiện ở tab hạn và không thể xác nhận DONE. Đây là regression của các bản sửa 25-26/05.
- **Fix:** cần quyết định sản phẩm: thêm nhánh OR cho record gắn proposal đã duyệt, hoặc `SkipCompanySubmit` cho ad-hoc.

### AHA-07 (H15) Record ad-hoc có 2 workflow instance; màn hình hiển thị nhầm instance
- **Code:**
  - `SubmitRecord` gọi `EnsureOnSubmit` tạo instance template trước instance proposal (`record_creator.go:136` so với `:173`; `disclosure/infra/workflow/bootstrap.go:26-67`).
  - Record detail và danh sách hạn chọn `ORDER BY workflow_instance_id ASC LIMIT 1` (`disclosure/infra/mysql/repository.go:100-106`), tức instance template.
- **DEV:** confirmed: `global_template` (1 task, giao cho người tạo) và `proposal_snapshot_v3` (gắn proposal).
- **Hệ quả:**
  - Người dùng làm việc trên workflow sai.
  - Người tạo có thêm một task thừa.
  - Khi finalize retry, proposal bị gắn vào instance template.
- **Fix:** bỏ bootstrap ở đường ad-hoc; khi replay, tìm instance có `workflow_source LIKE 'proposal_snapshot%'`; chọn instance active/mới nhất thống nhất ở mọi nơi (DLA-10).

### AHA-02 Xác nhận/rà soát ở bước cuối làm instance `approved` nhưng record không được duyệt và không gửi email
- **Code:**
  - Planner đặt `inst.Status="approved"` khi không còn bước sau, bất kể hành động là gì (`internal/workflow/app/service.go:401-407`).
  - Side effect chỉ chạy khi `terminalStatus == "approved"`, tức theo **hành động của task**, không theo trạng thái instance (`:526-537`).
- **Hệ quả:** người có `deadline.manage`/`deadline.assign` bấm Confirm/Review ở task cuối thì instance được duyệt, nhưng `MarkRecordApproved` và `NotifyWorkflowApproved` không chạy. Record kẹt PendingReview vĩnh viễn.
- **Fix:** chạy side effect theo `plan.Instance.Status=="approved"`.

### AHA-03 (+ DUE-06) Duyệt nhiều người: chỉ lấy ngày chốt của người bỏ phiếu cuối; `final_t0_date` bị bỏ qua khi tính hạn
- **Code:**
  - `VoteReservation` chỉ mang `LastFinal*` của người gọi hiện tại (`internal/adhoc/infra/mysql/repository.go:509-519`), và `CompleteFinalize` ghi đúng giá trị đó (`adhoc/app/service.go:575-586`).
  - Hạn hiển thị dùng `proposed_t0_date` (`deadlinealerts/infra/mysql/repository.go:328`, `personalops`, `proposal_due.go:58-75`).
- **Hệ quả:**
  - Người duyệt A rút hạn về đúng giới hạn 24h, người duyệt B bấm duyệt cuối với ô trống: hạn cuối thành NULL và quay về T0+N.
  - Reviewer dời T0 thì cảnh báo hiển thị hạn lệch với lịch các bước.
- **Fix:** rút ngày chốt từ `ad_hoc_proposal_approvals` khi finalize (cần quy tắc sản phẩm, ví dụ lấy hạn sớm nhất); dùng `COALESCE(final_t0_date, proposed_t0_date)`.

### DLA-02 Danh sách hạn chỉ chứa Draft, nhưng các truy vấn phụ lại bỏ qua Draft, nên bước/phòng ban/người phụ trách luôn trống
- **Code:** `listCurrentStepMeta` và `listTaskAssigneeRecords` lọc `LOWER(TRIM(dr.status)) <> 'draft'` (`deadlinealerts/infra/mysql/repository.go:252,390`), trong khi membership chỉ nhận `= 'draft'` (`list_rows_membership.go:20`).
- **DEV:** 308 Draft có task mở, `department_id='general'` cho toàn bộ.
- **Hệ quả:**
  - Cột bước hiện tại và phòng ban luôn hiện "Chưa xác định"; lọc theo phòng ban không bao giờ khớp.
  - Dashboard "phòng ban rủi ro" và "điểm nghẽn" trống.
  - Nhân viên chỉ có quyền theo phòng ban không thấy cảnh báo của chính task mình. `total` vẫn đếm các dòng đó, và `/deadlines/{id}/steps` trả 403.
- **Fix:** bỏ điều kiện `<> 'draft'` (hoặc dùng cùng điều kiện với membership); làm `AllowsRow` khớp với `BuildListRowsScopeSQL`.

### DLA-03 Dashboard: khoảng ngày kết thúc ở hôm nay, nên không bao giờ đếm mục sắp đến hạn và làm rơi mục quá hạn cũ
- **Code:**
  - Mọi preset đặt `to = today` (`portaldashboard/domain/range.go:55-67`).
  - Trạng thái chỉ được đếm khi hạn nằm trong `[from,to]` (`portaldashboard/app/service.go:136`).
- **DEV:** mỗi company 26 mục UPCOMING không được đếm và 6 mục quá hạn cũ hơn 7 ngày bị rơi.
- **Fix:** OVERDUE đếm không theo range; UPCOMING dùng `[today, ∞)` hoặc cửa sổ 7 ngày; range chỉ áp cho KPI hoàn thành.

### DLA-04 KPI hoàn thành luôn bằng 0; PENDING_CONFIRM không bao giờ xuất hiện; màu "tỷ lệ đúng hạn" bị đảo
- **Code:**
  - `completed_at` chỉ được ghi khi record sang trạng thái kết thúc (`disclosure/app/completion.go:20-31`), mà danh sách chỉ có Draft.
  - `severityFromOnTimeRate` dùng lại `severityFromRate` vốn dành cho tỷ lệ quá hạn: ≥50% ra "critical" (`portaldashboard/app/aggregate.go:318-338`), nên 100% đúng hạn bị tô "critical".
- **DEV:** `completed_at` bằng 0 ở mọi record.
- **Fix:** KPI hoàn thành đọc từ `disclosure_records` (record kết thúc + `completed_at` trong range); dùng thang màu riêng cho tỷ lệ đúng hạn; sản phẩm chốt số phận PENDING_CONFIRM.

### DUE-02 (= DLA-08) Kỳ định kỳ mặc định tính ngày làm việc; TT96 quy định ngày lịch; 3 nguồn loại ngày lệch nhau
- **Code:**
  - Worker: khi `applicability_rules_json` NULL và `deadlineDays>0` thì dùng `DurationTypeWorkingDays` (`disclosure/app/periodic.go:206-215`). Worker không đọc `deadline_config.deadline_duration_type`.
  - Preview CMS và fallback API đọc `deadline_duration_type`, mặc định WORKING (`deadline_calculator.go:348-351`).
- **Hệ quả:** BCTC quý 3, N=20: luật là 20/10/2026, hệ thống tính 28/10, tức hiển thị **sau hạn pháp lý 8 ngày**. Preview và runtime cũng có thể cho ra 2 ngày khác nhau.
- **Fix:** một resolver dùng chung: day type của rules → `deadline_duration_type` → **CALENDAR**; bỏ nhánh `rules==nil → WORKING`.
- **Lưu ý:** cách hiểu TT96 lấy theo skill `cobo-domain-cbtt`; cần BA/pháp chế xác nhận lần cuối.

### DUE-03 File ngày nghỉ 2026 chỉ có 01/01; DB lịch nghỉ rỗng
- **Hiện trạng:** `configs/non_trading_days/2026.json` chỉ có "Tet Duong Lich". DEV có 0 dòng ở `holiday_dates`.
- **Hệ quả:**
  - Mọi phép tính ngày làm việc năm 2026 bỏ qua Tết (16-20/02), 30/4, 1/5, 2/9 và Giỗ Tổ.
  - Ví dụ ad-hoc ngày làm việc N=1 vào 13/02 cho ra 16/02, trong khi đúng phải là 23/02.
- **Fix:** nạp lịch chính thức 2026/2027; thêm kiểm tra CI về số ngày nghỉ tối thiểu mỗi năm.

### DUE-01 Upload lịch nghỉ lưu lùi 1 ngày
- **Code:**
  - Parser dựng 00:00 theo giờ HCM (`holiday/app/parser.go:77`) và truyền `.In(s.location)` (`service.go:91`).
  - Repository bind thẳng `time.Time` vào cột DATE (`holiday/infra/mysql/repository.go:100-102`), trong khi DSN đặt `loc=UTC`.
- **Hệ quả:** driver đổi giá trị sang UTC thành ngày hôm trước lúc 17:00, rồi MySQL cắt còn **ngày hôm trước**. Ví dụ 2027-01-01 thành 2026-12-31 dưới `calendar_year=2027`.
- **Ảnh hưởng tới bản sửa khác:** cách sửa PER-06/DUE-03 bằng "upload lịch qua CMS" sẽ sai ngay. Đường này chưa từng chạy trên DEV (0 dòng).
- **Fix:** bind `d.Date.Format("2006-01-02")`.

## MEDIUM
- **DLA-01 / DUE-04** "Hôm nay" lấy theo ngày UTC (`deadlinealerts/app/service.go:138` → `status.go:58`; `steps_algorithm.go:120`). Từ 00:00 tới 06:59 giờ VN mỗi ngày:
  - quá hạn hiển thị thành "Sắp đến hạn";
  - due-today hiển thị thành "Trong hạn";
  - bước bắt đầu hôm nay bị khoá (409).
  - Fix rẻ: `now.In(loc)` trước khi lấy ngày.
- **DLA-05 / DUE-05** Draft có `planned_date` NULL:
  - hạn "trôi" theo kỳ hiện tại, nên không bao giờ thành quá hạn;
  - một dòng không tính được hạn (thiếu năm nghỉ, cấu hình rỗng) làm cả danh sách và dashboard trả **500**.
  - Fix: log và bỏ qua dòng đó.
- **DLA-06** `total` của đường phân trang tính bằng `COUNT(*)` SQL, không khớp số item sau khi lọc.
- **DLA-07** `ORDER BY dr.created_at DESC` (độ chính xác giây) kèm LIMIT/OFFSET, nên trang sau có thể lặp hoặc mất dòng (DEV: 47 nhóm trùng). Fix: thêm `, dr.record_id DESC`.
- **DLA-09** Cycle đã qua `open_at` nhưng chưa materialize thì không hiện ở đâu.
- **RMC-02** Reminder gửi muộn, có bằng chứng DEV:
  - không có cận dưới thời gian, không có khung giờ gửi, không kiểm lại khi dispatch;
  - mốc 3d và 1d thành 2 email giống hệt nhau trong cùng tick;
  - template workflow in "Số ngày còn lại: -1 ngày".
  - Fix: chỉ gửi 1 thông báo "Quá hạn" cho mỗi bước; gửi trong khung 07:00-21:00 giờ VN; kiểm lại điều kiện khi dispatch.
- **RMC-03** Reminder bước workflow không có cấu hình CMS thì dùng template "hạn nộp nghĩa vụ" (`repository.go:486` gắn cứng `REMINDER_DISCLOSURE_DUE`). Hạn nội bộ của bước bị trình bày như hạn pháp lý, trong khi màn hình CMS ghi template khác.
- **RMC-04** "Trễ N ngày" dời các bước sau trên màn hình, nhưng milestone/reminder không dời theo. Email ghi hạn cũ và đến sớm.
- **RMC-05 / AHA-11** Company inactive vẫn nhận reminder (truy vấn không lọc `companies.status`). Company suspended lại không được tạo kỳ định kỳ mới (`repository.go:2606 status='active'`), mâu thuẫn với "suspended vẫn có hạn".
- **RMC-06** Danh sách reminder của workflow hiện "Đã gửi" ngay khi seed (`MarkMilestoneSent` chạy trước dispatch), kể cả khi bị skip hoặc FAILED.
- **RMC-07 / AHA-08** (tiềm ẩn, khi `NOTIFICATION_RULES_CONSUMER_ENABLED=true`):
  - chọn "chỉ in-app" thì không nhận được gì;
  - tắt in-app vẫn nhận in-app;
  - company không có row prefs thì không nhận reminder nào.
- **RMC-08** (tiềm ẩn, cùng flag) Lịch mặc định `[30,7,3,0]` không có 1, nên reminder 1 ngày của bước không bao giờ gửi. Policy `department_focal` dùng phòng ban của template global thay vì phòng ban của company.
- **AHA-04** `shortNote` cắt theo byte, làm hỏng UTF-8 tiếng Việt. MySQL strict từ chối, nên mất in-app (thử cục bộ: 12/30 ghi chú điển hình bị hỏng).
- **AHA-05** Proposal kẹt vĩnh viễn trong 3 trường hợp:
  - reviewer bị khoá (không kiểm lại khi submit, không đổi được reviewer);
  - người tạo thiếu `disclosure.create` (finalize luôn 403);
  - assignee v3 inactive.
- **AHA-06** Reject/Cancel trong lúc finalize (hoặc sau khi finalize lỗi) để lại record và workflow còn sống.
- **AHA-09** Hầu như không ai được báo khi workflow chuyển bước. Email "đã duyệt" không tới người tạo ad-hoc, vì `created_by` lưu membership ID nhưng lại được join với `users`.
- **DUE-07** Workflow tạo khi submit thủ công lấy `T0 = planned_date`, tức chính ngày hạn, nên mọi bước kết thúc sau hạn. Cần xác nhận ý đồ sản phẩm.
- **DUE-08** Tổng `processing_days` của các bước không bị giới hạn bởi hạn nộp; không tính lại khi `planned_date` đổi.
- **DUE-09** `open_days_before_t` không được áp khi vượt quá `lead`. Preview nói mở ngày 26/09 nhưng record được tạo ngày 01/10.

## LOW
- **DLA-10:** chọn instance bằng UUID nhỏ nhất (gộp vào AHA-07).
- **DLA-11:** xác nhận lại ghi đè người/thời điểm xác nhận.
- **DLA-12:** FE map trạng thái lạ thành "Trong hạn"; DUE_SOON thực chất là "đến hạn hôm nay".
- **RMC-09:** "Hướng dẫn thực hiện" luôn là text mặc định.
- **RMC-10:** đường config reminder trực tiếp: `Both` bỏ qua phòng ban; record kết thúc không qua submit vẫn bị nhắc.
- **RMC-11:** lỗi render template thành FAILED vĩnh viễn và mất lý do gốc.
- **AHA-10:** badge in-app đếm từ top-20 và còn giữ số của company cũ 60 giây sau khi đổi.
- **AHA-12:** sửa T0 trên draft làm mất ngày hạn tuyệt đối.
- **AHA-13:** instance kẹt trên DEV do script fixture; cần sửa dữ liệu và xoá script.
- **DUE-10:** hạn rơi vào ngày nghỉ không được dời sang ngày làm việc kế tiếp (sai theo hướng sớm hơn, không gây rủi ro tuân thủ).
- **DUE-11:** đếm T là ngày 1 (periodic) và không đếm T0 (ad-hoc) không nhất quán; nhãn kỳ: BCTC Q3 lại mang nhãn "2026-Q4".
- **DUE-12:** import lịch nghỉ: bỏ qua cột day type; Excel format 14 bị từ chối; lỗi bị cache.
- **DUE-13:** 29/02 thành 01/03 ở năm không nhuận.

## Đã kiểm, không có vấn đề
- **Phạm vi tenant:** truy vấn ad-hoc, in-app, recipient đều theo `company_id` và member active. DEV có 0 người nhận ngoài company hoặc inactive.
- **Timezone đúng:** ngày milestone, `due_date`, `remaining_days` của email tính theo giờ VN; email thường gửi lúc 07:00 VN. Membership của danh sách hạn dùng ngày làm việc HCM. Next-alert dùng ngày VN.
- **Quy tắc ngày:** cuối tháng/năm nhuận (`ClampDayOfMonth`) đúng; `planned_date` = `periodic_cycles.due_date` (314/314 khớp).
- **Ad-hoc:** bỏ phiếu không đếm trùng, cần đủ số phiếu, reject sau khi duyệt trả 409.
- **Bảo vệ route:** token inspector chặn company inactive/suspended trên mọi route alert.

## Kế hoạch sửa đề xuất (chưa làm; mỗi mục qua `wf-bugfix`)
1. **PR-F1 (reminder đúng người, đúng bước; gộp với PR-W1 của risk review lần 5):** RMC-01 + REM-06 (đánh dấu hết ý nghĩa), RMC-02 (cận dưới + khung giờ + kiểm lại khi dispatch), RMC-06, RMC-03, RMC-04.
2. **PR-F2 (hạn pháp lý):**
   - DUE-02 (resolver chung, mặc định CALENDAR, kèm migration dữ liệu/kiểm lại các cycle đang mở sau khi chốt với BA);
   - DUE-03 (lịch 2026/2027 đầy đủ + kiểm tra CI);
   - DUE-01 (bind chuỗi ngày);
   - DLA-01/DUE-04 (`now.In(loc)`);
   - DUE-05/DLA-05 (bỏ qua dòng lỗi);
   - PER-06.
3. **PR-F3 (ad-hoc):**
   - AHA-01 (cần quyết định sản phẩm);
   - AHA-07 (bỏ bootstrap ở đường ad-hoc + chọn instance đúng);
   - AHA-02 (side effect theo trạng thái instance);
   - AHA-03/DUE-06;
   - AHA-04, AHA-05, AHA-06, AHA-09.
4. **PR-F4 (danh sách hạn/dashboard):** DLA-02, DLA-03, DLA-04, DLA-06, DLA-07, DLA-09, DLA-12 (FE).
5. **PR-F5 (company status + prefs):** RMC-05/AHA-11, RMC-07/AHA-08, RMC-08.
6. **Dữ liệu DEV (cần user cho phép):** sửa instance `01a0ba9c-…` (AHA-13); dọn instance template thừa của ad-hoc đã duyệt (AHA-07).

## Cần quyết định
- **AHA-01:** CBTT bất thường đã duyệt hiển thị ở tab hạn theo cách nào (giữ PendingReview + nhánh riêng, hay không submit)?
- **DUE-02:** xác nhận với BA/pháp chế rằng mặc định là ngày lịch, và hạn rơi vào ngày nghỉ thì dời sang ngày làm việc kế tiếp (DUE-10). Các cycle đang mở có được tính lại không?
- **AHA-03:** quy tắc ngày chốt khi nhiều người duyệt (sớm nhất? của người duyệt chính?).
- **DLA-04:** giữ hay bỏ trạng thái PENDING_CONFIRM.
- **RMC-05:** company suspended có còn được tạo kỳ CBTT và gửi nhắc không.
- **DUE-07:** T0 của workflow tạo khi submit thủ công lấy từ đâu.

## Not checked / BLOCKED
- Không có `MYSQL_TEST_DSN`, nên không tái hiện được trên DB thật. Bằng chứng dựa trên code và dữ liệu DEV.
- Chưa gọi API DEV bằng persona để so trực tiếp kết quả hiển thị với SQL. Chưa kiểm FE hiển thị ngày.
- Chưa kiểm sâu checker `deadline.confirm` (khả năng IDOR trong tenant), entitlement khi hạ gói, các gate của `workflowfulfillment`.
- Test có sẵn đang fail, không do lần này: `disclosure/app` 6 test template-import; `notification/app` `TestContract_VariableParity/workflow.approved`.
