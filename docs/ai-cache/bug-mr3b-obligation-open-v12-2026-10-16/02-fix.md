# 02: Sửa (Phase 3) và kiểm chứng (GATE V)

## Sửa tại nguyên nhân gốc
| # | File | Thay đổi |
|---|---|---|
| 1 AC-11 | `internal/disclosure/app/obligation_open.go` | `ObligationOpenSQL` thêm nhánh ad-hoc: `status NOT IN ('approved','published','completed','done') AND EXISTS (ad_hoc_proposals cùng company/record, status 'approved')`. Vẫn một bind (todayHCM). Các nơi đã dùng predicate tự nhận nhánh này: list/count/snapshot cảnh báo, dashboard Portal, dashboard CMS, archive-impact |
| | `internal/deadlinealerts/infra/mysql/list_rows_membership.go` | Bản Go song song thêm tham số `approvedAdHoc` |
| 2 AC-13 | `internal/personalops/infra/mysql/repository.go` | `ListMineRecords`, `ListMineOpenTasks`: `dr.status <> 'draft'` thành `(dr.status <> 'draft' OR ObligationOpen)`, đặt cuối WHERE. `ListMineRecords` thêm lọc theo công ty của membership (review DB-03/PERF-04). Record đã qua Draft giữ như cũ; thêm record Draft đang là nghĩa vụ mở. Không thay thẳng bằng `ObligationOpen` như PO-01 ghi, vì làm vậy sẽ bỏ record đã hoàn thành (KPI hoàn thành, tỉ lệ đúng hạn của "Việc của tôi") và task của record đã nộp mà workflow còn chạy |
| 3 Q-18 | `internal/portaldashboard/app/service.go` | Bucket mở (OVERDUE, DUE_SOON, UPCOMING, PENDING_CONFIRM) không lọc range. Range chỉ chọn ứng viên hoàn thành: DONE trong `fetchDeadlines`, PENDING_CONFIRM trong `fetchCompletion` |
| | `cobo_web_design` `src/features/portal-dashboard/i18n/dashboardOperationalText.ts`, `hooks/useOperationalDashboardData.ts` | Chú thích KPI "Cảnh báo đang xử lý" (bỏ PENDING_CONFIRM), empty state của khối deadline health và 2 panel rủi ro: bỏ "trong khoảng đã chọn" (vi/en); cập nhật 2 test. Đường fallback của FE bỏ lọc range cho Overdue/Due Soon/Upcoming |
| 4 reminder | `internal/reminder/infra/mysql/repository.go` | Nhắc hạn nộp (config `DISCLOSURE`) chỉ được tạo khi record qua cổng `reminderDeadlineOpenSQL` = `ObligationOpen`, hoặc `NotStarted` chưa nộp (giữ quy tắc cũ, chờ PO). Config theo bước giữ `submitted_at IS NULL`. `ListDispatchCandidates` trả cờ `obligation_closed` theo cùng cổng |
| | `internal/reminder/app/service.go`, `contracts.go`, `infra/inmemory/repository.go` | Lần nhắc DISCLOSURE có cờ đóng được đánh `FAILED + OBLIGATION_CLOSED` (kết thúc, giống `COMPANY_INACTIVE`) và không gửi. Hai đường skip chỉ ghi khi bản nhắc còn PENDING/RETRY_SCHEDULED (`OnlyIfDispatchable`), không đè dòng SENT |
| | `internal/reminder/infra/observe/db_collector.go`, `internal/platformcms/app/dashboard_overview_mysql.go`, `internal/workflow/infra/mysql/milestones.go` | `OBLIGATION_CLOSED` không tính là lỗi (metric, dashboard CMS); API milestone hiện là "cancelled" |

Không đổi: nhắc theo bước workflow (xem `01-root-cause.md`), schema, response shape.

**Giới hạn còn lại:** record ad-hoc do finalize tạo không có `planned_date`, nên:
- chưa có nhắc hạn nộp;
- dashboard CMS xếp chúng vào "Chưa có ngày hạn".

Follow-up cùng DUE-06 (`03-review.md`).

## Test mới hoặc mở rộng (gồm cả sau review)
- MySQL 8:
  - `TestObligationOpen_ApprovedAdHocIsOpenUntilItsWorkflowCompletes`;
  - `TestMineObligation_TasksOfOpenDraftRecordsAreMine`;
  - `TestReminderObligation_DeadlineRemindersFollowObligationOpen` (+NotStarted, config theo bước, ad-hoc không có `planned_date`);
  - `TestReminderObligation_SkipNeverOverwritesASentOccurrence`.
- Unit:
  - `TestOverview_openKPIsAreACurrentSnapshotWhateverTheRange`;
  - `TestDispatchDue_ClosedObligationDeadlineReminderClosedWithoutSending`;
  - ma trận `TestListRowsV1Membership_acceptanceMatrix` (+3 dòng ad-hoc);
  - `TestObligationOpenSQL_isRecordStateOnly` kiểm thêm nhánh ad-hoc.

## GATE V
- Cả 4 test tái hiện pass sau khi sửa.
- Kiểm ngược: tắt từng chỗ sửa thì test fail (bằng assertion, không phải lỗi build). Khôi phục file thì pass.
  - M1: bỏ nhánh ad-hoc;
  - M2: "Việc của tôi" lại loại Draft;
  - M3: dashboard lọc mọi bucket theo range;
  - M4: nhắc hạn quay lại `submitted_at IS NULL`;
  - M5: dispatch bỏ qua cờ đóng;
  - M6: cờ đóng luôn 0;
  - sau review, M7–M10: xem `03-review.md`.

## Kiểm chứng cuối (sau review)
| Lệnh | Kết quả |
|---|---|
| MySQL 8 thật (`cobo_iam_mr1`): `ObligationOpen` (3), `MineObligation`/`OpenTasksQuery` (2), `ReminderObligation` (2) | Pass |
| Kiểm ngược M1–M10 | Cả 10 lần đều có test fail; file khôi phục giống hệt bản lưu |
| `go test ./...` so với `3031d8f` | 0 fail mới (cùng tập fail có sẵn: 14 test, 5 package) |
| `go vet ./...` | Sạch (cảnh báo có sẵn ở `workflowfulfillment`) |
| `docker compose -f docker-compose.dev.yml build api` | Built |
| FE (`cobo_web_design`): `npx vitest run`, `npm run lint`, `npm run build`, `npm run check:mojibake` | 48 fail có sẵn, trùng tên với HEAD (3248 pass); 12 lỗi TS giống HEAD; build OK; mojibake OK |
| gofmt | Các file đã sửa vốn sạch ở HEAD thì vẫn sạch. `reminder/app/service.go` và `observe/db_collector.go` đã lệch gofmt từ HEAD, không format lại cả file |
