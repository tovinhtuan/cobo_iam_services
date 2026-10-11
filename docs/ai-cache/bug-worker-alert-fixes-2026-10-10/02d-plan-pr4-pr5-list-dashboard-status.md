# PR-4 (danh sách hạn + dashboard) và PR-5 (trạng thái company + kênh thông báo): kế hoạch chi tiết (wf-bugfix Phase 3–6, chỉ lập kế hoạch)

> **Cập nhật sau khi PO chốt (2026-10-11, Contract v1.2 §2.2). Các quyết định dưới đây thay thế khuyến nghị cũ trong file này:**
> - **DLA-04 (Q-19):** KPI hoàn thành đọc trực tiếp `disclosure_records` có hạn trong range, với `CompletedAt = COALESCE(dac.confirmed_at, dr.submitted_at [periodic/thủ công], completed_at/updated_at khi ad-hoc đạt Approved/Published/Completed)`. Không suy từ danh sách cảnh báo.
>   - Bỏ PENDING_CONFIRM: xoá nhánh này khỏi `resolveDueDateAndStatus` và dashboard. FE giữ mapping cũ cho tương thích, vì không còn giá trị nào được trả về.
>   - Ngưỡng màu đúng hạn: ≥95 success, 85–95 warning, <85 critical.
> - **DLA-03 (Q-18):** KPI #3 cũng thành ảnh chụp hiện tại (không theo range).
> - **PO-01 (mới, HIGH), thêm vào PR-4 BE commit 1b:** `personalops/infra/mysql/repository.go:115,246` (`ListMineRecords`, `ListMineOpenTasks`) bỏ `<> 'draft'` và thay bằng predicate `ObligationOpen`.
>   - Test: `TestMineQueries_IncludeDraftObligations` (source guard) + integration "task của record Draft hiện trong mine". AC-13.
> - **DLA-11 (Q-20):** trả 200 kèm người xác nhận đầu tiên.
> - **DLA-12 (Q-21):** được phép sửa `src/types.ts`.
> - **RMC-05 (Q-22):** inactive dừng; suspended vẫn tạo kỳ và vẫn nhắc. Email của company suspended thêm một dòng thông báo trạng thái (follow-up nhỏ).
> - **RMC-07 (Q-23):** thiếu row prefs thì dùng mặc định.
> - **RMC-08 (Q-24):** lịch gửi của company chỉ gate nhắc hạn nộp.
> - **Q-17:** nhân viên phòng ban thấy cảnh báo của bước thuộc phòng mình. Làm P1 sau PR-4 (cần thiết kế hiệu năng).

> **Tóm tắt (VI)**
> - **Baseline:** không cần đổi schema; `make deploy-be` là đủ. Đã chạy `go test ./internal/deadlinealerts/... ./internal/portaldashboard/... ./internal/reminder/app/...` và Vitest `deadlineAlertsApi`/`deadlineAlertUrgencyPresentation` (20/20) trên HEAD: đều pass.
> - **Test tái hiện:** một số test chỉ fail ở dạng "compile-fail", vì bản sửa thêm field/tham số mới.
> - **Hoà giải với PR-3:** ở DLA-02, **không** tự thêm `ORDER BY wi.workflow_instance_id ASC` vào `listCurrentStepMeta`. Thay vào đó dùng `workflowsel.PreferredInstanceSubquery` (PR-3 commit 5). PR-4 rebase lên PR-3 commit 5.
> - **Quyết định cần chốt trước khi code:**
>   - PR45-D2: KPI mở theo trạng thái hiện tại, range chỉ áp cho KPI hoàn thành. Cần PO ký.
>   - PR45-D3: hoàn thành = `COALESCE(completed_at, confirmed_at)`; ngưỡng màu đúng hạn đối xứng.
>   - PR45-D7: được phép sửa `src/types.ts` (file cần duyệt) không.
>   - PR45-D8: inactive = không nhắc; suspended vẫn seed + nhắc.
>   - PR45-D9: không có row prefs → dùng mặc định.
>   - PR45-D10: lịch gửi của company chỉ gate reminder hạn nộp.
>   - Các quyết định còn lại (D1, D4, D5, D6) dùng khuyến nghị mặc định.
>
> Nguồn: agent lập kế hoạch PR-4/5 (read-only, IAM `a8acb51`, WEB `79853d9d`).

## PR-4: danh sách hạn + dashboard

### DLA-02 Enrich Draft trống; `AllowsRow` lệch với SQL scope; endpoint steps trả 403
- **Mức độ / nguồn:** HIGH. Xác nhận trên DEV: 308 Draft, tất cả có `department_id='general'`.
- **Nguyên nhân gốc:**
  - Membership đã đổi sang `= 'draft'`, nhưng 2 helper enrich vẫn giữ filter cũ `<> 'draft'`: `listTaskAssigneeRecords` (`internal/deadlinealerts/infra/mysql/repository.go:252`) và `listCurrentStepMeta` (`:390`).
  - Vì vậy `HasTaskAssignee` và `CurrentStepDepartment` của mọi dòng trong danh sách luôn rỗng. `AllowsRow` (`app/access_scope.go:107,119`) lại loại các dòng mà nhánh task trong SQL đã nhận (`app/access_scope_sql.go:34`).
  - Đường chi tiết: `steps_service.go:155` trả 403.
  - Phát hiện phụ: `GetAlertRowByRecordID` (`repository.go:484-495`) tải metadata của cả company ở mỗi lần xem chi tiết.
  - Tầng read model DB. Confirmed (code + DEV).
- **Test tái hiện (GATE R):**
  - Unit, file mới `internal/deadlinealerts/infra/mysql/enrich_helpers_test.go`, `TestEnrichHelpers_doNotExcludeDraft`:
    - dùng helper đọc source/func-extract sẵn có (`readDeadlineAlertsRepositorySrc`);
    - `listCurrentStepMeta` và `listTaskAssigneeRecords` không được chứa `<> 'draft'`;
    - `GetAlertRowByRecordID` phải truyền `[]string{recordID}`.
    - HEAD fail.
  - mysql-integration, file mới `list_rows_draft_enrich_integration_test.go`, `TestIntegration_ListRowsPage_DraftTaskAssigneeEnriched` (skip nếu không có DSN):
    - Seed: Draft (`general`, `submitted_at` NULL, không có cycle); instance có bước `s1` thuộc phòng `legal` (đang là bước hiện tại); task pending giao cho `m_staff`.
    - Gọi `ListRowsPage` với scope `{MembershipID:m_staff, DepartmentIDs:[d_other]}`.
    - Kỳ vọng: total=1, `HasTaskAssignee`, có tên phòng ban và tên bước, `AllowsRow`=true.
- **Thiết kế sửa:**
  - Bỏ `AND LOWER(TRIM(dr.status)) <> 'draft'` ở cả 2 helper.
  - `listCurrentStepMeta` chỉ lấy instance ưu tiên của mỗi record bằng `workflowsel.PreferredInstanceSubquery` (từ PR-3 commit 5). Agent đề xuất `ORDER BY … ASC` và giữ dòng đầu, nhưng cách đó sẽ chọn nhầm instance template, nên **không dùng**.
  - `GetAlertRowByRecordID`: gọi cả 3 helper với `[]string{recordID}`.
  - Sửa comment test "helpers may still use it" (`list_rows_membership_test.go:41`).
  - Không đổi: SQL membership, `BuildListRowsScopeSQL`, logic `AllowsRow`, cache effective access.
- **Contract / schema / FE:**
  - Response cùng shape; các cột có dữ liệu; người được giao task thấy dòng của mình.
  - FE cũ chạy với BE mới: OK. Không đổi schema.
- **Dữ liệu:** không.
- **Rollout / rollback:** chỉ BE; rollback bằng revert.
- **Verify (GATE V):**
  - `go test ./internal/deadlinealerts/... -count=1`
  - `MYSQL_TEST_DSN=… go test ./internal/deadlinealerts/infra/mysql -run Integration -count=1`
  - Smoke DEV, ENT: `GET /api/v1/company/deadline-alerts?page=1&page_size=20` có `active_departments` và `current_step_name`.
  - Smoke DEV, MEMBER: nếu `run_sql` qa_ro cho thấy `m_qa_member_c001` có task đang mở thì dòng phải hiển thị, `total` = số item, và `GET /api/v1/company/deadlines/{id}/steps` trả 200. Nếu MEMBER không có task thì ghi BLOCKED cho tới khi fixture được duyệt.
- **Phạm vi ảnh hưởng:**
  - Nhánh phòng ban của bước trong `AllowsRow` giờ có tác dụng với Draft ở endpoint chi tiết/steps. Hành động trên bước vẫn cần `deadline.confirm`.
  - Đường full (snapshot/dashboard): helper giờ trả khoảng N dòng kèm `snapshot_json` thay vì gần 0. DEV có 77–308 dòng, rủi ro hiệu năng thấp. Đường phân trang vẫn tối đa 100 id.
- **Quyết định PR45-D1:** nhân viên phòng ban không phải người được giao vẫn không thấy record có bước hiện tại thuộc phòng ban mình, vì SQL không có nhánh "bước thuộc phòng ban".
  - (a) giữ SQL làm cổng — khuyến nghị.
  - (b) thêm nhánh dùng bảng ánh xạ phòng ban — cần JSON trong SQL, có rủi ro hiệu năng; để follow-up.
- **Ước lượng:** M

### DLA-03 Preset dashboard kết thúc ở hôm nay: không bao giờ đếm UPCOMING, làm rơi OVERDUE cũ
- **Mức độ / nguồn:** HIGH. DEV: mỗi company mất 26 UPCOMING và 6 OVERDUE.
- **Nguyên nhân gốc:**
  - Mọi preset đặt `to=today` (`portaldashboard/domain/range.go:55-67`).
  - `fetchDeadlines` loại item có hạn ngoài `[from,to]` **trước khi** chia nhóm (`app/service.go:136`).
  - Công thức đã duyệt (WEB `06-approved-kpi-formulas.md`): #1 (cần xử lý) và #5 (tổng OVERDUE) không có range; #2 có cửa sổ riêng; chỉ KPI hoàn thành #4/#6 gắn với range.
  - Tầng service. Confirmed.
- **Test tái hiện (GATE R, unit):** `internal/portaldashboard/app/deadline_snapshot_test.go` `TestFetchDeadlines_7dCountsFutureUpcomingAndOldOverdue`:
  - now = 2026-10-10 08:00Z, preset 7d.
  - Items:
    | Trạng thái | Hạn |
    |---|---|
    | OVERDUE | 09-20, 10-08 |
    | DUE_SOON | 10-10 |
    | UPCOMING | 10-15, 11-30 |
    | DONE | 09-01, 10-05 |
  - Kỳ vọng: overdueTotal 2, upcomingTotal 2, dueSoon 1, dueIn7 2, `len(done)` 1. Qua `buildOverview`: `pending_approval`=5, `blocked_or_exception`=2.
  - HEAD cho 1 và 0.
- **Thiết kế sửa (chỉ trong `fetchDeadlines`):**
  - OVERDUE, DUE_SOON, UPCOMING, PENDING_CONFIRM: chia nhóm không áp range.
  - DONE: vẫn lọc `dashboardDateInRange(due, From, To)`.
  - Thêm `out.completionCandidates` = DONE + PENDING_CONFIRM nằm trong range, không giới hạn số lượng; `fetchCompletion` đọc từ danh sách này.
  - Không đổi `ParseRange`, field `range` của response, cửa sổ 7 ngày.
- **Contract / schema / FE:**
  - Giá trị đổi, shape giữ nguyên; FE cũ chạy được.
  - Tooltip `pendingApprovalTooltip` của FE ("trong khoảng đang chọn") sẽ sai, sửa ở commit WEB W2.
- **Dữ liệu:** không.
- **Rollout / rollback:** chỉ BE; rollback bằng revert.
- **Verify (GATE V):**
  - `go test ./internal/portaldashboard/... -count=1`
  - Smoke DEV, ENT: `GET /api/v1/company/dashboard/overview?range=7d` và `?range=30d`. Các KPI mở (`needs_action_now`, `pending_approval`, `blocked_or_exception`) phải bằng nhau ở 2 range; `pending_approval` tăng khoảng 26 so với trước deploy.
- **Phạm vi ảnh hưởng:** nhóm tuổi quá hạn, phòng ban rủi ro, luồng hay trễ giờ tính trên mọi item đang mở. Giới hạn 100 item mỗi trạng thái vẫn giữ.
- **Quyết định PR45-D2:** KPI mở theo trạng thái hiện tại; range chỉ áp cho KPI hoàn thành. Khuyến nghị, nhưng cần PO ký vì công thức #3 ghi "trong khoảng".
- **Ước lượng:** S

### DLA-04 KPI hoàn thành luôn 0; PENDING_CONFIRM không thể xảy ra; màu đúng hạn bị đảo
- **Mức độ / nguồn:** HIGH. DEV: `completed_at` = 0 ở mọi record.
- **Nguyên nhân gốc:**
  - Danh sách chỉ có Draft, mà Draft không bao giờ có `completed_at` (`disclosure/app/completion.go:20-31`).
  - Hành động hoàn tất thật của tenant là confirm (DC-B), nhưng `MapCompletedAt` bỏ qua nó (`portaldashboard/infra/mysql/completed_at.go:44-50`).
  - `severityFromOnTimeRate` dùng lại thang của tỉ lệ quá hạn (`aggregate.go:318-338`), nên 100% đúng hạn hiện "critical".
  - Confirmed.
- **Test tái hiện (GATE R):**
  - Unit `aggregate_test.go` `TestSeverityFromOnTimeRate_highRateIsGood`: (100,4) → `success`; (85,20) → `warning`; (40,10) → `critical`. HEAD fail.
  - Unit (compile-fail) `completion_test.go` `TestComputeCompletion_confirmedDraftCounts`:
    - Item DONE có `ConfirmedAt` 10-04T03:00Z và 10-06T18:00Z, cùng hạn 10-05; completed map rỗng.
    - Kỳ vọng: tổng 2, đúng hạn 1, trễ 1.
- **Thiết kế sửa:**
  - `deadlinealerts/app/contracts.go`: thêm `DeadlineAlertDTO.ConfirmedAt *time.Time \`json:"-"\``, gán trong `enrichDeadlineAlertRows` (`service.go:212`) từ `row.ConfirmedAt`.
  - `computeCompletionFromAlerts`: ưu tiên `completedAt[id]`, nếu không có thì dùng `*a.ConfirmedAt`, tức `COALESCE(completed_at, confirmed_at)`.
  - `severityFromOnTimeRate = severityFromRate(100-pct)`; mẫu 0 vẫn `neutral`.
  - Không thêm SQL mới.
- **Contract / schema / FE:**
  - Field mới không gửi ra ngoài.
  - FE render `severity` do BE trả (`dashboardOverviewFromApi.mapper.ts:106`), nên tự đúng. FE cũ chạy được.
- **Dữ liệu:** không.
- **Rollout / rollback:** chỉ BE; rollback bằng revert.
- **Verify (GATE V):**
  - `go test ./internal/portaldashboard/... ./internal/deadlinealerts/... -count=1`
  - Smoke DEV: `completed_total` = `run_sql` qa_ro đếm `deadline_alert_confirmations` join Draft có hạn trong range. `on_time_rate.severity` không còn `critical` khi tỉ lệ cao.
- **Phạm vi ảnh hưởng:** chỉ dashboard; `on_time_rate` của personal-ops giữ nguyên.
- **Quyết định PR45-D3:**
  - Sự kiện hoàn thành = `COALESCE(completed_at, confirmed_at)` (khuyến nghị).
  - Ngưỡng đúng hạn là đối xứng của ngưỡng quá hạn (khuyến nghị).
  - PENDING_CONFIRM tạm thời vẫn không đạt được (khuyến nghị); muốn đạt được phải đổi membership, quyết cùng AHA-01 ở PR-3. Nếu PR-3 chọn phương án (a), ad-hoc Published/Completed sẽ tự có PENDING_CONFIRM.
- **Ước lượng:** M

### DLA-06 `total` của đường phân trang không khớp số item
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:**
  - `total` là `COUNT(*)` của SQL (`repository.go:172-190`), còn item bị lọc thêm bởi `AllowsRow` và `dueDate==""` (`app/service.go:185-193`).
  - Nguyên nhân có hệ thống trên DEV là DLA-02.
  - Phần còn lại: cache effective access 5 phút so với `assignments` thật; dòng không tính được hạn.
  - Confirmed.
- **Test tái hiện (GATE R):** integration test của DLA-02 kiểm thêm `total == len(items)` cho trang của người được giao. Unit không tái hiện được, vì stub đặt `HasTaskAssignee` trực tiếp.
- **Thiết kế sửa:**
  - Dựa vào DLA-02 để bỏ nguyên nhân có hệ thống.
  - `listUnfilteredDeadlineAlertPage`: thêm `if d := len(rows)-len(items); d>0 { slog.WarnContext(…company_id, page, dropped) }`.
  - Ghi vào `docs/api-contracts-json.md`: `total` có thể lớn hơn số item truy cập được, phần chênh là các dòng không có hạn.
  - Không đưa đường full-resolution trở lại cho request không lọc (rủi ro 504). Không fallback theo từng trang, vì trang 1 và trang N sẽ báo `total` khác nhau.
- **Contract / schema / FE:** chỉ ghi tài liệu ngữ nghĩa. FE cũ: trang cuối rỗng đã có empty state.
- **Dữ liệu:** không.
- **Rollout / rollback:** chỉ BE; rollback bằng revert.
- **Verify (GATE V):** như DLA-02. Smoke DEV: ENT và MEMBER, nếu chỉ có 1 trang thì `total` = số item, và không có log WARN.
- **Phạm vi ảnh hưởng:** thấp.
- **Quyết định PR45-D4:** đếm xấp xỉ chính xác hơn bằng cách thêm vào `countListRows` điều kiện `planned_date IS NOT NULL OR` "type có deadline config". Tuỳ chọn, chưa khuyến nghị.
- **Ước lượng:** S

### DLA-07 ORDER BY bị hoà nên phân trang không ổn định
- **Mức độ / nguồn:** MEDIUM. DEV có 47 nhóm Draft trùng `created_at` đến từng giây, nhóm lớn nhất 14 dòng.
- **Nguyên nhân gốc:** `ORDER BY dr.created_at DESC` (`repository.go:122`) không có khoá duy nhất, mà lại dùng với LIMIT/OFFSET. Confirmed.
- **Test tái hiện (GATE R):**
  - Unit `list_rows_membership_test.go` `TestListRows_orderByHasUniqueTieBreaker`: `listRowsFuncSource` phải chứa `dr.record_id DESC`. HEAD fail.
  - Integration (skip nếu không có DSN): 14 dòng cùng `created_at`, page size 5; gộp trang 1–3 phải ra đúng 14 id khác nhau.
- **Thiết kế sửa:** `ORDER BY dr.created_at DESC, dr.record_id DESC`.
- **Contract / schema / FE:** không; thứ tự trở nên tất định.
- **Dữ liệu:** không.
- **Rollout / rollback:** chỉ BE; rollback bằng revert.
- **Verify (GATE V):** smoke DEV, ENT: duyệt mọi trang với `page_size=5`, so với số Draft đủ điều kiện từ `run_sql`; không trùng, không thiếu `record_id`.
- **Phạm vi ảnh hưởng / hiệu năng:** query vốn đã filesort (index duy nhất là `(company_id, status)` và bị vô hiệu bởi `LOWER(TRIM())`), nên thêm khoá phụ không đổi chi phí.
- **Quyết định:** không.
- **Ước lượng:** S

### DLA-09 Cycle đã qua `open_at` nhưng chưa materialize thì không hiện ở đâu
- **Mức độ / nguồn:** MEDIUM.
- **Nguyên nhân gốc:**
  - `ListNextAlertCycles` chỉ lấy cycle mở trong tương lai (`repository.go:807`, `next_alerts.go:85`).
  - Danh sách chính chỉ hiện record đã materialize.
  - Cycle ở trạng thái PENDING/RETRY/CLAIMED/FAILED có `record_id IS NULL` và `open_at <= today` vì vậy không hiện ở đâu.
  - Confirmed (code); chưa biết số lượng trên DEV.
- **Test tái hiện (GATE R):**
  - Query script read-only trên DEV qua qa_ro:
    ```sql
    SELECT company_id, materialization_state, COUNT(*)
    FROM periodic_cycles
    WHERE record_id IS NULL AND COALESCE(open_at,cycle_start) <= <todayHCM>
    GROUP BY 1,2;
    ```
  - Unit test collector (compile-fail).
- **Thiết kế sửa (phương án A):**
  - File mới `internal/disclosure/infra/observe/periodic_cycle_collector.go`, chép theo mẫu `reminder/infra/observe/db_collector.go`.
  - Gauge `cobo_periodic_cycles_unmaterialized_past_open{state}`: `GROUP BY materialization_state`, dùng index `idx_pc_pending(record_id,…)`, timeout 2s.
  - Đăng ký trong `internal/httpserver/server.go` cạnh collector của reminder (khoảng dòng 820) và thêm alert rule.
  - Không đổi contract của next-alerts.
- **Contract / schema / FE:** không.
- **Dữ liệu:** chạy lại cycle FAILED chỉ qua `cmd/periodic-materialize-one` có guard, và cần cho phép.
- **Rollout / rollback:** deploy API; rollback bằng revert.
- **Verify (GATE V):** `go test ./internal/disclosure/infra/observe/...`; `/metrics` có gauge mới.
- **Phạm vi ảnh hưởng:** chỉ `/metrics`.
- **Quyết định PR45-D5:** hiển thị cho tenant là một tính năng (thêm `pending_items` tuỳ chọn vào `/deadline-alerts/next` + một phần FE), cần làm contract-first ở follow-up. Hiện tại khuyến nghị chỉ làm phương án A.
- **Ước lượng:** S

### DLA-11 Xác nhận lần hai ghi đè người xác nhận đầu tiên
- **Mức độ / nguồn:** LOW.
- **Nguyên nhân gốc:** `ON DUPLICATE KEY UPDATE confirmed_by/at/note=VALUES(…)` (`repository.go:602-607`), và service trả lại chính người gọi là người xác nhận (`service.go:407-412`). Confirmed (code).
- **Test tái hiện (GATE R):**
  - Unit (kiểm source) `TestConfirmDeadlineAlert_firstWriterWins`: SQL không được gán `confirmed_by`. HEAD fail.
  - Integration `TestIntegration_ConfirmTwice_KeepsFirst`: membership A xác nhận, rồi B xác nhận; row và response đều phải là A.
- **Thiết kế sửa:**
  - SQL: `ON DUPLICATE KEY UPDATE id = id`, rồi `SELECT confirmed_by, confirmed_at`.
  - Repo: `ConfirmDeadlineAlert(...) (by string, at time.Time, err error)`.
  - Service trả về giá trị đã lưu.
  - Cập nhật `stubRepo`, repo in-memory, các stub dùng ở màn chi tiết.
- **Contract / schema / FE:** cùng shape, cùng 200, nhưng là người xác nhận đầu tiên. FE cũ chạy được.
- **Dữ liệu:** lịch sử đã bị ghi đè không khôi phục được.
- **Rollout / rollback:** chỉ BE; rollback bằng revert.
- **Verify (GATE V):** smoke DEV: ENT xác nhận rồi ENT2 xác nhận cùng record → response của ENT2 hiện membership của ENT. Chỉ làm với record của QA, và cần cho phép vì có ghi dữ liệu.
- **Phạm vi ảnh hưởng:** chỉ endpoint confirm.
- **Quyết định PR45-D6:** trả 200 với giá trị gốc (khuyến nghị) hay trả 409 (trải nghiệm kém hơn trên FE cũ).
- **Ước lượng:** S

### DLA-12 (WEB) Status lạ hiển thị "Trong hạn"; nhãn DUE_SOON gây hiểu nhầm
- **Mức độ / nguồn:** LOW.
- **Nguyên nhân gốc:**
  - `normalizeDeadlineAlertStatus` mặc định trả `'Upcoming'` (`src/services/deadlineAlertsApi.ts:61-62`), hiển thị là "Trong hạn" (`deadlineAlertUrgencyPresentation.ts:15`).
  - BE chỉ trả DUE_SOON khi còn 0 ngày (`app/status.go:43-44`), nhưng FE ghi nhãn "Sắp đến hạn".
  - Confirmed.
- **Test tái hiện (GATE R, Vitest):**
  - `src/services/deadlineAlertsApi.test.ts`: "unknown status does not map to Upcoming". HEAD fail.
  - `deadlineAlertUrgencyPresentation.test.ts`: `getDeadlineAlertUrgency('Due Soon','vi').label` phải là `'Đến hạn hôm nay'`. HEAD fail.
- **Thiết kế sửa:**
  - Đổi nhãn 'Due Soon' thành "Đến hạn hôm nay" (VI) / "Due today" (EN) ở các file:
    - `deadlineAlertUrgencyPresentation.ts`
    - `pages/portal/deadlineListText.ts:96,170`
    - `deadlines/DeadlineListFilterBar.tsx:18`
    - `dashboardLabels.ts:34`
    - `deadlineDetailOperationalViewModel.ts:113`
    - `dashboardOperationalText.ts:68,75`
  - Giữ nguyên `dueNext7Days`.
  - Status lạ:
    - Thêm `'Unknown'` vào `DeadlineStatus` trong `src/types.ts`. Đây là file được bảo vệ, cần duyệt rõ ràng.
    - Sau đó `npm run lint` sẽ chỉ ra các map `Record<DeadlineStatus,…>` vét cạn (`dashboardLabels.ts`, `deadlineListText.ts`, `deadlineDetailOperationalViewModel.ts`, switch urgency). Thêm nhãn trung tính "Không xác định" cho các map đó.
    - Cập nhật `DeadlineAlertDetailHeader.test.tsx`.
- **Contract / schema / FE:** chỉ FE. FE mới chạy với BE cũ được.
- **Dữ liệu:** không.
- **Rollout / rollback:** `deploy-fe`; rollback bằng revert.
- **Verify (GATE V):**
  - `npx vitest run src/services src/pages/portal/deadlines src/features/portal-dashboard`, `npm run lint`, `npm run test`.
  - Trên trình duyệt: `/app/deadlines` có dòng "đến hạn hôm nay".
- **Phạm vi ảnh hưởng:** nhãn của personal-ops nằm ngoài phạm vi.
- **Quyết định PR45-D7:**
  - Duyệt việc sửa `types.ts`, hoặc chỉ sửa nhãn.
  - Không khuyến nghị nới DUE_SOON thành N ngày ở BE, vì sẽ đổi KPI.
- **Ước lượng:** S

## PR-5: trạng thái company + kênh thông báo

### RMC-05 / AHA-11 Company inactive vẫn nhận nhắc; company suspended không có kỳ mới
- **Mức độ / nguồn:** MEDIUM. Liên quan API-44 ở risk review.
- **Nguyên nhân gốc:**
  - `ListDispatchCandidates` (`reminder/infra/mysql/repository.go:400-450`) và `ListDueMilestones` (`milestone_scanner.go:23-33`) không đọc `companies.status`.
  - Seed lấy `status='active'` (`disclosure/infra/mysql/repository.go:2606`).
  - Trong khi đó commit `5715d6c` định nghĩa suspended là "nghĩa vụ CBTT vẫn áp dụng", và `companystatus.BlocksAccess` chỉ true với inactive.
  - Confirmed.
- **Test tái hiện (GATE R):**
  - Kiểm source, file mới `reminder/infra/mysql/company_status_filter_test.go` và test mysql của disclosure:
    - `TestListDispatchCandidates_excludesInactiveCompany`
    - `TestListAllActiveCompanyIDs_includesSuspended`
    - HEAD fail.
  - Integration (skip nếu không có DSN): mỗi company (inactive, active) có 1 occurrence PENDING; chỉ dòng của company active được trả về.
- **Thiết kế sửa:**
  - `ListDispatchCandidates`: `WHERE ((…PENDING…) OR (…RETRY…)) AND (comp.company_id IS NULL OR LOWER(TRIM(comp.status)) <> 'inactive')`. Dòng mồ côi đi đường cũ.
  - `ListDueMilestones`:
    - thêm `AND NOT EXISTS (SELECT 1 FROM companies c WHERE c.company_id=workflow_step_milestones.company_id AND LOWER(TRIM(c.status))='inactive')`;
    - dùng `FOR UPDATE OF workflow_step_milestones SKIP LOCKED`, để không khoá dòng của `companies`.
  - `ListAllActiveCompanyIDs`: `status IN ('active','suspended')`.
  - Tuỳ chọn: `ListPendingCycles` cũng loại company inactive.
  - Lọc ngay trong SQL thay vì skip, để không gây chặn hàng đợi như REM-05 (dòng bị skip nằm PENDING mãi).
  - Không xoá, không chuyển FAILED các occurrence.
- **Contract / schema / FE:** không.
- **Dữ liệu (nháp, qa_ro):**
  - `SELECT status, COUNT(*) FROM companies GROUP BY status`
  - Đếm `reminder_occurrences` PENDING theo trạng thái company.
  - Dòng của company inactive vẫn nằm PENDING. Nếu company được kích hoạt lại, các dòng này sẽ gửi dồn; điều đó được chặn khi có cận dưới RMC-02 và cơ chế hết hạn REM-05 (PR-1).
- **Rollout / rollback:** worker + API (dispatch nội bộ). Rollback: các kỳ đã seed cho company suspended vẫn còn, vô hại.
- **Verify (GATE V):**
  - `go test ./internal/reminder/... ./internal/disclosure/... -count=1`
  - `EXPLAIN` query dispatch trên DEV qua qa_ro.
  - Smoke: số reminder của company inactive không đổi qua 3 tick. Nếu DEV không có company inactive thì ghi BLOCKED; không tự deactivate company nào.
- **Phạm vi ảnh hưởng:** vòng dispatch của worker. Điều kiện mới chỉ quét nhiều dòng hơn nếu có nhiều dòng PENDING của company inactive.
- **Quyết định PR45-D8:**
  - Khuyến nghị: inactive = không nhắc và không materialize mới; suspended = vẫn seed và vẫn nhắc.
  - Lưu ý: các slot periodic bị bỏ lỡ trong lúc suspended sẽ không được backfill.
  - Phụ thuộc REM-05 (PR-1) để các dòng còn PENDING được cho hết hạn.
- **Ước lượng:** S–M

### RMC-07 / AHA-08 Email và in-app được đánh giá chung; thiếu row prefs bị coi là RULE_MISSING
- **Mức độ / nguồn:** MEDIUM, tiềm ẩn: chỉ xảy ra khi `NOTIFICATION_RULES_CONSUMER_ENABLED=true`. Flag mặc định tắt (`platform/config/notification_rules_consumer.go`); chưa biết giá trị trên DEV.
- **Nguyên nhân gốc:**
  - Runtime chỉ đánh giá `channel="email"` (`dispatch_decision.go:79`), nên prefs "chỉ in-app" ra CHANNEL_DISABLED và không gửi gì.
  - `AllowInApp = emailChannelAllowed` (`:226`) bỏ qua `in_app.enabled`.
  - `doc==nil` → RULE_MISSING (`notification_rules_evaluator.go:36-41`).
  - Reader lọc `status='active'` (`notification_rules_reader.go:40`). Vì vậy nếu chỉ đơn giản coi "thiếu row" là mặc định, row đang bị disabled sẽ bị bật lên ngoài ý muốn.
  - Confirmed.
- **Test tái hiện (GATE R)**, `notification_rules_dispatch_test.go`, đều fail trên HEAD:
  - `TestDispatchDue_FlagOn_InAppOnly_NoEmailOneInApp`
  - `TestDispatchDue_FlagOn_InAppDisabled_NoInApp`
  - `TestNotificationRulesEvaluator_NoRowAppliesDefaults` (offset 7)
- **Thiết kế sửa:**
  - Evaluator: sau khi kiểm scope và lịch, đặt `decision.ChannelAllowed = {"email": emailEnabled&&matched, "in_app": inAppEnabled&&matched}`. Giữ ý nghĩa `Allowed` = "kênh được yêu cầu được phép", để simulate không đổi.
  - Lớp quyết định: chỉ skip khi cả 2 kênh đều không được phép. Ngoài ra `out.SkipEmail = !ChannelAllowed["email"]`, `out.AllowInApp = ChannelAllowed["in_app"]`.
  - Dispatch:
    - thêm field nội bộ `SkipEmail` vào `DispatchOccurrenceRequest` (`json:"-"`);
    - trong `dispatchClaimedOccurrence`: nếu `SkipEmail` thì `providerMessageID="in_app_only"` và không gọi SMTP;
    - goroutine in-app sau SENT giữ nguyên (at-most-once, retry không tạo in-app trùng).
  - Reader: bỏ `AND status='active'` để evaluator báo RULE_DISABLED. Cần kiểm rằng row là duy nhất trên `(company_id, rule_code)`.
  - `doc==nil`: dùng một document mặc định riêng trong reminder, sao y `companyaccess/app/alert_channel_prefs.go:34-60` (không có import cycle), kèm parity test. Ghi audit `defaults_applied=true`.
  - Viết lại các test đang mã hoá hành vi cũ: `TestNotificationRulesEvaluator_NoRules`, `TestPrepareDispatch_FlagOn_RuleMissing_Skip`, `TestDispatchDue_FlagOn_RuleMissing_NoSend`.
- **Contract / schema / FE:** endpoint simulate có thêm các field trace (bổ sung). Mã RULE_MISSING vẫn giữ nhưng hiếm gặp. FE cũ chạy được.
- **Dữ liệu (nháp):** đếm `notification_rules` theo status cho các company DEV.
- **Rollout / rollback:** worker + API. Khi flag tắt thì không đổi hành vi. Rollback bằng revert.
- **Verify (GATE V):**
  - `go test ./internal/reminder/... ./internal/companyaccess/... -count=1`
  - Smoke, ENT: `GET /api/v1/admin/notification-rules/status` (ghi lại `runtime_consumer_enabled`); `POST /api/v1/admin/notification-rules/simulate` với `channel=in_app` → would-send. Simulate luôn bật evaluator, nên không cần đổi flag.
- **Phạm vi ảnh hưởng:** khi bật flag, mọi company không có row prefs sẽ bắt đầu nhận reminder theo mặc định. Đây là chủ đích; phải ghi trong release notes.
- **Quyết định PR45-D9:** thiếu row → dùng mặc định (khuyến nghị). Các phương án khác: tạo row cho từng company (ghi dữ liệu), hoặc giữ fail-closed.
- **Ước lượng:** M

### RMC-08 Lịch mặc định chặn reminder bước; `department_focal` dùng phòng ban của template global
- **Mức độ / nguồn:** MEDIUM, tiềm ẩn (cùng flag với RMC-07).
- **Nguyên nhân gốc:**
  - `matchScheduleAtDispatch` (`notification_rules_schedule.go`) so milestone của bước workflow (`DefaultWorkflowStepReminderDays={3,1}`, `disclosure/app/workflow_step_reminder_rule.go:13`) với danh sách của company [30,7,3,0], nên offset 1 không bao giờ khớp.
  - `department_focal` đưa thẳng `department_id` của bước global vào `EmailsByDepartments` (`recipient_policy_filter.go:79-91`) mà không ánh xạ sang phòng ban của company.
  - Confirmed.
- **Test tái hiện (GATE R):**
  - `notification_rules_test.go` `TestNotificationRulesEvaluator_WorkflowOffset1_NotGatedByCompanySchedules`. HEAD fail.
  - `TestApplyRecipientPolicies_DepartmentFocalResolvesCompanyDepartment`:
    - fake bước global có phòng `legal`;
    - `ResolveCompanyDepartment` ánh xạ `legal` → `d_legal_c1`;
    - `EmailsByDepartments` chỉ trả kết quả cho `d_legal_c1`.
    - HEAD trả rỗng.
- **Thiết kế sửa:**
  - Evaluator: khi `EventType=="workflow"` thì bỏ qua so lịch, coi như khớp.
  - Policy:
    - thêm `InstanceStepReader` vào `DispatchDecisionDeps`;
    - ưu tiên `GetStepByInstance`, nếu không có mới dùng bước global;
    - ánh xạ bằng `ResolveCompanyDepartment`; không khớp thì policy không đóng góp người nhận.
  - Không đổi lịch mặc định.
- **Contract / schema / FE:** không. Tuỳ chọn sửa copy WEB: "Thời điểm gửi" áp dụng cho reminder hạn nộp.
- **Dữ liệu:** không.
- **Rollout / rollback:** worker + API; rollback bằng revert.
- **Verify (GATE V):** như RMC-07; simulate với `event_type=workflow`, offset 1 → would-send.
- **Phạm vi ảnh hưởng / hiệu năng:** thêm 1 query resolve và 1 lần đọc snapshot cho mỗi candidate khi policy bật, tối đa 50 mỗi tick.
- **Quyết định PR45-D10:** lịch của company chỉ gate reminder hạn nộp (khuyến nghị).
- **Ước lượng:** S–M

## Thứ tự commit

**PR-4 BE** (rebase lên PR-3 commit 5; mỗi commit kèm test GATE R)
1. `fix(deadlinealerts): enrich Draft rows + single-record detail lookups` (DLA-02)
2. `fix(deadlinealerts): deterministic list order` (DLA-07)
3. `fix(deadlinealerts): log dropped paged rows; document total` (DLA-06)
4. `fix(deadlinealerts): first confirmation wins` (DLA-11)
5. `fix(portaldashboard): open KPIs are current-state` (DLA-03)
6. `fix(portaldashboard): confirmation counts as completion; on-time severity` (DLA-04)
7. `feat(observe): unmaterialized periodic cycle gauge` (DLA-09)
8. `docs`: `api-contracts-json.md` + ai-cache.

**PR-4 WEB** (deploy sau BE)
- W1 `fix(deadlines): due-today label + unknown status` (DLA-12; cần duyệt sửa `types.ts`)
- W2 `copy(dashboard): KPI tooltips match current-state semantics` (DLA-03)

**PR-5 BE** (sau PR-1b / REM-05)
1. `fix(reminder): skip inactive companies at source` (RMC-05)
2. `fix(disclosure): seed periodic cycles for suspended companies` (AHA-11; cần PR45-D8)
3. `fix(reminder): per-channel email/in-app decision` (RMC-07)
4. `fix(reminder): defaults when prefs row missing; reader returns disabled rows` (RMC-07; cần PR45-D9)
5. `fix(reminder): step reminders not schedule-gated; department_focal resolves company department` (RMC-08)

**PR-5 WEB:** không cần.

## Reviewer (mở song song; mỗi PR một message)
- **PR-4 BE:**
  - be-security + admin-role: Draft giờ hiển thị cho người được giao; nhánh phòng ban của bước áp dụng cho Draft ở đường chi tiết/steps.
  - perf-reliability: payload của helper ở đường full; lookup chi tiết thu hẹp; query của collector.
  - api-compat: ngữ nghĩa KPI, `total`, response của confirm.
- **PR-4 WEB:** api-compat + tự review theo frontend-test-regression.
- **PR-5:**
  - be-security: policy người nhận, phạm vi tenant.
  - perf-reliability: `EXPLAIN` SQL dispatch, `SKIP LOCKED OF`, goroutine in-app.
  - api-compat: output của simulate, trộn phiên bản worker/API.
  - Sau cùng: `premerge-system-review` cho cả hai PR.

## Checklist verify
- [ ] Mọi test GATE R fail trên HEAD và pass sau khi sửa (ghi rõ các trường hợp compile-fail).
- [ ] `go test ./... -count=1` và `go vet ./...`. Chỉ được còn các fail có sẵn: 6 test template-import của `disclosure/app`, VariableParity của `notification/app`, cảnh báo vet của `workflowfulfillment`.
- [ ] `go test -race ./internal/deadlinealerts/app ./internal/portaldashboard/app ./internal/reminder/app` (BLOCKED nếu Windows không có cgo).
- [ ] `MYSQL_TEST_DSN=… go test ./internal/deadlinealerts/infra/mysql ./internal/reminder/infra/mysql -run Integration` (MySQL compose local đã chạy migration), hoặc ghi BLOCKED.
- [ ] `docker compose -f docker-compose.dev.yml build api worker`
- [ ] WEB: `npx vitest run src/services src/pages/portal/deadlines src/features/portal-dashboard`, `npm run lint`, `npm run test`.
- [ ] Smoke DEV read-only:
  - ENT duyệt danh sách: không có trang trùng, cột enrich có dữ liệu;
  - MEMBER gọi steps → 200;
  - overview 7d so với 30d;
  - severity đúng hạn;
  - gauge trên `/metrics`;
  - notification status + simulate (`in_app`, offset 1 của workflow).
- [ ] Smoke DEV có ghi dữ liệu (confirm 2 lần): chỉ khi được cho phép.
- [ ] Trước khi PR-5 chạy thật: ghi lại `NOTIFICATION_RULES_CONSUMER_ENABLED` trên DEV và phân bố trạng thái company.
- [ ] Ma trận tương thích:
  - FE cũ + BE mới: OK cho mọi mục;
  - FE mới + BE cũ: OK (chỉ nhãn);
  - rollback riêng BE an toàn;
  - không đổi schema.
