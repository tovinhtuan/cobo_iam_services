# 03: Review (wf-bugfix Phase 6)

Ba reviewer chạy song song trên bản sửa đầu: `db-tx-reviewer`, `be-security-reviewer` (gồm cả phía FE) và `api-compat-reviewer`. Mọi phát hiện High/Medium đều được tự kiểm lại trên code trước khi sửa.

## Phát hiện và xử lý
| ID | Mức | Nội dung | Xử lý |
|---|---|---|---|
| DB-01 = BES-02 | HIGH | Hai finalize chạy song song: double-click, client retry, hoặc nút thử lại bấm đúng lúc finalize khác đang chạy. Cả hai cùng resume record, vì bản sửa đầu đã bật `ResumeWorkflowOnDuplicate`. Bảng `workflow_instances` không có unique index trên `record_id`, nên sinh được 2 instance | Đã sửa: `LockFinalize` (khoá tên theo proposal, chờ tối đa 5 giây, nếu không thì 409 `finalize_in_progress`). Dưới khoá, đọc lại proposal và số phiếu. Test: `TestAdhocVote_FinalizeLockSerializesFinalize` (MySQL), `TestApprove_RechecksVotesUnderTheFinalizeLock` |
| DB-02 = BES-01 = API-06 | MEDIUM | Bù trừ nhả phiếu cả khi finalize đã tạo, hoặc đã submit, record. Sau đó reject/cancel lại được và để lại record (có thể cả workflow) mồ côi. Ngoài ra lời gọi nhả phiếu dùng context của request, nên có thể bị huỷ theo | Đã sửa: `ReleaseVote` chỉ xoá phiếu khi chưa có record finalize (kiểm dưới khoá dòng proposal) và chạy bằng `context.WithoutCancel` có timeout. Khi đã có record, các phiếu giữ nguyên và retry sẽ resume. Test: `TestAdhocVote_FinalizeRecordKeepsTheVotes`, `TestApprove_FinalizeFailureAfterTheRecordKeepsTheVotes` |
| DB-03 | MEDIUM | Ngày chốt có thể lệch với T0 mà workflow đã dùng, nếu sau khi record đã tạo có người bỏ phiếu lại với giá trị khác (qua API) | Đã sửa: khi record finalize đã tồn tại, phiếu đổi giá trị trả 409 `final_values_locked`. Finalize lấy giá trị từ dòng proposal đọc dưới khoá. Test: `TestAdhocVote_FinalValuesFreezeOnceTheRecordExists` |
| API-01 | MEDIUM | FE mới chạy với BE cũ: nút thử lại gửi `{}`, BE cũ xoá ngày chốt và có thể duyệt mà không có workflow | Đã sửa: server gửi `finalize_retry_available`. FE chỉ hiện nút thử lại và chỉ ẩn Từ chối/Rút khi có cờ này. Test FE "keeps the earlier actions against a server that does not send the retry flag" |
| API-02 | MEDIUM | Tắt chuyển hướng 403 cho submit ảnh hưởng cả trang tạo đề xuất: trang hiện câu tiếng Anh thô, và mỗi lần bấm lại tạo thêm một draft | Đã sửa: trang tạo mở draft vừa lưu kèm lý do (422 reviewer, 403 `disclosure.*`). Trang chi tiết fallback về câu đã map. Test trong `AdHocProposalCreatePage.layout.test.tsx` |
| BES-03 | LOW | Đường resume gọi `GetRecord` với tư cách người đề xuất, nên cần `disclosure.view` | Đã sửa: submit kiểm thêm `disclosure.view`. Test `TestSubmitProposal_RequiresProposerRecordViewPermission` |
| BES-04 = DB-04 | LOW | Bỏ phiếu lại thì `approved_at` bị đổi và comment bị xoá. Fallback cho phiếu cũ không có tie-breaker. Nhả phiếu không để lại vết | Đã sửa: ODKU giữ `approved_at` và comment; thêm `membership_id DESC` làm tie-breaker; log `adhoc: finalize failed` kèm `vote_released`. DEV có 0 proposal phụ thuộc fallback. Test `TestAdhocVote_RepeatVoteKeepsApprovalTimeAndComment` |
| BES-05 = DB-06 | LOW | Lỗi hạ tầng khi kiểm reviewer bị trả thành 422 "reviewer không hợp lệ" | Đã sửa: trả lỗi gốc (5xx). Test `TestSubmitProposal_ReviewerCheckFailureIsNotAReviewerError` |
| DB-05 | LOW (có từ trước) | T0 đề xuất đọc từ cột DATE có dạng RFC 3339 khi `parseTime=true`, nên `time.Parse("2006-01-02")` lỗi và workflow không nhận T0 | Đã sửa trong cùng hàm finalize bằng `isoDate`. Test `TestApprove_UsesTheProposedT0ReadAsRFC3339` |
| API-03 | LOW | Tab FE mở từ trước không biết nghĩa mới của ô để trống | Ghi vào contract doc. Không cần đổi code |
| API-04 | LOW | Contract doc và Postman chưa cập nhật | Đã cập nhật `docs/api-contracts-json.md` và thêm 2 request `/approve` vào Postman |
| API-05 | LOW | `final_*` trả ra dạng RFC 3339 thay vì YYYY-MM-DD (có từ trước) | Follow-up riêng: đọc `DATE_FORMAT` trong các cột của DTO. FE đã cắt lấy phần ngày trong modal |

## Kết luận từng vùng sau khi sửa
- **Thứ tự khoá:** không có chu trình (db-tx-reviewer).
  - Mọi transaction đều khoá dòng proposal trước.
  - Bước tạo record/workflow chạy ngoài các transaction này, nhưng dưới khoá tên `LockFinalize`.
- **Phân quyền:**
  - kiểm `disclosure.create` lúc submit tương đương với kiểm tra ở `CreateRecord`/`SubmitRecord`;
  - thêm `disclosure.view` cho đường resume;
  - mọi câu SQL mới đều có `company_id`;
  - `ReleaseVote` chỉ xoá phiếu của chính người gọi (membership lấy từ token).
- **Tương thích:** thứ tự deploy BE rồi FE; rollback FE rồi BE. Ma trận ở `02-fix.md`.

## Follow-up ngoài phạm vi
- API-05: chuẩn hoá ngày `final_*` trong DTO thành YYYY-MM-DD.
- Instance workflow idempotent theo record ở tầng workflow: khoá record rồi trả instance đã có. Cách này bảo vệ cả các luồng khác ngoài ad-hoc (DB-01, đề xuất của reviewer).
- Dọn 2 record ad-hoc trên DEV có thêm instance template, sinh ra từ trước `06877e7`.
- Ngoài delta: `lockStepCompleted` coi mọi lỗi INSERT là "insert đồng thời"; approve chưa có `MaxBytesReader`.
- Danh sách migration dev thiếu 4 file, và backfill trong 0098 dùng sai tên cột: đã tách thành task riêng.
