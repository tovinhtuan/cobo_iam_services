# 01: Thiết kế, MR-4

## Nguyên tắc
- Một quy tắc kích hoạt duy nhất: làm một version thành bản đang dùng thì cần quyền **duyệt**. Quyền ghi chỉ lưu nháp và gửi duyệt.
- Không còn chặn tự duyệt. Người có quyền duyệt được kích hoạt bản mình soạn, và audit ghi `self_approved=true` (Q5-B). Doanh nghiệp muốn kiểm soát bốn mắt thì tách role (phương án C để sau).
- Chỉ **version mới nhất** (`MAX(version_no)`) mới được gửi duyệt, duyệt hay từ chối. Mỗi lần lưu tạo version mới, nên bản cũ tự bị thay thế. Quy tắc này sửa luôn WF-08c (duyệt bản nháp cũ sau rebase).
- Kiểm quyền **trước** mọi side effect.

## Trạng thái version
| Từ | Sự kiện | Quyền | Tới | Audit |
|---|---|---|---|---|
| (mới) | Lưu nháp (PUT draft) | ghi | `draft` | `draft_saved` (như cũ) |
| `draft` | Gửi duyệt | ghi | `pending_approval` | `submitted` |
| `draft` | Kích hoạt (PUT `publish:true`, hoặc POST approve) | duyệt | `approved`, thành bản đang dùng | `applied`/`approved`, có `self_approved` |
| `pending_approval` | Duyệt | duyệt | `approved`, thành bản đang dùng | `approved`, có `self_approved` |
| `pending_approval` | Từ chối (bắt buộc lý do) | duyệt | `rejected` | `rejected` |
| bản đang dùng | Rebase-apply | duyệt | version mới `approved` | `rebase_applied`, `self_approved=true` |
| bất kỳ | Có version mới hơn | — | giữ nguyên, không thao tác được (409) | — |

- Sửa nội dung sau khi gửi duyệt hoặc bị từ chối: tạo version mới `draft` (như hiện nay). Bản cũ không còn là bản mới nhất.
- `self_approved` là true khi người duyệt cũng là người soạn hoặc người gửi.

## BE
- **Migration `0156`** (chỉ thêm cột, chạy online): `submitted_by`, `submitted_at`, `rejected_by`, `rejected_at`, `decision_note` trên `company_template_workflow_override_versions`. `state` là `VARCHAR(32)`, đủ chỗ cho `pending_approval`.
- **`GetCompanyWorkflowOverride`:** `draft_version` là version mới nhất khi nó chưa `approved` (`draft`, `pending_approval` hoặc `rejected`). Trường `state` cho biết trạng thái.
- **Approve (repo MySQL và in-memory):**
  - nhận `draft` hoặc `pending_approval`;
  - version phải là bản mới nhất, nếu không trả 409 `STALE_ETAG`;
  - bỏ chặn tự duyệt; trả `self_approved`.
- **PUT `publish:true`:** kiểm quyền duyệt và validate kích hoạt **trước** khi lưu.
- **Rebase-apply:** cần quyền duyệt (403 trước mọi ghi); thêm audit.
- **Endpoint mới:**
  - submit và reject;
  - danh sách chờ duyệt của công ty.
- **Step-groups PUT** (sửa tại chỗ bản nháp mới nhất): chỉ khi bản đó là `draft`, nếu không trả 409.
- **Conflict resolve:**
  - `resolution` phải nằm trong `resolution_options` của conflict;
  - `merge_manual` phải có `resolution_value` hợp lệ cho field (bảng ở `02-contract.md`);
  - rebase-apply kiểm lại các điều kiện này.
- **In-memory:** cùng quy tắc (state, bản mới nhất, quyền), để test service chạy được trên cả hai.

## FE
- **Hộp thoại sửa workflow:**
  - người duyệt: "Lưu nháp" và "Kích hoạt…" (như cũ);
  - người chỉ có quyền ghi: "Lưu nháp" và "Gửi duyệt" (lưu rồi gửi, có hộp xác nhận).
- **Trang chi tiết loại thông tin:**
  - nhãn trạng thái bản nháp: "Bản nháp vN", "Chờ duyệt vN (gửi bởi …, lúc …)", "Bị từ chối vN: lý do";
  - người duyệt có "Duyệt" (xác nhận; ghi chú "Bạn đang tự duyệt bản do mình soạn" khi đúng như vậy) và "Từ chối" (bắt buộc lý do).
- **Danh sách chờ duyệt:** chỉ người duyệt thấy. Nếu không có mục nào thì ẩn.
- **Rebase:** "Áp dụng rebase" chỉ hiện với người duyệt; người chỉ có quyền ghi thấy ghi chú "Cần người có quyền duyệt".
- **Lỗi 403** của các thao tác mới hiện ngay tại chỗ (`suppressForbiddenNavigation`), không chuyển sang `/app/forbidden`.

## Kịch bản lỗi
| Kịch bản | Kết quả |
|---|---|
| Người chỉ có quyền ghi gửi `publish:true` (FE cũ) | 403, **không** lưu bản nháp |
| Hai người cùng sửa | ETag và version mới nhất (409), như hiện nay |
| Duyệt bản cũ sau rebase-apply | 409 `STALE_ETAG` (WF-08c) |
| Duyệt hai lần, hoặc duyệt rồi từ chối cùng lúc | Transaction khoá header override (`FOR UPDATE`); lần sau trả 409 trạng thái |
| `merge_manual` với `{}` | 422 tại lúc resolve, kiểm lại lúc apply |
| Gửi duyệt bản không hợp lệ để kích hoạt (thiếu mô tả bước…) | 422, giống lúc kích hoạt, để người duyệt không nhận bản không kích hoạt được |
