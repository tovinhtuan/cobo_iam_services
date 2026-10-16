# 99: Tổng kết MR-3 (16/10/2026)

## Kết quả
Theo quyết định PO Q1-B, hồ sơ đang mở của template đã lưu trữ vẫn là nghĩa vụ:
- vẫn hiện ở tab Cảnh báo, dashboard Portal, dashboard CMS và "Việc của tôi", kèm nhãn "Mẫu đã ngừng áp dụng";
- vẫn được nhắc hạn.

Lưu trữ thì treo các kỳ chưa có hồ sơ. Khi khôi phục:
- kỳ chưa đến ngày mở, hoặc đã có hồ sơ từ lần thử trước, tự chạy lại;
- kỳ đã lỡ chỉ được tạo bù nếu Admin Web chọn; kỳ không chọn bị bỏ qua.

## Gate
| Gate | Kết quả |
|---|---|
| A (contract) | User duyệt trong hội thoại ngày 16/10: MR-3 gọn (tách MR-3b); để 312 hồ sơ QA trên DEV hiện lại kèm nhãn. Bổ sung sau review ghi ở `02-contract.md` |
| B (premerge) | Đạt (`06-review.md`): không còn Critical/High; 3 Medium và 9 Low đã sửa; DB-04 và API-07 chấp nhận, có follow-up |

## Rollout
- Thứ tự: migration `0154` (chỉ đổi dữ liệu, chạy lại an toàn), BE (api và worker), rồi FE.
- Không có cờ tính năng, không đổi schema.
- Rollback BE: chạy runbook SQL trong `02-contract.md` trước.
- Trước khi deploy prod: chạy 3 truy vấn đếm trong `05-verification.md`.

## Lưu ý khi commit
`migrations/run_dev_migrations.sh` đang có thêm thay đổi của task khác (0099, 0150, 0155, preflight 0099, ghi chú 0038/0098), cùng các file `0155_*` chưa track. MR-3 chỉ thêm dòng `0154`. Cần tách khi commit.

## Follow-up
- MR-3b, sửa theo v1.2:
  - ad-hoc ở tab Cảnh báo (AC-11);
  - "Việc của tôi" hiện hồ sơ Draft (AC-13);
  - KPI trạng thái mở không lọc theo khoảng ngày;
  - nhắc hạn dùng `ObligationOpen`.
- Nhãn ở trang chi tiết hồ sơ khi hồ sơ không nằm trong 100 cảnh báo đầu (API-06): thêm `template_archived` vào response chi tiết hồ sơ.
- Index `disclosure_records (type_id, status)` (DB-04).
- Postman: archive, restore (`backfill_cycle_ids`), archive-impact (API-07).
- Dọn 312 hồ sơ QA trên DEV bằng "đóng nghĩa vụ" (MR-8).
- Có thể thêm: seeder kiểm tra trạng thái template khi chèn kỳ (DB-01, tuỳ chọn).
