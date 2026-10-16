# 01: Tái hiện (GATE R) và root cause

## GATE R: test fail trên code chưa sửa
| Lỗi | Test | Kết quả trước khi sửa |
|---|---|---|
| H1 | `internal/adhoc/infra/mysql/vote_rules_integration_test.go`: `TestAdhocVote_BlankFinalFieldsKeepTheEarlierEntry` (MySQL 8 thật) | Fail: A bỏ phiếu lại với ô trống thì `final t0=<nil> due=<nil> note=""`, mất giá trị A vừa nhập |
| H2 | `internal/adhoc/app/adhoc_v12_rules_test.go`: `TestReject_RefusedOnceEveryReviewerApproved`, `TestCancel_RefusedOnceEveryReviewerApproved` | Fail: `err=<nil>`, reject/cancel vẫn được dù mọi reviewer đã duyệt |
| H3 | `TestApprove_FinalizeFailureReleasesTheLastVote` | Fail: sau khi finalize lỗi vẫn còn đủ 2 phiếu, đề xuất kẹt |
| H3 | `TestBuildCreateRecordOptsForFinalize_ResumesWorkflowOnDuplicate`; `internal/adhoc/infra/disclosure/record_creator_test.go`: `TestCreateAndSubmitRecordWithOpts_AdhocFinalizeRetryResumesWorkflow` | Fail: lần thử lại trả `workflow=""` |
| XL-06 | `TestSubmitProposal_RechecksReviewers`, `TestSubmitProposal_RequiresProposerRecordCreatePermission` | Fail: submit thành công dù reviewer inactive, mất quyền duyệt, hoặc người đề xuất không có `disclosure.create` |

Đối chứng `TestRejectAndCancel_AllowedWhileAVoteIsOutstanding` pass: reject/cancel khi còn phiếu chưa bỏ vẫn được.

## Root cause (đã xác nhận bằng test và code)
- **H1.** Khi finalize, ngày chốt được lấy từ input của phiếu cuối cùng (`ReserveVote` trả `LastFinal* = in.Final*`, `internal/adhoc/infra/mysql/repository.go:518-520`). Ngoài ra, khi một reviewer bỏ phiếu lại, câu `ON DUPLICATE KEY UPDATE` ghi đè ô trống thành NULL (`repository.go:490-495`). Không có chỗ nào giữ "giá trị hiện hành" qua các phiếu. Tầng: repository.
- **H2.** `Reject` và `Cancel` chỉ kiểm trạng thái, không kiểm đã đủ phiếu chưa (`internal/adhoc/app/service.go:760-843`). Lệnh UPDATE có điều kiện chỉ khoá theo `status`. Tầng: service và repository.
- **H3.** Có hai đường dẫn tới lỗi:
  - **Kẹt luồng.** Phiếu cuối được ghi và commit trước khi finalize chạy (`service.go:490-531`). Khi finalize lỗi, phiếu đó vẫn ở lại. FE chỉ hiện nút Duyệt cho reviewer chưa bỏ phiếu (`AdHocProposalDetailPage.tsx:323-327`), và theo v1.2 thì reject/cancel đều bị chặn. Đề xuất không còn đường đi tiếp.
  - **Duyệt không có workflow.** Lần thử lại tạo record với ID cố định và gặp lỗi ID trùng. Vì `BuildCreateRecordOptsForFinalize` không bật `ResumeWorkflowOnDuplicate`, `record_creator.go:121-123` trả về record cũ với `WorkflowInstanceID` rỗng, rồi `CompleteFinalize` chuyển proposal sang `approved`.

  Tầng: service, adapter và FE.
- **XL-06.** `SubmitProposal` chỉ kiểm `len(reviewers) > 0` (`service.go:382-388`). Các kiểm tra active/quyền chỉ chạy lúc tạo (`service.go:169-178`), dù reviewer có thể bị vô hiệu hoá hoặc mất quyền trong lúc đề xuất còn là nháp. Quyền tạo hồ sơ của người đề xuất chỉ được kiểm khi finalize, tức là sau khi mọi người đã duyệt. Tầng: service.

## Phát hiện phụ (ngoài phạm vi, chỉ ghi lại)
- `migrations/run_dev_migrations.sh` thiếu 4 migration: 0038, 0098, 0099, 0150. DB dựng theo danh sách này không có bảng reviewer/approval của ad-hoc.
- Câu backfill trong 0098 dùng `SELECT id`, nhưng bảng chỉ có cột `proposal_id`, nên câu này lỗi. Bảng vẫn được tạo vì câu tạo bảng chạy trước.
