# 06: Review song song (Phase 6) và premerge (GATE B)

Ba reviewer chạy song song, chỉ đọc: `admin-role-reviewer`, `db-tx-reviewer`, `api-compat-reviewer`. Không có phát hiện Critical hay High.

## Phát hiện và xử lý
| ID | Mức | Nội dung | Xử lý |
|---|---|---|---|
| API-01 | Medium | Khôi phục bỏ (`SKIPPED`) cả kỳ chưa đến ngày mở; seeder không tạo lại được, nên lưu trữ nhầm rồi khôi phục là mất kỳ sắp tới | Đã sửa: kỳ chưa đến ngày mở tự về `PENDING`; "Tạo bù" chỉ áp cho kỳ đã lỡ. Test `RestoreResumesCyclesThatHaveNotOpened` |
| API-02 / DB-01 | Medium | Kỳ `PENDING/RETRY` của template đã lưu trữ thoát việc treo (worker cũ, reaper, seeder chạy chồng) rồi bị tạo hồ sơ khi khôi phục | Đã sửa: khôi phục gom chúng về treo trước, dưới khoá template; archive-impact liệt kê chúng. Template lưu trữ trước bản này đã có 0154. Test `RestoreResolvesCyclesThatEscapedTheArchive` |
| API-03 | Medium | Rollback BE để lại kỳ `SUSPENDED` kẹt | Đã ghi runbook SQL trước khi rollback vào `02-contract.md` (bảng tương thích) |
| ROLE-01 / API-05 / DB-05 | Medium/Low | Danh sách kỳ treo bị cắt mà không báo; giới hạn danh sách (500/5000) lệch giới hạn id khi khôi phục (1000) | Đã sửa: danh sách kỳ đã lỡ tối đa 1000 (= `MaxBackfillCycleIDs`), thêm `suspended_total`; FE chặn khôi phục và báo khi danh sách bị cắt |
| ROLE-02 | Low | Audit khôi phục thiếu id kỳ được chọn | Đã sửa: audit ghi `backfill_cycle_ids`, `resumed_cycles` |
| ROLE-03 | Low | Thiếu test 404 / 403 / 422 | Đã thêm: impact template doanh nghiệp → 404 (MySQL); có `cms.template.archive` hoặc `disclosure_type.publish` mà thiếu `platform.cms.view` → 403 cho impact và khôi phục, repo không được gọi; id của template khác → 422 (MySQL) |
| ROLE-04 | Low | Worker treo kỳ dù template vừa được khôi phục | Đã sửa: câu treo chỉ chạy khi template vẫn `archived`, nếu không thì kỳ chuyển `RETRY` |
| DB-02 | Low | Adapter tra workflow trước `CreateRecord`, nên template đã lưu trữ thường trả 404 và kỳ thành `FAILED`, không treo | Đã sửa: worker thử treo cả khi nhận 404; câu treo tự kiểm tra template. 404 với template đang hoạt động vẫn `FAILED` |
| DB-03 | Low | Câu treo (UPDATE JOIN) có thể deadlock với khôi phục | Đã sửa: treo lỗi thì log cảnh báo và chuyển `RETRY`, không dừng tick |
| DB-06 | Low | Kỳ đã có hồ sơ từ lần thử trước bị `SKIPPED`, để lại hồ sơ Draft mồ côi | Đã sửa: kỳ có `attempt_record_id` trỏ tới hồ sơ còn tồn tại tự chạy lại (worker hoàn tất hồ sơ, id cố định) |
| API-04 | Low | Ghi chú lỗi hộp thoại "Khôi phục" khẳng định "không kỳ nào được tạo bù", sai với BE cũ | Đã sửa: câu trung tính, không khẳng định kết quả |
| API-06 | Low | Trang chi tiết: `GET disclosure-types/{id}` trả 404 với template đã lưu trữ, làm mất tên phòng ban và workflow đã tải | Đã sửa một phần: lỗi 404 không còn xoá dữ liệu đã tải (`DeadlineDetail.tsx`). Nhãn ở trang chi tiết khi hồ sơ không có trong 100 cảnh báo đầu: follow-up (cần thêm `template_archived` vào response chi tiết hồ sơ) |
| DB-04 | Low | Archive-impact quét toàn bảng `disclosure_records` (không có index `type_id`) | Chấp nhận trong MR-3 (chỉ CMS, đọc không khoá, DEV 360 dòng). Follow-up: index `(type_id, status)` |
| API-07 | Info | Postman/OpenAPI chưa có các endpoint này | Có từ trước MR-3. Follow-up `api-postman-sync` |

Không làm: seeder kiểm tra trạng thái template khi chèn kỳ (DB-01, tuỳ chọn), vì bước gom khi khôi phục đã xử lý kết quả; lưu trữ lại không chạy lại câu treo (khôi phục đã gom).

## Kiểm ngược sau khi sửa
Tắt từng hành vi, test fail, khôi phục file:
- M7: khôi phục không gom kỳ sót lại;
- M8: khôi phục bỏ cả kỳ chưa đến ngày mở;
- M9: khôi phục bỏ kỳ đã có hồ sơ;
- M10: worker không treo khi nhận 404;
- tắt `requireCMSRouteAccess` trong `requireCMSTemplateArchive` (test 403);
- bỏ điều kiện template `archived` trong `MarkPeriodicCycleSuspended`.

## GATE B (premerge)
| Mục | Kết luận |
|---|---|
| Đúng nghiệp vụ | Q1-B: hồ sơ đang mở vẫn hiện và được nhắc; không tạo hàng loạt hồ sơ cho kỳ đã lỡ; kỳ sắp tới không bị mất |
| Ranh giới vai trò | Endpoint mới dưới `/api/v1/platform/cms`, cần `platform.cms.view` và quyền archive; template doanh nghiệp 404. Portal vẫn theo `company_id` trong token |
| Contract | Chỉ thêm trường và endpoint; bảng tương thích có cả rollback |
| Dữ liệu và đồng thời | Lưu trữ/khôi phục khoá template trước rồi mới tới kỳ; khôi phục tất cả hoặc không gì; worker không ghi đè kỳ của template vừa khôi phục |
| Hiệu năng | Archive-impact quét toàn bảng (DB-04, chấp nhận, follow-up index) |
| Quan sát | Audit và log có số kỳ treo, tạo bù, tự chạy lại, bỏ qua |
| Test | MySQL integration 13/13, unit mới, kiểm ngược; `go test ./...` và vitest không có fail mới |
| Kết quả | **Đạt**, không còn phát hiện Critical/High mở |
