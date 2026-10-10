# Bug: thu hồi quyền không có hiệu lực ngay (PR-A) — 2026-10-10

Nguồn: `../risk-review-2026-10-10/10-risk-report.md`, kế hoạch: `../risk-review-2026-10-10/20-fix-plan.md`.

## Triệu chứng (từ code, chưa có báo cáo runtime)

| Finding | Kỳ vọng | Thực tế |
|---|---|---|
| H3 | Membership inactive, suspended hoặc đã xoá thì mất quyền ngay | Resolver vẫn trả đủ quyền và data scope của role. Refresh vẫn cấp token mới. |
| H17 / ROLE-06 | Gỡ role, quyền hoặc phòng ban của một membership thì có hiệu lực ngay | Cache effective access (Redis/in-memory, TTL 5 phút) chỉ bị xoá ở rollback, apply approval và 3 route company-admin |
| CACHE-11 | Sau khi invalidate, không còn phục vụ quyền cũ | Request đọc DB trước commit `SET` lại giá trị cũ sau lệnh `DEL` |
| CACHE-12 / PERF-21 | Invalidate luôn chạy sau commit | Bỏ qua im lặng khi list member lỗi, Redis lỗi hoặc client ngắt kết nối. Chi phí 3N+1 query + N lệnh `DEL`. |
| BES-10 | Restore không cấp lại direct grant cho membership inactive | Có cấp lại |

- **Môi trường:** code tại `ff5ada2`.
- **Role bị ảnh hưởng:** mọi tenant.
- **Tần suất:** mỗi lần admin khoá hoặc hạ quyền một tài khoản.

## Lịch sử
- Risk review lần 1 (H3, H17), lần 2 (ROLE-06), lần 3 (CACHE-11/12, BES-10).
- `ff5ada2` đã thêm invalidate ở 3 route company-admin (ROLE-22). Không có bản sửa nào khác.
