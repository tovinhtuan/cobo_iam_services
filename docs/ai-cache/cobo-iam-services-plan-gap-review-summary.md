# Rà soát code vs plan (`implementation-step-by-step.md`) — cập nhật

Tham chiếu: `docs/implementation-step-by-step.md` + wire `internal/httpserver/server.go`.

## Đã khớp / đã bù so với plan (bootstrap → runtime DB)

- **P0.1–P0.3**, **P0.4** (login, refresh **rotation**, select/switch, logout), **P0.5–P0.6** (authorize, me, effective access, capabilities theo tiến độ đã implement).
- **P0.8** (khi `MYSQL_DSN`): IAM **sessions + credentials** MySQL; **membership query** MySQL; **authorization repository** MySQL (permissions/assignments/departments + `membership_effective_responsibilities`); migration **0005** + seed dev.
- **Audit**: khi có DSN ghi **`audit_logs`** (`internal/audit/infra/mysql`); không DSN vẫn in-memory.
- **P1 HTTP + MySQL**: disclosure / workflow / notification repos 0004; **notification enqueue** transactional với outbox MySQL.
- **P2.2** Redis cache projection; **P2.3** SSO/MFA hooks (interfaces).
- **Outbox MySQL** + worker poll + **backoff** + **jitter** + **`failed_permanent`** sau 10 retry (`MarkFailedPermanent`).
- **Admin P1.4 MySQL** khi có DSN: `companyaccess/infra/mysql` + migration **0006** (workflow/notification rule tables); `ListRoles` theo company (global + company-scoped **role_id**).
- **`TODO.md`** root: backlog còn lại (idempotency, projection writer, P1 sâu, …).
- Integration smoke `internal/httpserver` (không DB).

## Khoảng trống còn lại (ưu tiên theo rủi ro / plan)

### 1. Admin P1.4 — đã có MySQL khi DSN

- (Hoàn thành phần persist chính.) Lưu ý: `GET /admin/permissions` trả **permission_id** (UUID) khi MySQL; in-memory dev vẫn trả mã ngắn — client cần khớp môi trường.

### 2. Authorization — checker vs DB

- **Resolver** đọc DB (hoặc fixture khi không DSN); **checker** vẫn `authinmem.NewChecker()` — map action → permission **cố định trong code**. Plan muốn consistency end-to-end: cần rà soát drift checker vs `permissions`/`role_permissions` trong DB (hoặc generate checker từ DB sau này).

### 3. Audit đầy đủ (Sec 16)

- Cột `effective_permissions_snapshot` / `effective_scope_snapshot` có trong schema; **IAM/admin hooks hiện ít khi gửi** (metadata có, snapshot thường trống).
- **login_attempts** (0001): schema có thể có — **chưa** ghi rate/audit login attempt từ app.

### 4. Idempotency (0001)

- Bảng `idempotency_keys` — **chưa** gắn confirm/submit/approve/disclosure theo plan.

### 5. Projection P2.1 / 0003

- Resolver **đọc** `membership_effective_responsibilities` khi dùng MySQL repo; **không có** pipeline **ghi/recompute** từ outbox events (worker không cập nhật projection). Dữ liệu projection phụ thuộc migration/seed hoặc công cụ ngoài.

### 6. Outbox & worker (P0.7 / Sec 15)

- Transactional outbox **toàn diện**: mới **notification enqueue**; disclosure/workflow **chưa** cùng transaction với outbox.
- **Jitter** + **failed_permanent** (10 lần) + unit test cơ bản **đã có**; còn thiếu: reclaim event kẹt `processing`, idempotent consumer nâng cao, side-effect thật (email).
- Worker chỉ xử lý `notification.dispatch` (log); **không** consumer cập nhật projection.

### 7. P1 độ sâu nghiệp vụ

- **Disclosure**: chưa dùng `disclosure_histories` / state machine + audit theo matrix đầy đủ.
- **Workflow**: schema 0004 đơn giản (`workflow_tasks`…), **chưa** đủ definition/optimistic locking như Sec 14 mô tả dài hạn.
- **Notification**: `ResolveRecipients` **stub** (trả membership hiện tại); chưa role/department/title/workflow như P1.3.

### 8. Test & deliverables (Sec 6–7, 19)

- Thiếu: integration **có MySQL** (auth + tenant), migration smoke tự động trong CI, suite regression authorize theo Sec 18.
- **`TODO.md`**: đã thêm; cần thực thi các mục trong file.

### 9. Tài liệu

- Sec 14 entity matrix vẫn có thể lệch tên bảng thực tế (0004 dùng tên cụ thể khác một phần so “target tree”) — khi chỉnh plan nên đối chiếu `migrations/*.sql`.

## Gợi ý thứ tự xử lý tiếp

1. **Checker** đồng bộ với model permission trong DB hoặc test matrix chống drift.
2. **Idempotency** cho API nhạy cảm; **login_attempts** nếu cần security baseline.
3. Outbox: transactional cho disclosure/workflow khi cần; reclaim `processing`; consumer thật.
4. Projection writer / consumer hoặc đơn giản hóa plan nếu chỉ maintain SQL batch.
5. P1 sâu: disclosure history, notification recipients, workflow definition.
