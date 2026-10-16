# 01: Thiết kế, MR-3

## Nguyên tắc
- Trạng thái template chỉ ảnh hưởng tới danh mục, việc sinh kỳ và việc tạo hồ sơ mới. Nó không quyết định một hồ sơ có còn là nghĩa vụ hay không (OQ-006, Q1-B).
- "Đã lưu trữ" là `dt.status = 'archived'` (template chung) hoặc `review_status = 'archived'` (template doanh nghiệp). Không suy ra từ `active_version_no = 0`, vì giá trị này còn có nghĩa "chưa từng kích hoạt" hoặc "khôi phục về nháp".

## 1. `ObligationOpen` dùng chung (BE)
- Một định nghĩa duy nhất trong `disclosure/app` (`obligation_open.go`: SQL và bản Go), có alias ở `deadlinealerts/app`, thay cho `listRowsV1ObligationMembershipSQL`, `ListRowsActiveTemplateSQLJoin` và `dashboardOpenObligationSQL` (platformcms).
- Phạm vi MR-3: chỉ bỏ điều kiện về template. Điều kiện về hồ sơ giữ nguyên như hiện tại: `draft AND submitted_at IS NULL`, cộng cổng `OpenAt` của periodic. Phần ad-hoc và "Việc của tôi" để MR-3b.
- JOIN template đổi thành:

  ```sql
  LEFT JOIN disclosure_types dt
  LEFT JOIN disclosure_type_versions dtv ON dtv.version_no = COALESCE(NULLIF(dt.active_version_no, 0), dt.archived_from_version_no)
  ```

  Như vậy hồ sơ của template đã lưu trữ vẫn có tên, `template_category` và cấu hình hạn.
- `GetActiveVersionDeadlineConfig` dùng cùng cách chọn version (`active`, nếu không có thì `archived_from`). Nếu không làm vậy, hồ sơ không có `planned_date` sẽ mất hạn và bị loại khỏi danh sách.
- Thêm `template_archived` (bool) vào `AlertRow` và `DeadlineAlertDTO`. Nhờ đó dashboard Portal và nguồn dự phòng của FE cũng nhận được cờ này.
- "Kỳ sắp tới" (`ListNextAlertCycles`) giữ điều kiện template có version đang hoạt động (chỉ template như vậy mới sinh hồ sơ, nên template đã lưu trữ tự bị loại), và loại thêm kỳ `SUSPENDED`/`SKIPPED`.
- **"Việc của tôi":**
  - thêm `template_archived` vào `MyTaskItem`, đọc từ `disclosure_types`;
  - không đổi bộ lọc của màn này trong MR-3.
- **Dashboard Portal:** thêm `template_archived` vào `ImmediateActionItem`, lấy từ `DeadlineAlertDTO`.
- **Nhắc hạn:** không đổi. Hiện nay nhắc hạn đã không lọc theo template, nên vẫn nhắc đúng như Q1-B.

## 2. Kỳ khi lưu trữ hoặc khôi phục (BE)
- Thêm trạng thái `materialization_state`:
  - `SUSPENDED`: 9 ký tự, vừa cột `VARCHAR(16)`, nên không cần migration; kèm `last_error_code = 'TEMPLATE_ARCHIVED'` để ghi lý do;
  - `SKIPPED`.

  Không dùng tên `SUSPENDED_BY_ARCHIVE` (20 ký tự), vì sẽ phải nới cột.
- **Lưu trữ** (`ArchiveGlobalTemplate`, cùng transaction và dưới khoá `FOR UPDATE` của template):

  ```sql
  UPDATE periodic_cycles SET materialization_state='SUSPENDED', last_error_code='TEMPLATE_ARCHIVED'
  WHERE type_id=? AND record_id IS NULL AND materialization_state IN ('PENDING','RETRY')
  ```

  - Response trả thêm `suspended_cycles`.
  - Kỳ `CLAIMED` không bị đụng tới.
- **Kỳ đã claim mà gặp lưu trữ:** worker nhận 409 `TEMPLATE_ARCHIVED`, hoặc 404 từ bước tra workflow, thì chuyển kỳ sang `SUSPENDED` thay vì `RETRY`/`FAILED`, nếu template vẫn đang lưu trữ.
  - Hồ sơ không thể tạo từ template đã lưu trữ, nên kỳ này được đối xử như kỳ chưa claim.
  - Đây là chỗ lệch so với câu "hoàn tất bình thường" trong `01-design.md` của bộ quyết định PO; lý do đã ghi ngay trên.
- **Khôi phục** (`RestoreGlobalTemplate`, cùng transaction): body nhận thêm `backfill_cycle_ids []string`, tuỳ chọn.
  - Trước tiên gom kỳ `PENDING/RETRY` sót lại (worker cũ, reaper, seeder chạy chồng) về `SUSPENDED`.
  - Kỳ `SUSPENDED` có trong danh sách: chuyển về `PENDING`, xoá `materialized_at`/`last_error`, để worker tạo hồ sơ.
  - Kỳ `SUSPENDED` chưa đến ngày mở, hoặc đã có hồ sơ từ lần thử trước: tự về `PENDING` (không có gì bị lỡ).
  - Các kỳ đã lỡ còn lại: chuyển `SKIPPED`.
  - Mặc định (FE cũ, không gửi trường này): bỏ qua mọi kỳ đã lỡ.
  - Response trả thêm `backfilled_cycles`, `resumed_cycles` và `skipped_cycles`.
  - Kỳ của các slot mới vẫn do worker sinh như cũ, sau khi template có version hoạt động.
- **Tác động** (endpoint mới, chỉ đọc): `GET /api/v1/platform/cms/templates/{type_id}/archive-impact` trả về:
  - hồ sơ đang mở theo công ty (cùng predicate ObligationOpen, không lọc template);
  - các kỳ sẽ bị treo (`PENDING/RETRY` chưa có hồ sơ);
  - các kỳ đã lỡ mà khôi phục có thể tạo bù (tối đa 1000, kèm `suspended_total`), và số kỳ sẽ tự chạy lại (`auto_resume`).

  Quyền giống archive.
- **Không đổi trong MR-3:**
  - lưu trữ template doanh nghiệp (Q6, MR-7);
  - công ty `suspended` (H13, MR-5).

## 3. FE
- **Tab Cảnh báo:** thẻ có chip "Mẫu đã ngừng áp dụng" (tone neutral, kèm biểu tượng, không chỉ dùng màu). Header trang chi tiết dùng chip giống vậy, lấy cờ từ alert đã khớp.
- **"Việc của tôi"** (`MyTasksPanel`) và **dashboard** (`ActionItemCard`): dùng cùng chip.
- **CMS Templates:**
  - hộp thoại "Ngừng cung cấp" gọi archive-impact, hiện số hồ sơ đang mở theo công ty và số kỳ sẽ bị treo;
  - hộp thoại "Khôi phục" liệt kê kỳ đã lỡ, mỗi kỳ có checkbox "Tạo bù", mặc định bỏ chọn; ghi số kỳ sẽ tự chạy lại; chặn khôi phục khi danh sách bị cắt;
  - `CmsConfirmDialog` thêm một slot `children`.
  - Lỗi khi tải tác động không chặn thao tác; hộp thoại ghi "Chưa tải được số liệu" và không khẳng định kết quả (BE cũ làm sống lại mọi kỳ).

## 4. Kịch bản lỗi
| Kịch bản | Kết quả |
|---|---|
| Lưu trữ trong lúc worker vừa liệt kê kỳ nhưng chưa claim | Claim cần `PENDING/RETRY`, mà kỳ đã là `SUSPENDED`, nên claim thua và không có hồ sơ |
| Worker đã claim, sau đó template bị lưu trữ | Tra workflow trả 404 (hoặc `CreateRecord` trả 409), template vẫn lưu trữ nên kỳ chuyển `SUSPENDED` |
| Worker nhận 409 nhưng template vừa được khôi phục | Câu treo không đổi gì, kỳ chuyển `RETRY` |
| Câu treo bị deadlock với khôi phục | Kỳ chuyển `RETRY`; nếu template vẫn lưu trữ, lần khôi phục sau gom nó về treo |
| Khôi phục sau nhiều tuần | Không tạo hồ sơ cho kỳ đã lỡ; chỉ kỳ được chọn mới chuyển `PENDING`. Kỳ chưa đến ngày mở tự chạy lại |
| Lưu trữ nhầm rồi khôi phục ngay | Mọi kỳ chưa đến ngày mở tự chạy lại; không mất kỳ sắp tới |
| `backfill_cycle_ids` có id không thuộc template hoặc không ở trạng thái `SUSPENDED` | 422 `INVALID_REQUEST` (`details.field = backfill_cycle_ids`), không đổi gì |
| Lưu trữ hai lần | Lần hai là no-op, như hiện nay; `suspended_cycles = 0` |
| FE cũ gọi khôi phục | Mọi kỳ treo chuyển `SKIPPED`, đúng tinh thần "không tạo hàng loạt khi chưa chọn" |
| BE cũ với FE mới | Không có cờ, không có endpoint tác động; FE hiện "Chưa tải được số liệu" và khôi phục như cũ |

## 5. Quan sát
- Log và audit:
  - `cms_template.archive` thêm `suspended_cycles`;
  - `cms_template.restore` thêm `backfilled_cycles`, `resumed_cycles`, `skipped_cycles` và `backfill_cycle_ids`.
- Collector `observe/periodic_collector.go` đếm theo state, nên tự có thêm `SUSPENDED` và `SKIPPED`.
