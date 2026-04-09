# Rà soát logic vs implementation-step-by-step (tóm tắt)

Ngày tham chiếu: plan `docs/implementation-step-by-step.md` + code tree hiện tại.

## Đã khớp plan (ở mức bootstrap / tối thiểu)

- P0.1–P0.3, P0.5–P0.6 (kể cả `/me/capabilities`), P2.2 Redis, P2.3 hooks, README, outbox MySQL + worker poll + backoff cơ bản.
- P1 HTTP + authorize + MySQL repos disclosure/workflow/notification (0004), transactional notification enqueue + outbox.
- Integration smoke `internal/httpserver` (không DB).

## Khoảng trống chính (theo mức độ rủi ro / plan)

### A. IAM & session (P0.4)

- **Refresh token rotation**: **đã bổ sung** — `Refresh` phát `refresh_token` mới + `RotateRefreshToken`; token cũ vô hiệu. Client bắt buộc lưu token mới (xem `docs/api-contracts-json.md`).

### B. Dữ liệu runtime vs schema 0001 (P0.2 / Sec 14)

- **IAM** (`users`, `credentials`, `sessions`, …): schema có, **runtime vẫn in-memory** (static user + session map).
- **Authorization** (`permissions`, `role_permissions`, …): **chưa đọc từ MySQL**; resolver/checker dùng fixture in-memory.
- **Audit**: **in-memory**; `AppendAuditLog` có field snapshot nhưng admin/iam hook **chưa gửi** `effective_*_snapshot` đầy đủ theo Sec 16.
- **Idempotency**: bảng `idempotency_keys` có, **chưa gắn** vào confirm/submit/approve.

### C. Projection P2.1

- Migration **0003** tạo bảng projection (`membership_effective_*`) nhưng **không có** code ghi/đọc hay **recompute từ outbox** (plan: “Recompute tu outbox events”).
- Cache hiện tại là **snapshot resolver in-memory/Redis**, không đồng bộ với bảng 0003.

### D. Outbox & worker (P0.7 / Sec 15)

- **Transactional outbox toàn diện**: mới **notification enqueue**; disclosure/workflow/admin chưa gắn tx chung.
- **Jitter** retry, **dead-letter** `failed_permanent`, **idempotent consumer** có kiểm thử — chưa.
- Worker chỉ đăng ký `notification.dispatch`; **không** có consumer cập nhật projection.

### E. P1 nghiệp vụ (độ sâu so với mô tả plan)

- **Disclosure**: chưa có `disclosure_histories`, state machine/audit theo Sec 16 đầy đủ.
- **Workflow**: chưa có **definition** / **optimistic locking** / bảng `tasks` như matrix Sec 14 (đang dùng `workflow_tasks` đơn giản trong 0004).
- **Notification**: `ResolveRecipients` **stub** (trả về membership hiện tại); chưa role/department/title/workflow như plan P1.3.

### F. Admin P1.4

- Route skeleton + audit hook cơ bản có; **persist MySQL** cho membership/role/rule **vẫn in-memory** (`cainmem.AdminRepository`).

### G. Test & deliverables (Sec 6–7, 18–19)

- Thiếu: integration **với MySQL** (auth + tenant), **migration smoke** tự động, suite authorize regression đủ dòng Sec 18.1, worker poison/dead-letter.
- **TODO.md** (Sec 19 deliverables) **chưa có**.

### H. Lệch tài liệu nội bộ

- Sec 14 ghi nhãn migration `0002_business_modules` / `0003_projection_opt`; repo thực tế: **0003** projection + **0004** P1 tables — nên đồng bộ lại doc để tránh nhầm thứ tự.

## Gợi ý thứ tự xử lý

1. Refresh rotation + (tuỳ ưu tiên) IAM session trên MySQL.
2. Đồng bộ projection: hoặc wire đọc 0003 hoặc đơn giản hoá doc nếu chỉ dùng cache resolver.
3. Outbox: jitter + dead-letter + test idempotent.
4. P1 sâu: admin MySQL, resolver MySQL, recipient thật cho notification.
