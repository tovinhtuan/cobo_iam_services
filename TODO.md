# TODO — lệch còn lại so với `docs/implementation-step-by-step.md`

Đã có: IAM/authz/audit/admin MySQL khi `MYSQL_DSN`, outbox retry + jitter + `failed_permanent` sau 10 lần, migration **0006** cho admin rule tables.

## Ưu tiên cao

- **Idempotency**: đã wire cho **disclosure** `POST .../submit` và `POST .../confirm` khi có `MYSQL_DSN` (header `Idempotency-Key`, scope `disclosure.submit` / `disclosure.confirm`). Mở rộng: workflow, notification, admin mutations.
- **login_attempts**: đã ghi từ **IAM Login** khi có `MYSQL_DSN` (success + failure + mã lỗi). Tiếp: rate limit / dashboard từ bảng này.
- **Projection writer**: job hoặc consumer outbox cập nhật `membership_effective_*` thay vì chỉ seed/SQL thủ công.
- **Outbox**: transactional publish cho disclosure/workflow khi cần consistency; xử lý event stuck `processing` (timeout reclaim).

## P1 độ sâu

- **Disclosure**: `disclosure_histories` + state machine + audit theo Sec 16.
- **Workflow**: definition tables / optimistic locking như matrix dài hạn.
- **Notification**: `ResolveRecipients` theo role/department/title/workflow (thay stub membership hiện tại).

## QA / vận hành

- Integration tests với MySQL (docker hoặc `INTEGRATION_MYSQL_DSN`).
- Test worker: handler fail → retry → `failed_permanent`.
- Runbook: thứ tự migration 0001 → 0003 → 0004 → 0005 → 0006 + seed dev.

## Tài liệu

- Đồng bộ Sec 14 entity matrix với tên bảng thực tế trong `migrations/*.sql` nếu còn lệch.
