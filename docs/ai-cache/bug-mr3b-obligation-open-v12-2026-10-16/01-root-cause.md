# 01: Tái hiện (GATE R) và nguyên nhân gốc

## GATE R: 4 test fail trước khi sửa
| # | Test | Fail trước khi sửa |
|---|---|---|
| 1 | `TestObligationOpen_ApprovedAdHocIsOpenUntilItsWorkflowCompletes` (MySQL 8, deadlinealerts) | Record ad-hoc `PendingReview` có proposal `approved` không có trong danh sách (`listed=false, want true`) |
| 2 | `TestMineObligation_TasksOfOpenDraftRecordsAreMine` (MySQL 8, personalops) | Task của record Draft đang mở (irregular và periodic đã tới kỳ) không có trong "Việc của tôi" lẫn "bản ghi của tôi" |
| 3 | `TestOverview_openKPIsAreACurrentSnapshotWhateverTheRange` (unit, portaldashboard) | Cả range 7d và 30d: overdue=1, upcoming=0 (đúng là 2/1). `needs_action_now` 2 (đúng 3), `pending_approval` 2 (đúng 4), `blocked_or_exception` 1 (đúng 2) |
| 4 | `TestReminderObligation_DeadlineRemindersFollowObligationOpen` (MySQL 8, reminder) | Ad-hoc đã duyệt không được nhắc. Draft có kỳ chưa mở vẫn bị nhắc. Lần nhắc của record đã nộp không bị đánh dấu đóng |

## Nguyên nhân gốc (đã xác minh bằng code và test trên)
1. **AC-11, tầng read model (SQL).** `disclosure/app.ObligationOpenSQL` chỉ có nhánh `draft AND submitted_at IS NULL`. Finalize ad-hoc gọi `SubmitRecord` (`adhoc/infra/disclosure/record_creator.go`), nên record thành `PendingReview` và có `submitted_at`. Định nghĩa v1.2 cho ad-hoc (proposal `approved` và record chưa hoàn tất) chưa từng được cài.
2. **AC-13, tầng repository.** `personalops/infra/mysql/repository.go:116,249` lọc `dr.status <> 'draft'`, có từ trước v1.2, khi record Draft còn bị coi là "chưa vào việc". Nhưng record periodic giữ trạng thái Draft suốt lúc workflow chạy.
3. **KPI, tầng service.** `portaldashboard/app/service.go:136` bỏ mọi mục có hạn ngoài `[From, To]` trước khi chia bucket. Preset kết thúc ở hôm nay, nên UPCOMING không bao giờ được đếm, và OVERDUE có hạn trước ngày đầu range bị rơi. Amendment Q-18 chưa được cài ở BE (doc FE `06-approved-kpi-formulas.md` ghi "#1/#5 already range-free", nhưng code thì không).
4. **Reminder, tầng repository/worker.**
   - `MaterializeDueOccurrences` dùng `dr.submitted_at IS NULL`, một định nghĩa thứ hai, khác `ObligationOpen`: không xét status, không có nhánh ad-hoc, không có cổng `OpenAt`.
   - `ListDispatchCandidates` không đọc lại trạng thái record, nên lần nhắc đã tạo vẫn được gửi sau khi record được nộp.

## Lớp
SQL read model (1, 2, 4), service (3), worker (4). Không đụng auth, tenant scope, cache hay migration.

## Kết luận khi đọc code nhắc theo bước (không sửa)
- Nhắc theo bước workflow (scope `WORKFLOW_STEP`) xét trạng thái instance và bước, không xét `ObligationOpen`.
- DEV có 134 milestone đang chờ của record đã nộp (`PendingReview`, không phải ad-hoc) mà instance còn `in_progress`. Nếu áp `ObligationOpen` cho nhắc theo bước, những người duyệt nội bộ này sẽ thôi được nhắc.
- Contract v1.2 không nói rõ trường hợp này: §2.2 nói "reminder" dùng `ObligationOpen`, còn §7 nói nhắc theo bước hiện tại. MR-3b chỉ áp cho nhắc hạn nộp. Câu hỏi cho PO ghi ở `03-review.md`.
