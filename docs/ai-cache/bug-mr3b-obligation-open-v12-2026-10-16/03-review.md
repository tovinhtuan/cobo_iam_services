# 03: Review (Phase 6), blast radius (Phase 5), câu hỏi PO

Ba reviewer chạy song song, chỉ đọc: `db-tx-reviewer`, `api-compat-reviewer`, `perf-reliability-reviewer`. Không có finding High nào do MR-3b gây ra. PERF-01 là High nhưng có từ trước (xem bên dưới).

## Đã sửa
| ID | Mức | Nội dung | Xử lý |
|---|---|---|---|
| DB-02 | Medium | Record `NotStarted` (bản ghi toàn cục của CMS, được service coi là trước khi nộp, như Draft) sẽ mất nhắc hạn. Lần nhắc đang chờ sẽ bị đóng hẳn | Cổng nhắc hạn (`reminderDeadlineOpenSQL`) giữ quy tắc cũ cho `NotStarted` (chưa nộp) tới khi PO quyết. Có test |
| DB-05 / API-06 | Low | Cổng mới áp cả cho `reminder_configs` theo bước | Chỉ áp cho config `DISCLOSURE`; config theo bước giữ `submitted_at IS NULL`. Có test |
| DB-03 / PERF-04 | Medium | `ListMineRecords` quét toàn bảng, và nay mỗi record Draft còn chạy thêm subquery | Thêm lọc `dr.company_id IN (công ty của các membership)`, không đổi nghĩa vì task và assignment đều theo công ty. Điều kiện trạng thái đặt cuối. Test MySQL dùng membership thật |
| DB-07 / PERF-02 | Low/Medium | Đường skip ghi đè vô điều kiện, có thể đè lên dòng SENT của lần dispatch chạy song song | `DispatchResultInput.OnlyIfDispatchable`: chỉ ghi khi còn PENDING/RETRY_SCHEDULED. Áp cho cả `COMPANY_INACTIVE` và `OBLIGATION_CLOSED`. Có test MySQL |
| DB-08 | Low | Comment cũ; helper ngày gọi `LoadLocation` ở mỗi query | Đã sửa comment; location chỉ nạp một lần |
| API-04 | Low | Copy FE còn "khoảng đã chọn" ở khối deadline health; tooltip còn PENDING_CONFIRM; đường fallback FE vẫn lọc các danh sách mở theo range | Đã sửa copy (vi/en). Fallback bỏ lọc range cho Overdue/Due Soon/Upcoming; Pending Confirm (ứng viên hoàn thành) giữ range |
| API-05 | Low | `docs/api-contracts-json.md` chưa có MR-3b | Thêm mục "ObligationOpen in every read model and deadline reminders (MR-3b)" và sửa định nghĩa ở mục MR-3 |

Kiểm ngược sau khi sửa (tắt từng hành vi thì test fail; khôi phục file thì pass):
- M7: `NotStarted` mất quy tắc cũ;
- M8: config theo bước bị áp cổng mới;
- M9: skip ghi đè dòng SENT;
- M10: lọc công ty sai.

## Chấp nhận, ghi follow-up
| ID | Mức | Nội dung | Lý do |
|---|---|---|---|
| DB-01 / API-01 / API-02 | Medium | Record ad-hoc do finalize tạo không có `planned_date` (hạn nằm ở proposal), nên: (1) không có nhắc hạn nộp; (2) dashboard CMS xếp vào "Chưa có ngày hạn", không vào quá hạn | Sửa tận gốc là ghi hạn ad-hoc đã resolve vào `planned_date` lúc finalize, kèm backfill. Việc này gắn với DUE-06 (hạn bỏ qua `final_t0_date`) và cần duyệt ghi dữ liệu DEV. Không phải regression: trước MR-3b các mục này không được đếm, cũng không được nhắc. DEV: 5 record ad-hoc đã duyệt, 0 `reminder_configs`. Test ghi rõ giới hạn (`adhocNoDate`) |
| PERF-01 | **High, có từ trước** | Bản bị skip vì lý do khác (template tắt, không có người nhận, tier/rule, binding) vẫn ở PENDING. Nếu đủ 50 bản thì chặn hàng đợi `ORDER BY scheduled_at LIMIT 50` toàn nền tảng, trái v1.2 §7 ("Bản bị skip không chặn hàng đợi") | Ngoài MR-3b; MR-3b giảm được phần `OBLIGATION_CLOSED`. DEV hiện 0 bản PENDING (261 bản SENT). Follow-up riêng: đưa skip vĩnh viễn về trạng thái kết thúc, skip theo khung giờ chuyển sang RETRY_SCHEDULED |
| PERF-03 | Medium | Cổng OpenAt theo ngày HCM có thể dồn các mốc nhắc trước kỳ mở | Thực tế không xảy ra: record periodic chỉ được tạo khi kỳ đã mở (`ListPendingCycles` cắt theo hôm nay), và config nhắc gắn theo record nên chỉ có sau đó. Mốc đã qua được tạo ngay, giống hệt trước MR-3b |
| DB-04 / PERF-06 | Low | Nhánh ad-hoc không có index `(company_id, record_id)`; `INSERT…SELECT` dưới REPEATABLE READ giữ khoá đọc mỗi tick | DEV có 8 proposal. Follow-up: migration index online, chạy materialize ở READ COMMITTED |
| PERF-05 | Low | Subquery trong SELECT của `ListDispatchCandidates` có thể chạy cho cả backlog | Tuỳ plan; phần join có sẵn tốn hơn. Follow-up: lọc theo 2 pha |
| DB-06 | Low | Khoá do `INSERT IGNORE … SELECT` | Như DB-04 |
| API-03 | Low | Lệch phiên bản: API cũ đếm `OBLIGATION_CLOSED` là lỗi | Runbook: deploy api và worker cùng lúc (`deploy-dev.sh be` làm đúng vậy). Khi rollback, tắt cảnh báo `ReminderFailedRecent` 15 phút |
| DB-09 | Info | Ad-hoc có workflow bị reject: record vẫn `PendingReview`, nên mở mãi | Câu hỏi PO (bên dưới) |
| API-07, PERF-07 | Info | Ánh xạ milestone là code phòng thủ; mỗi lần đóng vẫn bắn `REMINDER_SLA_BREACH` | Ghi nhận |

## Blast radius trên DEV (chỉ đọc)
- **Tab Cảnh báo:** c_001 thêm 5 ad-hoc đã duyệt.
- **"Việc của tôi":** thêm 332 task đang chờ của record Draft đang mở, gồm cả task của 312 hồ sơ QA thuộc mẫu đã lưu trữ (đã chốt ở GATE A của MR-3, dọn ở MR-8).
- **Dashboard Portal:** KPI mở giống nhau ở mọi range.
- **Reminder:** 0 `reminder_configs` và 0 bản nhắc DISCLOSURE, nên không có thư mới hay bản bị đóng.
- **Dữ liệu:** không cần sửa (MR-3b chỉ đổi cách đọc, không ghi dữ liệu).

## Câu hỏi cho PO
1. Nhắc theo bước workflow có dừng khi record hết `ObligationOpen` không (v1.2 §2.2 so với §7)? DEV có 134 milestone đang chờ của record đã nộp mà workflow còn chạy. MR-3b giữ nguyên.
2. Record `NotStarted` (bản ghi toàn cục do CMS đẩy cho công ty) có là nghĩa vụ mở không? Nếu có, chúng phải hiện ở tab Cảnh báo, dashboard và "Việc của tôi". Hiện tại chúng chỉ được nhắc hạn như cũ.
3. Ad-hoc có workflow bị reject: đóng nghĩa vụ hay để mở (hiện là mở mãi)?
