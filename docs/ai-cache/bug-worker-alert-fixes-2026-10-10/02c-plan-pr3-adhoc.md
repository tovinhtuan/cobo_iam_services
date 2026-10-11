# PR-3 Ad-hoc (CBTT bất thường 24h): kế hoạch triển khai chi tiết (wf-bugfix Phase 3–6, chỉ lập kế hoạch)

> **Cập nhật sau khi PO chốt (2026-10-11, Contract v1.2 §2.2). Các quyết định dưới đây thay thế khuyến nghị cũ trong file này:**
> - **AHA-01 (Q-12) = (a′).** Nhánh OR chỉ nhận record gắn với proposal `approved` **và** có `LOWER(TRIM(dr.status)) NOT IN ('approved','published','completed','done')`. Sau khi workflow hoàn tất, record rời tab Cảnh báo. Không còn PENDING_CONFIRM (Q-19). AC-11.
> - **AHA-03 (Q-13): bỏ quy tắc "ngày sớm nhất".** Áp dụng "giá trị được nhập sau cùng thắng":
>   - `ReserveVote` dùng `COALESCE(VALUES(x), x)`;
>   - `DeriveFinalAdjustments` lấy **giá trị khác null của phiếu mới nhất** (theo `approved_at`) cho từng trường;
>   - modal duyệt (WEB `AdHocProposalDetailPage.tsx`) hiển thị giá trị hiện hành và người đã nhập;
>   - vẫn sửa DUE-06 bằng `COALESCE(final_t0_date, proposed_t0_date)`.
> - **AHA-06 (Q-15) = A + B.**
>   - A: Reject/Cancel trả 409 khi `approvals >= reviewers`, kiểm dưới `FOR UPDATE`;
>   - B: `MaterializationAbandoner` xử lý trường hợp finalize lỗi.
> - **AHA-09b (Q-16):** làm thành **PR-3b**, chạy ngay sau PR-3 (in-app khi được giao bước, sau flag).
> - **AHA-05 (Q-14) = F2.**

> **Tóm tắt (VI)**
> - **Không đổi schema.** Baseline `go test` của adhoc, deadlinealerts, workflow, `disclosure/infra`, personalops đều pass ở HEAD `a8acb51`.
> - **Thành phần dùng chung mới:** `internal/platform/workflowsel` chọn instance ưu tiên `proposal_snapshot%`, rồi đến instance mới nhất. Thành phần này cũng dùng cho DLA-02/DLA-10 của PR-4, nên PR-4 phải rebase lên commit 5 của PR-3.
> - **Quyết định (cần sản phẩm/BA chốt):**
>   - PR3-D1 (AHA-01): phương án (a) OR-branch, khuyến nghị; hay (b) SkipCompanySubmit.
>   - PR3-D2 (AHA-03): quy tắc ngày chốt khi nhiều người duyệt; khuyến nghị lấy ngày **sớm nhất**.
>   - PR3-D3 (AHA-05): F2, kiểm quyền người tạo lúc submit (khuyến nghị); hay F1, materialize nội bộ.
>   - PR3-D4 (AHA-06): B, bù trừ (khuyến nghị); hay A, chặn Reject/Cancel khi đã đủ phiếu.
>   - PR3-D5 (AHA-09b): làm trong PR-3 hay để follow-up (khuyến nghị: follow-up).
>
> Nguồn: agent lập kế hoạch PR-3 (read-only).

---

### AHA-01 Ad-hoc đã duyệt không bao giờ có trong tab Hạn
- **Mức độ / nguồn:** HIGH. Regression của các bản sửa 25–26/05. Đã xác nhận trên DEV.
- **Nguyên nhân gốc:** tầng read model API. Confirmed.
  - Finalize gọi `SubmitRecord` (`record_creator.go:136-143`), chuyển record sang `PendingReview` và đặt `submitted_at` (`disclosure/app/service.go:327-337`).
  - Membership của danh sách chỉ nhận `status='draft' AND submitted_at IS NULL` (`list_rows_membership.go:19-33`).
- **Test tái hiện (GATE R):**
  - Unit, file `internal/deadlinealerts/infra/mysql/list_rows_membership_test.go`:
    - `TestListRowsMembership_IncludesApprovedAdHocBranch`: hằng SQL phải chứa `ad_hoc_proposals` và `status = 'approved'`.
    - Mở rộng `TestListRowsV1Membership_acceptanceMatrix` thêm các dòng `ADHOC_APPROVED_PENDINGREVIEW_INCLUDED`, `ADHOC_APPROVED_PUBLISHED_INCLUDED`, `UNLINKED_PENDINGREVIEW_EXCLUDED`.
  - mysql-integration `TestIntegration_ListRows_ApprovedAdHocIncluded`:
    - Seed: record `PendingReview` có `submitted_at`, cộng một proposal approved trỏ tới nó.
    - Kỳ vọng `ListRowsPage` trả dòng đó với `total=1`; nếu proposal là rejected thì không trả.
- **Thiết kế sửa (khuyến nghị phương án a: giữ `SubmitRecord`, thêm nhánh OR):**
  ```
  -  AND LOWER(TRIM(dr.status)) = 'draft' AND dr.submitted_at IS NULL AND ( <periodic gate> )
  +  AND ( ( LOWER(TRIM(dr.status)) = 'draft' AND dr.submitted_at IS NULL AND ( <periodic gate> ) )
  +        OR EXISTS (SELECT 1 FROM ad_hoc_proposals ahp
  +                   WHERE ahp.company_id = dr.company_id AND ahp.record_id = dr.record_id
  +                     AND ahp.status = 'approved') )
  ```
  - Tham số bind không đổi.
  - Parity in-memory: `listRowsV1MembershipEligible(status, submittedAtNull, linkedApprovedAdHoc bool, cycles, today)` trả true sớm nếu `linkedApprovedAdHoc`.
  - Resolver hiện có (`deadlinealerts/app/service.go:415-454`) chạy đúng mà không cần sửa:
    - PendingReview/Approved → UPCOMING/DUE_SOON/OVERDUE, dùng hạn ad-hoc;
    - Published/Completed → PENDING_CONFIRM;
    - đã confirm → DONE.
  - Phương án (b) `SkipCompanySubmit: true`:
    - Ưu điểm: bỏ luôn bootstrap AHA-07, và người tạo không cần quyền `disclosure.submit`.
    - Nhược điểm:
      - Truy vấn enrich đang lọc `<> 'draft'`, nên bước/phòng ban/người phụ trách trống cho tới khi có PR-4.
      - personalops loại Draft (`personalops/infra/mysql/repository.go:115,246`), nên task ad-hoc biến mất khỏi "của tôi".
      - Draft sửa/xoá được; nếu ai đó Submit tay thì record lại biến mất.
      - `MarkRecordApproved` chuyển sang Approved làm dòng rời danh sách, hỏng luồng PENDING_CONFIRM → DONE của 26/05.
      - FE hiện badge "Draft" (`normalizers.ts:455`).
  - Không đổi: filter enrich `<> 'draft'` (để PR-4/DLA-02), resolver trạng thái, cổng OpenAt của periodic.
  - Nơi đọc bị ảnh hưởng (phương án a): `listRows`, `countListRows`, `ListDeadlineAlertSnapshot` (dashboard, `portaldashboard/app/service.go:126`). `GetAlertRowByRecordID`, personalops và reminder không bị ảnh hưởng.
- **Contract / schema / FE:**
  - Shape response không đổi, chỉ thêm dòng.
  - Màn hình WEB: `DeadlineList.tsx`, `DeadlineDetail.tsx`, `deadlines/DeadlineListFilterBar.tsx`, widget dashboard. Mọi status đã được map (`deadlineAlertsApi.ts:48-63`).
  - FE cũ chạy với BE mới: OK.
- **Dữ liệu DEV:** không cần sửa. Query kiểm tra (= số dòng mới sẽ xuất hiện):
  ```sql
  SELECT p.proposal_id, p.record_id, dr.status, dr.submitted_at
  FROM ad_hoc_proposals p
  JOIN disclosure_records dr ON dr.company_id=p.company_id AND dr.record_id=p.record_id
  WHERE p.status='approved';
  ```
- **Rollout / rollback:** chỉ BE (`make deploy-be`). Rollback bằng cách deploy lại image cũ.
- **Verify (GATE V):**
  - `go test ./internal/deadlinealerts/...` + integration test.
  - DEV, persona admin: `GET /api/v1/company/deadline-alerts?page=1&page_size=100` có record đã duyệt với `due_date` = hạn ad-hoc, `total` tăng đúng N.
  - DEV, persona người được giao bước: cũng thấy record này (scope qua EXISTS task).
- **Phạm vi ảnh hưởng:**
  - Số đếm danh sách và dashboard của từng company.
  - EXISTS dùng `idx_adhoc_company_status`; chưa có index `(company_id, record_id)`, cần chạy EXPLAIN trên DEV.
- **Phụ thuộc / quyết định:** PR3-D1. Sửa cùng file với PR-4, nên PR-3 vào trước.
- **Ước lượng:** S

### AHA-07 / H15 / DLA-10 Record ad-hoc có 2 workflow instance; nơi đọc chọn sai
- **Mức độ / nguồn:** HIGH. Đã xác nhận trên DEV: có cả `global_template` và `proposal_snapshot_v3`.
- **Nguyên nhân gốc:** tầng service + SQL. Confirmed.
  - `SubmitRecord` gọi `EnsureOnSubmit` vì lúc đó chưa có instance (`disclosure/app/service.go:342-346`; `bootstrap.go:26-67`), tạo instance template và giao task cho người tạo.
  - Sau đó adapter mới tạo instance proposal (`record_creator.go:173`).
  - Các nơi đọc chọn `ORDER BY workflow_instance_id ASC LIMIT 1`, tức instance template:
    - `disclosure/infra/mysql/repository.go:100-106`
    - `deadlinealerts/infra/mysql/repository.go:110-116,514-520`
    - `steps_repository.go:18-25`, cũng được `workflowfulfillment/deadline_bridge.go:116` dùng
  - `listCurrentStepMeta` lấy instance cuối cùng quét được (`repository.go:382-420`).
  - Khi finalize replay, hàm trả `existing.WorkflowInstanceID` (instance template) và gắn nó vào proposal (`record_creator.go:128-129`). Ad-hoc chưa bao giờ đặt `ResumeWorkflowOnDuplicate` (`runtime_materialize.go:184-202`).
- **Test tái hiện (GATE R, unit):**
  - `internal/adhoc/infra/disclosure/record_creator_bootstrap_test.go` `TestAdHocFinalize_DoesNotInvokeSubmitBootstrap`:
    - Dùng `disclosureapp.NewService(inmemory.NewRepository(), nil, idg)` thật, `SetWorkflowBootstrap(countingBootstrap)`, materializer giả, `ProposalWorkflow` v3.
    - Kỳ vọng bootstrap được gọi 0 lần và chỉ có 1 instance `proposal_snapshot_v3`. HEAD gọi 1 lần.
  - `TestBuildCreateRecordOptsForFinalize_ResumesWorkflowOnDuplicate` (adhoc/app). HEAD trả false.
  - `TestReplay_OnlyTemplateInstance_CreatesProposalInstance`: fake `GetRecord` trả id instance template và `SubmittedAt` đã đặt. Kỳ vọng tạo instance proposal mới, không gọi `SubmitRecord` lần hai.
  - mysql-integration `TestIntegration_InstanceSelection_PrefersProposalSnapshot`: seed instance template (id nhỏ hơn) và instance proposal. `FindByID`, `GetWorkflowInstanceByRecord`, `listRows`, `GetAlertRowByRecordID`, `listCurrentStepMeta` đều phải trả instance proposal.
  - Fallback khi chưa có `MYSQL_TEST_DSN`: test kiểm mã nguồn của 3 file không còn chứa `workflow_instance_id ASC`.
- **Thiết kế sửa:**
  1. Thêm `SkipWorkflowBootstrap bool` vào `disclosureapp.SubmitRecordRequest`. Struct này chỉ được tạo ở `record_creator.go:137` và `transport/http/handler.go:187`, và handler không đặt cờ này. Sửa `service.go:342` thêm `&& !req.SkipWorkflowBootstrap`.
  2. `record_creator.go`: truyền `SkipWorkflowBootstrap: a.workflowOn`, và chỉ submit khi `!opts.SkipCompanySubmit && rec.SubmittedAt == nil` (submit lại sẽ lật PendingReview → "In Progress").
  3. Replay ở chế độ proposal (`opts.ProposalWorkflow != nil`), khi `existing.WorkflowInstanceID != ""`:
     - gọi `a.workflow.GetWorkflowInstanceInternal`;
     - chỉ trả lại id cũ nếu `workflowapp.IsProposalSnapshotFrozen(inst.WorkflowSource)`;
     - nếu không thì tạo instance proposal.

     Replay của legacy và periodic không đổi.
  4. `BuildCreateRecordOptsForFinalize` đặt `ResumeWorkflowOnDuplicate: true`.
  5. Thêm `GetWorkflowInstanceInternal` vào `workflowapp.Service`, không authz, theo tiền lệ `CreateWorkflowInstanceInternal`. Không có fake test nào implement `Service`.
  6. File mới `internal/platform/workflowsel/instance.go` (không phụ thuộc gì):
     - `PreferredInstanceOrderBy(a)` = `ORDER BY (COALESCE(a.workflow_source,'') LIKE 'proposal_snapshot%') DESC, a.created_at DESC, a.workflow_instance_id DESC`
     - `PreferredInstanceSubquery(companyExpr, recordExpr)`

     Dùng ở 4 chỗ đọc nêu trên. Trong `listCurrentStepMeta`, thêm `AND wi.workflow_instance_id = <subquery>`. "Mới nhất" cũng đúng cho ad-hoc chế độ legacy.
  - Không đổi: đường periodic; các nơi đọc dùng EXISTS (scope, assignee, personalops), vì bước sửa dữ liệu sẽ huỷ các task lạc; nơi đọc của reminder.
- **Contract / schema / FE:**
  - `workflow_instance_id` trong response record/alert/steps giờ là instance proposal (cùng field, cùng kiểu).
  - Màn hình WEB: `DisclosureDetail`, `DeadlineDetail`, `deadlines/DeadlineWorkflowCard.tsx`, `WorkflowStepsOverview.tsx`, `adhoc/ProposalRuntimeTrackingPanel.tsx` (panel này sẽ đúng sau khi sửa dữ liệu).
  - FE cũ chạy với BE mới: OK.
- **Dữ liệu DEV (bản nháp, chưa chạy):**
  - Kiểm tra trước:
    ```sql
    SELECT p.proposal_id, p.record_id, p.workflow_instance_id bound, wi.workflow_instance_id, wi.workflow_source, wi.status,
      (SELECT COUNT(*) FROM workflow_tasks t WHERE t.workflow_instance_id=wi.workflow_instance_id AND t.status<>'pending') non_pending,
      (SELECT COUNT(*) FROM workflow_instance_step_states s WHERE s.workflow_instance_id=wi.workflow_instance_id) step_states
    FROM ad_hoc_proposals p
    JOIN workflow_instances wi ON wi.company_id=p.company_id AND wi.record_id=p.record_id
    WHERE p.status='approved'
      AND (SELECT COUNT(*) FROM workflow_instances w2 WHERE w2.record_id=p.record_id)>1;
    ```
  - Sửa, trong một transaction:
    1. Gắn lại proposal bị gắn sai:
       ```sql
       UPDATE ad_hoc_proposals p
       JOIN workflow_instances wi ON wi.company_id=p.company_id AND wi.record_id=p.record_id
                                AND wi.workflow_source LIKE 'proposal_snapshot%'
       SET p.workflow_instance_id=wi.workflow_instance_id
       WHERE p.status='approved' AND p.workflow_instance_id<>wi.workflow_instance_id;
       ```
    2. Với instance không phải instance đã gắn, `workflow_source NOT LIKE 'proposal_snapshot%'`, `non_pending=0` và `step_states=0`:
       ```sql
       UPDATE workflow_tasks SET status='cancelled' WHERE status='pending' AND workflow_instance_id IN (...);
       UPDATE workflow_instances SET status='rejected' WHERE status='in_progress' AND workflow_instance_id IN (...);
       ```
       Reminder dừng vì task không còn pending (`workflow_step_task_state.go:62`).
    3. Instance có step state hoặc task đã được xử lý: xem xét thủ công.
- **Rollout / rollback:** deploy BE, rồi chạy bước sửa dữ liệu khi được cho phép. Rollback BE an toàn; khi đó thứ tự ASC cũ sẽ hiển thị instance template đã rejected.
- **Verify (GATE V):**
  - Các test ở trên.
  - DEV, với một proposal v3 mới:
    - `qa_ro`: `SELECT workflow_source FROM workflow_instances WHERE record_id=?` trả đúng 1 dòng `proposal_snapshot_v3`;
    - `GET /api/v1/company/deadlines/{record_id}/steps` và GET record trả cùng id với proposal.
- **Phạm vi ảnh hưởng:** mọi record có >1 instance (thực tế chỉ ad-hoc). Bản trùng tạo tay qua `POST /api/v1/workflows/instances` giờ sẽ chọn instance mới nhất thay vì cũ nhất.
- **Phụ thuộc:** không. Truy vấn deadlinealerts chồng với PR-4.
- **Ước lượng:** M

### AHA-02 Side effect hoàn tất bị gate theo hành động của task thay vì trạng thái instance
- **Mức độ / nguồn:** HIGH, confirmed (code).
- **Nguyên nhân gốc:** tầng service workflow. Confirmed.
  - Cả hai planner đều đặt instance thành approved ở bước cuối, bất kể hành động là gì (`service.go:401-407` cho v2, `:515-523` cho legacy).
  - Nhưng `runInstanceCompletionSideEffects` chỉ chạy khi hành động của task là `approved` (`service.go:526-537`).
  - Vì vậy Confirm/Review ở task cuối không gọi `MarkRecordApproved` và `NotifyWorkflowApproved`.
- **Test tái hiện (GATE R, unit)** trong `internal/workflow/app/multistep_runtime_test.go`:
  - `TestV2MultiStep_FinalConfirm_RunsCompletionSideEffects`:
    - `seedV2ThreeStep`, approve s1 và s2, rồi `ConfirmTask` s3;
    - dùng `countingRecordUpdater` và `countingNotifier`;
    - kỳ vọng 1/1. HEAD cho 0/0.
  - `TestLegacy_FinalReview_RunsCompletionSideEffects`.
  - Kiểm ngược: `TestFinalReject_NoApprovalSideEffects`.
- **Thiết kế sửa:** trong `runInstanceCompletionSideEffects`:
  ```go
  - if terminalStatus == "approved" && s.recordUpdater != nil {
  + if inst.Status != "approved" { return nil }
  + if s.recordUpdater != nil {
  ```
  Sửa nhánh notifier tương tự. Không đổi planner và guard của `ApplyTaskTransition`: guard `WHERE status=?` đảm bảo chạy đúng một lần.
- **Contract / schema / FE:** không đổi. Record thành Approved, FE hiển thị "Published" (mapping sẵn có, `normalizers.ts:460`). Approved vẫn là trạng thái chưa kết thúc trong danh sách hạn.
- **Dữ liệu DEV (bản nháp):**
  - Kiểm tra trước: instance ưu tiên đã approved nhưng record còn ở `draft`, `pendingreview` hoặc `in progress`. Loại trừ các instance template lạc (đã xử lý ở AHA-07).
  - Sửa:
    ```sql
    UPDATE disclosure_records dr
    JOIN workflow_instances wi ...
    SET dr.status='Approved',
        dr.published_date=COALESCE(dr.published_date, DATE(wi.updated_at));
    ```
  - Không gửi bù email.
- **Rollout / rollback:** chỉ BE.
- **Verify (GATE V):**
  - Unit test ở trên.
  - DEV: persona được giao gọi `POST /api/v1/workflows/tasks/{id}/confirm` trên task cuối → record thành Approved, và có dòng `email_notifications` với key `workflow.approved.*`.
- **Phạm vi ảnh hưởng:** mọi workflow kết thúc bằng Confirm/Review, kể cả periodic. Email "đã duyệt" sẽ nhiều hơn, và như vậy mới đúng.
- **Phụ thuộc:** chạy sau bước sửa dữ liệu của AHA-07.
- **Ước lượng:** S

### AHA-03 / DUE-06 Duyệt nhiều người chỉ lấy ngày chốt của người cuối; hạn bỏ qua `final_t0_date`
- **Mức độ / nguồn:** HIGH, confirmed (code).
- **Nguyên nhân gốc:** tầng service + SQL. Confirmed.
  - Finalize chỉ dùng `VoteReservation.LastFinal*` của người bỏ phiếu cuối (`adhoc/infra/mysql/repository.go:517-519`; `adhoc/app/service.go:533,584`).
  - `ON DUPLICATE KEY UPDATE` xoá ngày chốt trước đó khi reviewer bỏ phiếu lại với ô trống (`repository.go:489-494`).
  - Các nơi tính hạn dùng `proposed_t0_date`: `deadlinealerts/infra/mysql/repository.go:328-334`, `personalops/infra/mysql/repository.go:343-348`, `proposal_due.go:58-63`.
- **Test tái hiện (GATE R):**
  - Unit (adhoc/app) `TestApprove_MultiReviewer_FinalDatesFromAllVotes`:
    - A gửi hạn 2026-10-11 và T0 2026-10-10; B bỏ phiếu với ô trống.
    - Kỳ vọng `repo.lastUpdate.FinalDeadlineDate=="2026-10-11"` và `recordCreator.t0Date==2026-10-10`. HEAD cho nil.
  - Unit `proposal_due_test.go` `TestFormatProposalDueDate_FinalT0Overrides`.
  - mysql-integration `TestIntegration_ReserveVote_RetryKeepsAdjustments`.
- **Thiết kế sửa:**
  1. File mới `adhoc/app/final_adjustments.go`: `DeriveFinalAdjustments([]ApprovalDTO) (t0, deadline *string, note string)`.
  2. `finalizeApprovedProposal` gọi `s.repo.ListApprovals` và dùng giá trị đã suy ra cho `t0Time` và `CompleteFinalize`. Không đổi `AdminApprove`.
  3. `ReserveVote`: dùng `final_t0_date=COALESCE(VALUES(final_t0_date),final_t0_date)`, tương tự cho `final_deadline_date` và `adjustment_note`. Cập nhật fake repo theo.
  4. Thêm `FinalT0Date sql.NullTime` vào `ProposalDueInput`; nếu hợp lệ thì dùng làm ngày bắt đầu. Cả hai nơi đọc thêm `SELECT final_t0_date` (cột đã có từ migration 0037).
- **Contract / schema / FE:**
  - Giá trị của `final_t0_date`, `final_deadline_date`, `adjustment_note` thay đổi; field giữ nguyên.
  - WEB: `AdHocProposalDetailPage.tsx`, hạn trong danh sách hạn, personal ops.
  - FE cũ chạy với BE mới: OK.
- **Dữ liệu DEV (bản nháp, chỉ sau khi chốt quy tắc):**
  - Kiểm tra trước: proposal approved có `MIN(a.final_deadline_date)` khác `p.final_deadline_date` (`GROUP BY` với `<=>`).
  - Chỉ sửa `final_deadline_date`. Không sửa `final_t0_date`, vì T0 của workflow và các milestone đã được tạo.
- **Rollout / rollback:** chỉ BE.
- **Verify (GATE V):**
  - Các test ở trên.
  - DEV với 2 persona reviewer: A chỉnh hạn, B duyệt với ô trống → GET proposal hiện ngày của A; deadline alert có cùng `due_date`.
- **Phạm vi ảnh hưởng:** chỉ proposal v3 nhiều reviewer. Với một reviewer, kết quả như cũ.
- **Phụ thuộc / quyết định:** PR3-D2.
  - Phương án: ngày sớm nhất / giá trị khác null cuối cùng / reviewer chính (`sort_order` 0).
  - Khuyến nghị: **sớm nhất** cho cả hạn và T0 (an toàn nhất cho cửa sổ tuân thủ 24h); ghi chú nối theo thứ tự bỏ phiếu.
  - Cần BA ký duyệt.
- **Ước lượng:** M

### AHA-04 `shortNote` cắt theo byte làm hỏng UTF-8, insert in-app thất bại
- **Mức độ / nguồn:** MEDIUM, confirmed. Thử cục bộ: 12/30 ghi chú điển hình bị hỏng.
- **Nguyên nhân gốc:** tầng notifier. Confirmed.
  - `note[:117]` (`adhoc/infra/notification/notifier.go:381-386`) cắt giữa ký tự.
  - MySQL strict từ chối giá trị cho cột TEXT `body`; lỗi chỉ được log (`:89-95`).
- **Test tái hiện (GATE R, unit):** `notifier_test.go` `TestShortNote_VietnameseKeepsValidUTF8`.
  - Đầu vào: 116 ký tự ASCII rồi "Điều chỉnh...".
  - Kỳ vọng `utf8.ValidString`, ≤120 rune, có hậu tố `...`.
- **Thiết kế sửa:** cắt theo rune (`utf8.RuneCountInString` / `[]rune`), giới hạn 120 rune. Không còn chỗ nào khác trên đường thông báo cắt theo byte.
- **Contract / schema / FE:** không.
- **Dữ liệu DEV:** không sửa được (dòng in-app đã mất).
- **Rollout / rollback:** chỉ BE.
- **Verify (GATE V):**
  - Unit test ở trên.
  - DEV: tạo proposal có `change_note` tiếng Việt dài → reviewer có dòng `user_in_app_notifications`.
- **Phạm vi ảnh hưởng:** chỉ phần body của in-app ad-hoc.
- **Phụ thuộc / quyết định:** không.
- **Ước lượng:** S

### AHA-05 Proposal kẹt: không kiểm lại reviewer, quyền người tạo, `PrepareV3Materialization` không được gọi
- **Mức độ / nguồn:** MEDIUM, confirmed (code).
- **Nguyên nhân gốc:** tầng service/authz. Confirmed.
  - Lúc submit chỉ kiểm danh sách reviewer không rỗng (`service.go:375-381`). Lúc tạo thì có kiểm reviewer active và có quyền focal_review (`:168-177`), nhưng không kiểm lại khi submit.
  - Finalize chạy `CreateRecord` + `SubmitRecord` với subject là người tạo (`record_creator.go:79-83`). Việc này cần các quyền:
    - `disclosure.create` (`disclosure/app/service.go:183-193`);
    - `disclosure.submit` hoặc `disclosure.update` (`:322-326`);
    - `disclosure.view` khi replay.
  - `PrepareV3Materialization` (`runtime_materialize.go:147-165`) không có caller nào. v2 có gọi hàm tương đương (`service.go:555`).
- **Test tái hiện (GATE R, unit, adhoc/app):**
  - `TestSubmitProposal_InactiveReviewer_Rejected`: kỳ vọng 422 (HEAD trả 200).
  - `TestSubmitProposal_CreatorLacksDisclosureCreate_Rejected`: kỳ vọng 403.
  - `TestFinalize_V3InactiveAssignee_FailsBeforeRecordCreate`: kỳ vọng 422 và `recordCreator.callCount==0`.
- **Thiết kế sửa:**
  - (a) `SubmitProposal` chạy lại đúng vòng kiểm reviewer như lúc tạo (`IsActiveMembership` + `HasPermission ad_hoc_alert.focal_review`). Trả 422 với field error ở `reviewer_membership_ids`, dùng đúng thông điệp hiện có mà FE đã map (`adHocProposalErrors.ts:104`).
  - (b) Không đổi quorum (cần mọi reviewer được giao đồng ý). Lối thoát khi kẹt: người tạo Cancel rồi clone, đã có sẵn khi pending.
  - (c) Phương án F2 (khuyến nghị): lúc submit kiểm `hasPermission(creator,"disclosure.create", cùng ResourceRef với CreateRecord)` và (`disclosure.submit` hoặc `disclosure.update`), thiếu thì 403.
  - Phương án F1: materialize nội bộ tin cậy (bỏ authz theo subject, giữ gate của template). Đây là thay đổi mô hình quyền, cần admin-role + be-security review.
  - (d) Finalize gọi `PrepareV3Materialization` khi `mode==MaterializationModeV3Snapshot`, giống cách v2 làm.
- **Contract / schema / FE:**
  - Submit chặt hơn: có thêm 422/403 cho đầu vào trước đây pass. Approve có thể trả 422.
  - Envelope lỗi giữ nguyên; FE hiển thị thông điệp có sẵn hoặc thông điệp chung.
  - FE cũ chạy với BE mới: OK.
- **Dữ liệu DEV:** liệt kê proposal đang pending có reviewer inactive hoặc người tạo thiếu quyền. Không tự sửa; người tạo tự cancel và clone.
- **Rollout / rollback:** chỉ BE.
- **Verify (GATE V):**
  - Unit test ở trên.
  - DEV: không deactivate ai; dùng persona QA sẵn có để xác nhận proposal hợp lệ vẫn submit được.
- **Phạm vi ảnh hưởng:** PrepareV3 còn kiểm phòng ban active và assignee thuộc đúng phòng ban, chặt hơn HEAD nhưng bằng v2.
- **Phụ thuộc / quyết định:** PR3-D3 (khuyến nghị F2 trước); quy tắc quorum của reviewer.
- **Ước lượng:** M

### AHA-06 Reject/Cancel trong lúc (hoặc sau khi lỗi) finalize để lại record và workflow sống
- **Mức độ / nguồn:** MEDIUM, confirmed (code).
- **Nguyên nhân gốc:** tầng service. Confirmed.
  - Finalize không có transaction xuyên module.
  - Reject/Cancel chỉ guard `status=pending_focal_approval` (`service.go:740-823` → `repository.go:212-260`), nên có thể thắng trong khoảng giữa lúc tạo record/workflow và `CompleteFinalize` (`repository.go:540-570`).
  - Không có bước nào gỡ record hoặc instance đã tạo.
- **Test tái hiện (GATE R, unit, adhoc/app):**
  - `TestCancelDuringFinalize_AbandonsMaterialization`: hook trong record creator gọi `svc.Cancel`; fake `CompleteFinalize` trả 409. Kỳ vọng abandoner được gọi 1 lần với record id tất định.
  - `TestRejectAfterFailedFinalize_AbandonsMaterialization`.
- **Thiết kế sửa:**
  - Interface tuỳ chọn `MaterializationAbandoner{AbandonMaterialization(ctx, companyID, recordID) error}`, type-assert trên `s.recordCreator`. Cách này không phải sửa `RecordCreator` và 4 fake test của nó.
  - Tách `deterministicRecordID()` ra khỏi `service.go:547,667`.
  - Gọi abandoner trong 2 trường hợp:
    - sau Reject/Cancel thành công (`applied==true`) từ trạng thái pending;
    - khi `CompleteFinalize` lỗi mà proposal chưa approved.
  - Adapter: `workflow.CancelOpenInstancesForRecordInternal`, một transaction: task pending → `cancelled`, instance `in_progress` → `rejected`.
  - Lỗi chỉ log, vì chuyển trạng thái proposal đã commit.
  - Không thêm trạng thái record mới (FE `DisclosureStatus` không có, `types.ts:817`).
- **Contract / schema / FE:** không. Record mồ côi vẫn hiện "In Progress" ở `/app/disclosures` (quyết định mở).
- **Dữ liệu DEV:**
  - Kiểm tra trước:
    ```sql
    SELECT wi.* FROM workflow_instances wi
    WHERE wi.status='in_progress' AND wi.workflow_source LIKE 'proposal_snapshot%'
      AND NOT EXISTS (SELECT 1 FROM ad_hoc_proposals p
                      WHERE p.company_id=wi.company_id AND p.record_id=wi.record_id AND p.status='approved');
    ```
  - Mồ côi ở chế độ legacy: cần dry-run bằng Go để suy ra id tất định của các proposal đã rejected/cancelled.
  - Sửa bằng chính các lệnh cancel ở trên.
- **Rollout / rollback:** chỉ BE.
- **Verify (GATE V):** unit test + `go test -race ./internal/adhoc/app/`.
- **Phạm vi ảnh hưởng:** chỉ đường Reject/Cancel; nếu không có record thì không làm gì.
- **Phụ thuộc / quyết định:** PR3-D4.
  - A: từ chối Reject/Cancel khi đã đủ phiếu, kiểm dưới `FOR UPDATE`.
  - B: bù trừ.
  - Khuyến nghị B; có thể thêm A cho Reject như một quy tắc sản phẩm.
- **Ước lượng:** M

### AHA-09 Email "đã duyệt" không tới người tạo ad-hoc; chuyển bước không báo ai
- **Mức độ / nguồn:** MEDIUM, confirmed (code).
- **Nguyên nhân gốc:** tầng notifier/SQL. Confirmed.
  - (09a) `LookupCreatorEmail` join `users.user_id = dr.created_by` (`workflow/infra/notification/notifier.go:122-129`). Với ad-hoc, `created_by` lưu membership id (`record_creator.go:80` → `disclosure/app/service.go:228`), nên email rỗng và lặng lẽ bị bỏ qua (`:57-60`).
  - (09b) `NextTask` (`service.go:449`) và task đầu tiên (`:117`) không gọi notifier nào.
- **Test tái hiện (GATE R):**
  - mysql-integration `internal/workflow/infra/notification/lookup_integration_test.go`:
    - `TestIntegration_LookupCreatorEmail_MembershipCreatedBy` (HEAD trả "");
    - `_InactiveMembershipNoEmail`.
  - Fallback khi chưa có DSN: unit test kiểm mã nguồn của query có tham chiếu `memberships`.
- **Thiết kế sửa:**
  - (09a):
    ```sql
    SELECT COALESCE(CASE WHEN mu.membership_id IS NOT NULL THEN u.email END, u2.email, ''), ...
    LEFT JOIN users u ON u.user_id=dr.created_by
    LEFT JOIN memberships mu ON mu.user_id=u.user_id AND mu.company_id=dr.company_id AND mu.membership_status='active'
    LEFT JOIN memberships m ON m.membership_id=dr.created_by AND m.company_id=dr.company_id AND m.membership_status='active'
    LEFT JOIN users u2 ON u2.user_id=m.user_id
    ```
    - Đồng thời ngừng gửi cho người đã rời company.
    - Không sửa dữ liệu `created_by`.
  - (09b), tuỳ chọn, sau flag `WORKFLOW_STEP_ASSIGNED_INAPP_ENABLED` (mặc định tắt):
    - thêm `NotifyTaskAssigned(...)` vào `WorkflowNotifier`;
    - chỉ in-app, kind `workflow.task_assigned`, `resource_type='disclosure'`;
    - người nhận chỉ gồm membership active.
    - Khuyến nghị: để PR riêng.
- **Contract / schema / FE:**
  - 09a: không.
  - 09b: kind mới được `inAppNotificationsApi.ts:25-35` link tới `/app/disclosures/:id`, nên tương thích.
- **Dữ liệu DEV:** không gửi bù.
- **Rollout / rollback:** chỉ BE.
- **Verify (GATE V):** DEV: hoàn tất task cuối của workflow ad-hoc → có dòng `email_notifications` gửi tới persona người tạo.
- **Phạm vi ảnh hưởng:** người nhận email `workflow.approved`. Record portal có người tạo đã rời company sẽ không còn nhận.
- **Phụ thuộc / quyết định:** PR3-D5.
- **Ước lượng:** S (09a) / M (09b)

### AHA-12 Sửa T0 trên draft làm mất hạn tuyệt đối
- **Mức độ / nguồn:** LOW, confirmed (code).
- **Nguyên nhân gốc:** tầng service. Confirmed.
  - `legacyDeadline` khởi tạo bằng `""` (`service.go:263-274`).
  - Patch chỉ có T0 sẽ tính lại với days=0, và `resolveProposedDeadline` trả nil (`deadline_fields.go:37-45`).
- **Test tái hiện (GATE R, unit):** `TestPatchDraft_T0Only_KeepsAbsoluteDeadline`:
  - Draft có `ProposedDeadlineDate="2026-10-20"`, days nil; patch chỉ đổi T0.
  - Kỳ vọng DTO trả về vẫn giữ ngày đó. HEAD trả nil.
- **Thiết kế sửa:** thêm `else if req.ProposedDeadlineDays == nil && cur.ProposedDeadlineDate != nil { legacyDeadline = *cur.ProposedDeadlineDate }`.
- **Contract / schema / FE:** không. WEB không bao giờ gửi `proposed_deadline` khi PATCH (`adHocAlertsApi.ts:131-152`).
- **Dữ liệu DEV:** không khôi phục được (không có lịch sử).
- **Rollout / rollback:** chỉ BE.
- **Verify (GATE V):**
  - Unit test ở trên.
  - DEV: gọi API PATCH chỉ đổi T0 trên một draft có hạn tuyệt đối.
- **Phạm vi ảnh hưởng:** client legacy/API dùng hạn tuyệt đối.
- **Phụ thuộc / quyết định:** không.
- **Ước lượng:** S

### AHA-13 Fixture trên DEV làm một instance bị kẹt
- **Mức độ / nguồn:** LOW, confirmed trên DEV.
- **Nguyên nhân gốc:** tầng dữ liệu/vận hành. Confirmed.
  - Script đang được track `deploy-artifacts/_tmp_fixture_complete_ready.sql` (commit `7cf7711`) sửa thẳng `workflow_tasks` thành `confirmed`, bỏ qua `ApplyTaskTransition`.
  - Kết quả: task `confirmed`, nhưng instance `01a0ba9c-5cf3-774b-b253-164511cf6c4b` vẫn `in_progress`; record `01a0ba9c-5c81-7a19-b75a-ac524480fbf7` vẫn Draft.
- **Test tái hiện (GATE R):** không có test code (lỗi dữ liệu). Các query kiểm tra trước ở dưới chính là bước tái hiện.
- **Thiết kế sửa:**
  - `git rm` script này. Đây là file `_tmp_*` duy nhất đang được track có SQL ghi dữ liệu.
  - Tuỳ chọn: thêm `/deploy-artifacts/_tmp_*` vào `.gitignore`.
- **Contract / schema / FE:** không.
- **Dữ liệu DEV (bản nháp):**
  - Kiểm tra trước:
    - dòng instance (source, status) và status / `submitted_at` của record;
    - các task của instance;
    - `COUNT(*)` instance của record: phải bằng 1, vì subquery trong script dùng `LIMIT 1` mà không có `ORDER BY`;
    - các dòng `workflow_instance_step_states` và `deadline_alert_confirmations`;
    - milestone chưa gửi có `scheduled_date < today`.
  - Sửa (transaction, user `qa_rw`):
    ```sql
    UPDATE workflow_tasks SET status='pending'
    WHERE workflow_instance_id='01a0ba9c-5cf3-774b-b253-164511cf6c4b' AND status='confirmed';
    -- kỳ vọng N dòng

    UPDATE workflow_step_milestones SET reminder_sent=1
    WHERE workflow_instance_id=… AND reminder_sent=0 AND scheduled_date < :today_hcm;
    -- tránh dồn reminder trễ kiểu RMC-02
    ```
  - Không thể hoàn tất instance này qua API, vì task không còn pending.
- **Rollout / rollback:** chỉ chạy khi được cho phép. Rollback: đặt lại đúng các task id đó về `confirmed`.
- **Verify (GATE V):**
  - Chạy lại các query kiểm tra trước.
  - `GET /api/v1/company/deadlines/{record}/steps`.
  - Log worker không có đợt gửi dồn.
- **Phạm vi ảnh hưởng:** 1 instance trên DEV.
- **Phụ thuộc / quyết định:** không.
- **Ước lượng:** S

---

## Thứ tự commit PR-3
Mỗi commit gồm test tái hiện + bản sửa. Dán kết quả test đỏ trên HEAD vào mô tả PR.
1. `fix(adhoc): rune-safe shortNote` (AHA-04)
2. `fix(adhoc): T0-only draft patch keeps absolute deadline` (AHA-12)
3. `fix(workflow): completion side effects follow instance status` (AHA-02)
4. `fix(workflow): resolve creator email via membership, active only` (AHA-09a)
5. `refactor(platform): workflowsel preferred instance; use in disclosure/deadlinealerts readers` (AHA-07 phía đọc, DLA-10). **PR-4 rebase lên commit này.**
6. `fix(adhoc): no submit bootstrap on ad-hoc; replay binds proposal instance` (AHA-07 phía ghi)
7. `fix(deadlinealerts): approved ad-hoc records in membership + parity` (AHA-01)
8. `fix(adhoc): final dates from all approvals; COALESCE final T0 in due` (AHA-03)
9. `fix(adhoc): submit re-validation + PrepareV3Materialization` (AHA-05)
10. `fix(adhoc): abandon materialization on reject/cancel/lost finalize` (AHA-06)
11. `chore: remove DEV fixture; ai-cache + draft repair SQL` (AHA-13)
12. Tuỳ chọn, sau flag: `feat(workflow): step-assigned in-app` (AHA-09b)

## Reviewer (mở song song trong một message)
- **be-security:** AHA-09a (chọn người nhận); AHA-05 (gate quyền); các hàm nội bộ không authz `GetWorkflowInstanceInternal` / `CancelOpenInstancesForRecordInternal`; AHA-06.
- **admin-role:** AHA-05 (kiểm quyền `disclosure.*` lúc submit; bắt buộc nếu chọn F1).
- **api-compat:** AHA-01 (membership danh sách, `total`); AHA-07 (giá trị `workflow_instance_id`); AHA-05 (validate chặt hơn); AHA-03 (giá trị field).
- **perf-reliability:** EXISTS/subquery tương quan trên từng dòng (AHA-01, AHA-07); bù trừ không có transaction (AHA-06).
- Không cần cache-versioning và secrets-cors.

## Checklist verify
- `gofmt -l internal` rỗng; `go vet ./...` pass.
- `go test ./internal/adhoc/... ./internal/deadlinealerts/... ./internal/workflow/... ./internal/disclosure/... ./internal/personalops/... ./internal/portaldashboard/...`
- `go test -race ./internal/adhoc/app/ ./internal/workflow/app/`
- `go test ./...`: tập fail có sẵn phải giữ nguyên.
- `MYSQL_TEST_DSN=… go test -run Integration ./internal/deadlinealerts/infra/mysql/ ./internal/disclosure/infra/mysql/ ./internal/workflow/infra/notification/ ./internal/adhoc/infra/mysql/`, hoặc ghi `BLOCKED`.
- Revert check: mỗi test tái hiện phải fail lại khi revert bản sửa tương ứng.
- `docker compose -f docker-compose.dev.yml build api`
- WEB: không đổi code.
- Smoke DEV sau `make deploy-be` (cần cho phép; persona qua `devqa_env.py`). Persona người tạo tạo proposal v3 với reviewer A và B, rồi submit:
  - A duyệt và chỉnh hạn, B duyệt với ô trống → ngày chốt là của A (AHA-03);
  - chỉ có đúng 1 instance `proposal_snapshot_v3` (AHA-07);
  - alert xuất hiện ở `/deadline-alerts` với hạn đúng; endpoint steps trả cùng instance (AHA-01/07);
  - Confirm ở task cuối → record thành Approved và có email `workflow.approved` gửi người tạo (AHA-02/09a);
  - ghi chú tiếng Việt dài → có dòng in-app (AHA-04).
- Sửa dữ liệu DEV (cần cho phép), theo thứ tự, mỗi bước có query kiểm tra trước/sau: AHA-13 → AHA-07 gắn lại → AHA-07 huỷ instance lạc → AHA-02 → AHA-06 mồ côi.
