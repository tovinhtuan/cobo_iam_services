# 02: Sửa lỗi và kiểm chứng (chưa commit, chưa deploy)

Đã gồm các bản sửa sau review (`03-review.md`).

## Thay đổi
### BE `cobo_iam_services`
| File | Nội dung |
|---|---|
| `internal/adhoc/app/contracts.go` | Xem danh sách bên dưới |
| `internal/adhoc/app/field_errors.go` | Lỗi 409 với `details.reason`: `all_reviewers_approved`, `finalize_in_progress`, `final_values_locked` |
| `internal/adhoc/app/service.go` | Xem danh sách bên dưới |
| `internal/adhoc/app/runtime_materialize.go` | `BuildCreateRecordOptsForFinalize` bật `ResumeWorkflowOnDuplicate`. Áp cho cả finalize v3 và `AdminApprove` cũ |
| `internal/adhoc/infra/disclosure/record_creator.go` | Khi resume record đã có `SubmittedAt` thì không submit lại, vì submit lần hai sẽ chuyển `PendingReview` thành `In Progress` |
| `internal/adhoc/infra/mysql/repository.go` | Xem danh sách bên dưới |
| `docs/api-contracts-json.md`, `docs/WebPortal.postman_collection.json` | Mục "Ad-hoc proposal — v1.2 vote rules (2026-10-16)". Thêm 2 request `/approve`: bỏ phiếu v3 và thử lại hoàn tất |

`internal/adhoc/app/contracts.go`:
- `StatusUpdate.RefuseIfAllApproved`;
- `Repository.ReleaseVote(…, recordID) (bool, error)` và `Repository.LockFinalize`;
- `ReserveVoteInput.FinalizeRecordID`;
- `ProposalDTO.finalize_retry_available`.

`internal/adhoc/app/service.go`:
- **Reject và Cancel:** bật `RefuseIfAllApproved`.
- **Approve:**
  - giữ khoá finalize theo proposal;
  - dưới khoá, đọc lại proposal và số phiếu (`reloadForFinalize`); finalize từ giá trị hiện hành trên dòng proposal;
  - nếu finalize lỗi, `releaseVoteAfterFailedFinalize` chạy bằng context không bị huỷ và có log.
- **Finalize:** đọc T0 bằng `isoDate`, nên chấp nhận cả dạng RFC 3339.
- **Submit:** `recheckSubmitParties`:
  - reviewer phải active và có `ad_hoc_alert.focal_review`, nếu không trả 422;
  - người đề xuất phải có `disclosure.create` và `disclosure.view`, nếu không trả 403;
  - lỗi của chính bước kiểm tra trả 5xx.
- **`embedReviewState`:** đặt cờ `finalize_retry_available`.
- `AdHocRecordID` dùng chung cho finalize và `AdminApprove`.

`internal/adhoc/infra/mysql/repository.go`:
- **`ReserveVote`:**
  - nếu record finalize đã tồn tại mà phiếu đổi giá trị chốt thì trả 409;
  - ODKU: ô trống giữ giá trị cũ; bỏ phiếu lại không đổi `approved_at` và không xoá comment;
  - dòng proposal mang giá trị hiện hành: `COALESCE(giá trị nhập, giá trị hiện hành, giá trị mới nhất trong các phiếu cũ)`, với tie-breaker `membership_id`;
  - đọc lại các ngày bằng `DATE_FORMAT`.
- **`UpdateStatus`:** khi bật cờ, khoá dòng proposal rồi đếm số phiếu, trong cùng transaction.
- **`CompleteFinalize`:** chạy trong transaction; không duyệt khi đã thiếu phiếu.
- **`ReleaseVote`:** chỉ xoá phiếu khi proposal còn chờ và chưa có record finalize.
- **`LockFinalize`:** khoá `GET_LOCK` trên một connection riêng, chờ tối đa 5 giây; theo mẫu `LockCompanyAdmins`.

### FE `cobo_web_design`
| File | Nội dung |
|---|---|
| `src/pages/portal/AdHocProposalDetailPage.tsx` | Xem danh sách bên dưới |
| `src/pages/portal/AdHocProposalCreatePage.tsx` | "Gửi đề xuất" sau khi draft đã tạo mà submit bị từ chối (422 reviewer, 403 `disclosure.*`): mở trang draft kèm lý do, không tạo thêm draft |
| `src/pages/portal/adhoc/adHocProposalErrors.ts` | Map `all_reviewers_approved`, `finalize_in_progress`, 422 reviewer và 403 `disclosure.create`/`disclosure.view` |
| `src/services/adHocAlertsApi.ts` | Submit dùng `suppressForbiddenNavigation`: lỗi 403 hiện lý do trên trang thay vì chuyển sang `/app/forbidden` |
| `src/services/workflowOverrideMappers.ts`, `src/types.ts` | Thêm `finalize_retry_available` |

`src/pages/portal/AdHocProposalDetailPage.tsx`:
- Ẩn Từ chối/Rút và hiện "Hoàn tất phê duyệt" **chỉ khi server gửi `finalize_retry_available`**. Với server cũ, trang giữ hành vi trước đây (review API-01).
- `doAction` tải lại trang sau khi gặp lỗi.
- Submit ở trang này fallback về câu của `mapAdHocProposalSubmitError`.
- Modal duyệt hiện giá trị hiện hành và mô tả đúng quy tắc v1.2.
- Trang đọc `submitErrorMessage` do trang tạo đề xuất truyền sang.

### Test
- **BE app:** `internal/adhoc/app/adhoc_v12_rules_test.go` (13 test). Sửa các fake trong `service_test.go`. `TestSubmitProposalAutoApproveIsNoOp` truyền validator (validator nil là lỗi cấu hình). `raceFakeRepo` trả reviewer và approval khớp với `ReserveVote` của nó.
- **BE record creator:** `TestCreateAndSubmitRecordWithOpts_AdhocFinalizeRetryResumesWorkflow`.
- **BE MySQL 8:** `internal/adhoc/infra/mysql/vote_rules_integration_test.go` (9 test), gồm test xen kẽ reject với phiếu cuối và test khoá finalize.
- **FE:**
  - `AdHocProposalDetailPage.v12-actions.test.tsx` (9 test);
  - thêm 1 test trong `AdHocProposalCreatePage.layout.test.tsx`.

## Hành vi sau khi sửa
- **Ngày chốt và ghi chú:**
  - mỗi trường lấy giá trị được nhập sau cùng; ô trống giữ giá trị trước;
  - proposal đang chờ duyệt mang "giá trị hiện hành", và modal hiển thị giá trị đó;
  - khi record finalize đã có, các giá trị bị khoá (409 `final_values_locked`), để không lệch với T0 mà workflow đang dùng.
- **Đủ phiếu:** từ chối hoặc rút lại trả 409 `all_reviewers_approved`. Việc kiểm tra chạy dưới cùng khoá dòng mà phiếu bầu dùng.
- **Finalize:**
  - mỗi proposal chỉ có một finalize chạy tại một thời điểm; request phải chờ quá 5 giây thì nhận 409 `finalize_in_progress`;
  - dưới khoá, nếu thiếu phiếu thì trả "chưa hoàn tất"; nếu đã duyệt thì trả kết quả cũ.
- **Finalize lỗi:**
  - chưa tạo record: nhả phiếu của người vừa bấm; họ duyệt lại được, và reviewer khác vẫn từ chối được;
  - đã tạo record: giữ nguyên các phiếu; từ chối hoặc rút lại vẫn bị chặn; server gửi `finalize_retry_available`, reviewer bấm "Hoàn tất phê duyệt" để resume record và workflow, không submit lại.
- **Submit:**
  - reviewer inactive hoặc mất quyền: 422;
  - người đề xuất thiếu `disclosure.create`/`disclosure.view`: 403;
  - lỗi hạ tầng: 5xx.
  - Ở trang tạo đề xuất, draft đã lưu được mở ra kèm lý do.
- **T0 đề xuất:** khi không ai nhập T0 chốt, finalize truyền đúng T0 đề xuất cho workflow. Trước đây chuỗi RFC 3339 parse lỗi nên workflow không nhận T0 (DB-05, lỗi có từ trước).

## Kiểm chứng
| Lệnh / test | Kết quả |
|---|---|
| GATE R | BE 7 test fail đúng lỗi (MySQL 1, app 5, record creator 1), FE 5 test fail; test đối chứng pass (`01-root-cause.md`) |
| GATE V vòng 1 (bản sửa đầu) | BE 9/9, FE 6/6: mỗi lần tắt một hành vi đều có test fail; khôi phục thì pass |
| GATE V vòng 2 (bản sửa sau review) | BE W1–W9: bỏ khoá finalize, bỏ đọc lại số phiếu, nhả phiếu dù đã có record, bỏ khoá giá trị chốt, đổi `approved_at`/xoá comment khi bỏ phiếu lại, không parse T0 dạng RFC 3339, không gửi cờ, trả lỗi hạ tầng thành 422, bỏ kiểm `disclosure.view`. FE G1–G3: cờ, thông báo `finalize_in_progress`, mở draft ở trang tạo. Mỗi lần đều có test fail; khôi phục (file giống hệt bản lưu) thì pass |
| `go test ./internal/adhoc/...` | ok |
| MySQL 8.0 thật (`cobo_iam_mr1`, áp thêm 0098/0099): `-run AdhocVote -count=3` | 9/9 pass cả 3 lần |
| `go test ./...` so với HEAD `28142fb` | 0 fail mới (20 fail có sẵn, cùng danh sách) |
| `go vet ./...` | sạch (cảnh báo có sẵn ở `internal/workflowfulfillment/required_document_gate_test.go`) |
| `gofmt -l` trên file đã đổi/mới | sạch |
| `docker compose -f docker-compose.dev.yml build api` | Built |
| `npx vitest run` toàn bộ FE so với HEAD (`git archive`) | 0 fail mới (48 fail có sẵn ở cả hai, cùng danh sách); 3206 pass so với 3196 ở HEAD |
| `npm run lint` | 12 lỗi TS, giống hệt HEAD |
| `npm run build`, `npm run check:mojibake` | OK |

## Blast radius và dữ liệu
- `AdminApprove` cũ dùng chung `BuildCreateRecordOptsForFinalize` và `AdHocRecordID`. Nó có claim idempotency riêng (`ReserveAdminApproval`) và không đi qua khoá finalize.
- Luồng periodic không đổi: vốn đã resume workflow, không gọi submit, và không đi qua code vote.
- Chỗ đọc `ad_hoc_proposals.final_*`: `deadlinealerts` và `personalops` chỉ đọc proposal `approved`. Danh sách và chi tiết trả thêm giá trị hiện hành cho proposal đang chờ; FE chỉ dùng giá trị này trong modal.
- **DEV** (`qa_ro`, 16/10):
  - 0 proposal approved bị mất ngày chốt;
  - 0 proposal reject/cancel sau khi đủ phiếu;
  - 0 proposal approved thiếu record hoặc workflow;
  - 0 proposal kẹt; 0 proposal đang mở có reviewer inactive;
  - 0 proposal đang chờ còn dựa vào giá trị trong phiếu cũ (phần fallback của DB-04).
  - **2 record ad-hoc có 2 workflow instance:** một `proposal_snapshot_v3` và một template (`global_template`, `company_override`), tạo cùng giây vào 2026-09-20 và 2026-10-14 02:16 UTC. Cả hai có từ trước bản sửa `SkipWorkflowBootstrap` (`06877e7`, 2026-10-14 02:41 UTC), không phải do race của MR-2. Cần kế hoạch dọn riêng (giữ instance v3, đóng instance template); không tự chạy.
- **Prod:** chạy các truy vấn chỉ đọc dưới đây trước khi deploy. Nếu có kết quả, lập kế hoạch sửa dữ liệu riêng; không tự chạy.

```sql
-- H1: proposal đã duyệt mà mất ngày chốt của người duyệt trước
SELECT p.proposal_id FROM ad_hoc_proposals p WHERE p.status = 'approved' AND (
  (p.final_t0_date IS NULL AND EXISTS (SELECT 1 FROM ad_hoc_proposal_approvals a WHERE a.proposal_id = p.proposal_id AND a.final_t0_date IS NOT NULL))
  OR (p.final_deadline_date IS NULL AND EXISTS (SELECT 1 FROM ad_hoc_proposal_approvals a WHERE a.proposal_id = p.proposal_id AND a.final_deadline_date IS NOT NULL)));
-- H2: bị từ chối hoặc rút lại sau khi đủ phiếu
SELECT p.proposal_id, p.status FROM ad_hoc_proposals p WHERE p.status IN ('rejected','cancelled')
  AND (SELECT COUNT(*) FROM ad_hoc_proposal_reviewers r WHERE r.proposal_id = p.proposal_id) > 0
  AND (SELECT COUNT(*) FROM ad_hoc_proposal_approvals a WHERE a.proposal_id = p.proposal_id)
   >= (SELECT COUNT(*) FROM ad_hoc_proposal_reviewers r WHERE r.proposal_id = p.proposal_id);
-- H3: đã duyệt mà thiếu record/workflow; đang chờ mà đã đủ phiếu (kẹt)
SELECT proposal_id FROM ad_hoc_proposals WHERE status = 'approved' AND (record_id IS NULL OR workflow_instance_id IS NULL);
SELECT p.proposal_id FROM ad_hoc_proposals p WHERE p.status = 'pending_focal_approval'
  AND (SELECT COUNT(*) FROM ad_hoc_proposal_reviewers r WHERE r.proposal_id = p.proposal_id) > 0
  AND (SELECT COUNT(*) FROM ad_hoc_proposal_approvals a WHERE a.proposal_id = p.proposal_id)
   >= (SELECT COUNT(*) FROM ad_hoc_proposal_reviewers r WHERE r.proposal_id = p.proposal_id);
-- DB-01: record có nhiều workflow instance
SELECT record_id, COUNT(*) FROM workflow_instances GROUP BY record_id HAVING COUNT(*) > 1;
-- DB-04: proposal đang chờ còn dựa vào giá trị trong phiếu cũ
SELECT DISTINCT p.proposal_id FROM ad_hoc_proposals p JOIN ad_hoc_proposal_approvals a ON a.proposal_id = p.proposal_id
WHERE p.status = 'pending_focal_approval' AND p.final_t0_date IS NULL AND p.final_deadline_date IS NULL
  AND (a.final_t0_date IS NOT NULL OR a.final_deadline_date IS NOT NULL OR COALESCE(a.adjustment_note, '') <> '');
```

Nếu sau deploy còn proposal "đang chờ mà đủ phiếu", reviewer bấm "Hoàn tất phê duyệt" để resume. Cách này không cần sửa dữ liệu.

## Tương thích và rollout
- Không có migration, không có cờ tính năng.
- **Deploy BE trước, FE sau. Khi rollback: rollback FE trước, BE sau.**
- **FE cũ với BE mới:** chạy được nhưng UX kém hơn:
  - Từ chối/Rút khi đủ phiếu bị 409 và hiện thông báo chung chung;
  - submit 403 sẽ chuyển sang `/app/forbidden`;
  - phiếu bị nhả thì nút Duyệt hiện lại.
- **FE mới với BE cũ:** an toàn. BE cũ không gửi `finalize_retry_available`, nên FE không hiện nút thử lại và không ẩn Từ chối/Rút. Các mapper lỗi mới không bao giờ được kích hoạt.
- **Tab mở từ trước:** không biết nghĩa mới của ô để trống (API-03); đây là quy tắc chủ đích của v1.2.
- **Rollback BE:**
  - `final_*` đã ghi trên proposal đang chờ vẫn còn; bản cũ ghi đè chúng khi duyệt;
  - phiếu đã nhả thì không khôi phục được; người duyệt bỏ phiếu lại.
