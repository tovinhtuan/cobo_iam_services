# 06: Review song song (Phase 6 của MR-4)

Ba reviewer read-only chạy song song trên diff chưa commit của cả hai repo, 16/10/2026:

- `db-tx-reviewer`: transaction, khoá, migration.
- `admin-role-reviewer`: quyền duyệt, tenant, IDOR.
- `api-compat-reviewer`: contract FE/BE và deploy lệch phiên bản.

`api-compat-reviewer` đọc lúc các bản sửa DB-01/DB-02 đang được ghi, nên API-01 là trạng thái giữa chừng; bản sửa cuối cùng đã xử lý nó.

Không có finding CRITICAL. Finding HIGH duy nhất (API-01) đã sửa.

## Kết quả xử lý
| ID | Mức | Nội dung | Xử lý |
|---|---|---|---|
| API-01 | HIGH | `publish:true` đã kích hoạt rồi vẫn trả 409, không có audit | **Đã sửa**: repo lưu và kích hoạt trong một transaction; service bỏ lời gọi approve thứ hai; in-memory cùng ngữ nghĩa. Test `TestOverrideReview_PublishActivatesAtOnce` (in-memory) và `TestOverrideReview_VersionsAreImmutableOnMySQL` (MySQL) |
| DB-01 | MEDIUM | `publish:true` dùng hai transaction | **Đã sửa**, cùng bản sửa API-01 |
| DB-02 / ROLE-03 | MEDIUM / LOW | Duyệt `draft` không gắn với nội dung: step-groups sửa tại chỗ; xoá rồi lưu dùng lại số version | **Đã sửa**: step-groups ghi version mới `v:N+1`; DELETE giữ dòng `discarded`; GET ẩn `discarded`; stale check bỏ qua `discarded`. Test MySQL + in-memory |
| ROLE-01 | MEDIUM | Người duyệt không thấy nội dung bản được duyệt | **Đã sửa (FE)**: panel duyệt có phần xem nội dung chỉ đọc, đánh dấu bước mới/đã sửa/bị bỏ so với bản đang dùng; dùng được khi chỉ có quyền đọc + duyệt |
| ROLE-02 | MEDIUM | Rebase-apply áp giá trị xử lý xung đột do người khác chọn, không audit; `self_approved=true` cứng | **Đã sửa**: audit `conflict_resolved`; `self_approved`/`resolved_by` tính từ người xử lý xung đột (response + audit); FE hiện giá trị `merge_manual` đã lưu |
| API-02 | MEDIUM | Xung đột `merge_manual {}` đã lưu làm rebase-apply kẹt 422 | **Không cần sửa trên DEV**: 0 dòng `workflow_override_conflicts` (truy vấn chỉ đọc). Ghi bước kiểm dữ liệu cho môi trường khác vào 02-contract |
| API-03 | MEDIUM | Xung đột chỉ có `merge_manual` không xử lý được trên Portal | **Giảm thiểu**: `__step_existence__` nhận object bước qua API; FE hiện ghi chú khi không có lựa chọn. Rule 5/7 cần ô nhập vai trò: follow-up (đã chốt ở GATE A). DEV: 0 xung đột |
| API-04 | MEDIUM | MySQL không kiểm resolution thuộc lựa chọn | **Đã sửa**: lựa chọn lấy theo `conflict_type` (`ResolutionOptionsForConflictType`). Test |
| API-05 / DB-03 | MEDIUM / LOW | Duyệt hai lần trả 404 | **Đã sửa**: 409 `STATE_CONFLICT`. Test |
| API-06 | MEDIUM | BE mới phụ thuộc 0156; `push-migration.sh` luôn exit 0 | **Gate deploy**: chạy 0156, kiểm `information_schema` đủ 6 cột rồi mới deploy BE. Sửa script là follow-up có sẵn (`migration-runner-gaps-2026-10-16.md`) |
| DB-04 / API-15 | LOW | Runbook rollback sai | **Đã sửa** trong 02-contract: sao lưu, chỉ version mới nhất `pending_approval` về `draft`, giữ `rejected`/`discarded` |
| DB-05 | LOW | Timestamp review dùng `CURRENT_TIMESTAMP` | **Đã sửa**: dùng thời gian Go (UTC) |
| DB-06 / API-07 | LOW | Ghi chú gửi duyệt và lý do từ chối dùng chung cột | **Đã sửa**: cột `submit_note` (0156 chưa lên DEV); FE đọc `submit_note`. Test giữ ghi chú sau khi từ chối |
| DB-07 | LOW | Rebase-apply thay thế bản chờ duyệt mà không ai thấy | **Đã sửa**: 409 `STATE_CONFLICT`. Test MySQL + in-memory |
| ROLE-04 | LOW | 403 của `publish:true` chuyển trang Forbidden, mất chỉnh sửa | **Đã sửa (FE)**: `suppressForbiddenNavigation`, guard quyền duyệt, báo tại chỗ |
| ROLE-05 | LOW | Quyền ghi cấp trực tiếp mà không có quyền duyệt | **Đã kiểm (chỉ đọc)**: DEV có 4 membership như vậy; sau MR-4 họ dùng "Gửi duyệt", đúng Q5-B. Ghi vào bảng tương thích |
| ROLE-06 | LOW | `merge_manual` cho `stage`/`documents`/`groups` yếu hơn contract | **Đã sửa**: `documents`/`groups` không nhận `merge_manual`; `stage` tối đa 255 ký tự. Contract cập nhật |
| ROLE-07 | LOW | Danh sách chờ duyệt có thể lộ tên loại thông tin của công ty khác | **Đã sửa**: join chỉ lấy loại hệ thống hoặc loại cùng công ty. Kiểm loại thông tin khi PUT draft (có từ trước): follow-up |
| API-08 | LOW | Lỗi resolve bị nuốt | **Đã sửa (FE)**: báo lỗi tại chỗ theo mã |
| API-09 | LOW | FE cũ + người chỉ có quyền ghi: 403 chuyển trang | **Chấp nhận**: deploy FE ngay sau BE |
| API-10 | LOW | Tạo loại nhanh (ad-hoc) không còn bản nháp để điền lại khi publish lỗi | **Chấp nhận / follow-up**: luồng này chỉ bật khi có quyền duyệt |
| API-11 | LOW | Thẻ tổng quan gọi mọi bản mở là "Bản nháp" | **Đã sửa (FE)**: nhãn theo trạng thái |
| API-12 | LOW | Siết stale check và step-groups | **Ghi tài liệu** trong api-contracts-json |
| API-13 | LOW | Body hỏng ở submit/reject trả 500 | **Đã sửa**: 400 `INVALID_REQUEST` |
| API-14 | LOW | Lệch tài liệu, thiếu nhãn audit | **Đã sửa**: 02-contract, api-contracts-json, hai file inventory, nhãn audit FE |
| ROLE-08 | INFO | Audit thiếu khi bước duyệt của publish lỗi | Hết áp dụng sau DB-01 |
| ROLE-09 | INFO | `canActivate`/`canApply` mặc định `true` | **Đã sửa (FE)**: mặc định `false` |
| DB-08, DB-09, API-16 | INFO | Thứ tự deploy, thiếu test | Thứ tự deploy ghi trong 02-contract; test bổ sung ở bảng dưới |

## Test bổ sung sau review
- In-memory (`internal/disclosure/app/workflow_override_review_test.go`):
  - `TestOverrideReview_PublishActivatesAtOnce`
  - `TestOverrideReview_ApproveTwiceIsAStateConflict`
  - `TestOverrideReview_SubmitNoteIsKeptAfterRejection`
  - `TestOverrideReview_DeletedDraftNumberIsNotReused`
  - `TestRebaseApply_RefusedWhileASubmissionIsPending`
  - `TestResolutionOptionsForConflictType`
  - phần lựa chọn lấy theo loại xung đột trong `TestResolveConflict_MergeManual…`
  - các trường hợp mới trong `TestValidateMergeManualValue`
- MySQL (`override_review_integration_test.go`): `TestOverrideReview_VersionsAreImmutableOnMySQL` kiểm:
  - publish trong một transaction;
  - `discarded` không dùng lại số version;
  - step-groups tạo `v:N+1`, nên duyệt `v:N` bị `STALE_ETAG`;
  - step-groups trên bản chờ duyệt trả 409;
  - rebase-apply khi có bản chờ duyệt trả 409.

  Ngoài ra kiểm `submit_note` trong danh sách chờ duyệt.

## Còn thiếu (ghi nhận)
- Test handler cho meta audit (`self_approved`, `conflict_resolved`) và không ghi audit khi 403.
- Test MySQL chéo tenant cho approve/submit/reject/pending (tenant được bảo vệ bởi `company_id` của token trong mọi truy vấn; đã có test in-memory cho danh sách chờ duyệt của công ty khác).
- Test FE cho persona chỉ có quyền duyệt (không có quyền ghi), đi qua phần xem nội dung.
