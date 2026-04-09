# cobo_iam_services

Dịch vụ IAM / phân quyền / company context cho nền tảng Cobo (Go). Hai process: **API** (`cmd/api`) và **Worker** (`cmd/worker`).

## Kiến trúc tóm tắt

- **Transport HTTP**: `/api/v1` (client), `/internal/v1` (authorize nội bộ).
- **IAM**: login, refresh, logout, select/switch company, JWT/opaque token. Khi có **`MYSQL_DSN`**: credential + session (refresh hash SHA-256) lưu MySQL; không DSN thì in-memory (dev).
- **Company context**: membership/company/role khi có **`MYSQL_DSN`** đọc MySQL; không DSN thì fixture in-memory.
- **Authorization**: resolver + checker + effective access. Khi có **`MYSQL_DSN`**: permissions/assignments/departments join runtime + responsibilities từ `membership_effective_responsibilities`; cache Redis/TTL tùy cấu hình. Không DSN: fixture in-memory.
- **Audit + Outbox**: khi có **`MYSQL_DSN`** ghi **`audit_logs`** (MySQL); không DSN thì audit in-memory. Outbox **MySQL** khi có DSN, không thì in-memory (chỉ dev).
- **Module nghiệp vụ**: disclosure, workflow, notification, company access admin — chủ yếu skeleton + in-memory.

## Ranh giới module (gói chính)

| Khu vực | Đường dẫn |
|--------|-----------|
| IAM | `internal/iam/` |
| Company access | `internal/companyaccess/` |
| Authorization | `internal/authorization/` |
| Audit | `internal/audit/` |
| Outbox | `internal/platform/outbox/` (+ `mysql/`, `inmemory/`) |
| Nền tảng | `internal/platform/` (config, db, errors, httpx, redis, …) |

## Yêu cầu

- Go **1.22+**
- **MySQL 8** (cho outbox `SKIP LOCKED` và schema migration); tùy chọn nếu chỉ chạy API không `/readyz` và outbox in-memory.

## MySQL cục bộ

Ví dụ tạo DB và user (điều chỉnh mật khẩu):

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS cobo_iam CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

DSN khuyến nghị (có `parseTime` và UTC):

```text
user:pass@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false
```

## Migration

Chưa tích hợp CLI migrate trong repo; áp file SQL theo thứ tự:

1. `migrations/0001_init_core.up.sql`
2. `migrations/0003_effective_access_projection.up.sql`
3. `migrations/0004_p1_business_tables.up.sql` (disclosure / workflow / notification; FK tới `companies` — cần có company hợp lệ trước khi seed/ghi nghiệp vụ)
4. `migrations/0005_sessions_refresh_hash_unique.up.sql` (unique `sessions.refresh_token_hash` — lookup refresh an toàn)

Ví dụ:

```bash
mysql -u user -p cobo_iam < migrations/0001_init_core.up.sql
mysql -u user -p cobo_iam < migrations/0003_effective_access_projection.up.sql
mysql -u user -p cobo_iam < migrations/0004_p1_business_tables.up.sql
mysql -u user -p cobo_iam < migrations/0005_sessions_refresh_hash_unique.up.sql
```

Rollback: chạy tương ứng file `*.down.sql` (theo thứ tự ngược).

### Seed dev (IAM + authorization khớp fixture in-memory cũ)

Sau khi chạy các migration trên, có thể nạp dữ liệu dev (`user@example.com` / `single@example.com`, mật khẩu `secret`, bcrypt cố định trong file):

```bash
mysql -u user -p cobo_iam < migrations/seed_dev_identity_authorization.sql
```

File seed dùng `ON DUPLICATE KEY UPDATE`; nên chạy sau **0001**, **0003**, khuyến nghị sau **0004** (nếu cần FK đầy đủ) và **0005**. Chi tiết xem comment đầu file seed.

## Biến môi trường

Xem `configs/config.example.env`. Các biến thường dùng:

| Biến | Ý nghĩa |
|------|---------|
| `MYSQL_DSN` | Kết nối MySQL: outbox durable, IAM session/credential, membership query, authorization từ DB, **audit_logs**, disclosure/workflow/notification MySQL; `/readyz` ready khi ping OK |
| `HTTP_ADDR` | API bind, mặc định `:8080` |
| `REDIS_ADDR` | Tùy chọn — cache effective-access projection |
| `EFFECTIVE_ACCESS_CACHE_TTL` | TTL cache projection |
| `WORKER_TICK_INTERVAL` | Chu kỳ poll outbox (worker) |
| `LOG_LEVEL` | `debug`, `info`, … |

## Chạy API

```bash
export MYSQL_DSN='...'   # tùy chọn
go run ./cmd/api
```

- `GET /healthz` — luôn OK nếu process sống.
- `GET /readyz` — ready khi có MySQL và ping thành công.

## Chạy Worker

Worker và API nên dùng **cùng** `MYSQL_DSN` để consumer đọc outbox API đã ghi.

```bash
export MYSQL_DSN='...'
go run ./cmd/worker
```

Không có DSN: worker dùng outbox in-memory và seed demo (không chia sẻ với API).

## Test

```bash
go test ./...
go build ./...
```

Integration smoke (không cần MySQL): package `internal/httpserver` — `healthz`, `readyz` (không DB → 503), `POST /api/v1/auth/login` user một company.

## Hạn chế đã biết (bootstrap)

- **Không `MYSQL_DSN`**: IAM session/credential, membership query, authorization resolver vẫn in-memory (mất session khi restart; fixture cố định).
- **Audit** không DSN: in-memory (không persist); có DSN: insert `audit_logs`.
- **Notification enqueue** khi có MySQL + outbox MySQL: `notification_jobs` + `outbox_events` trong **một transaction** (`notificationapp.WithTransactionalEnqueue`). Các module khác vẫn autocommit từng lệnh.
- **Admin / access model APIs** vẫn chủ yếu in-memory skeleton (chưa đồng bộ full CRUD lên MySQL như runtime authz).
- MySQL outbox cần **8.0+** (`FOR UPDATE SKIP LOCKED`).

## Tài liệu thêm

- `docs/implementation-step-by-step.md` — lộ trình và DoD (Step **P0.8** — IAM + authz MySQL).
- `docs/api-contracts-json.md` — ví dụ JSON API.
- `docs/ai-cache/cobo-iam-services-iam-authz-mysql-summary.md` — tóm tắt wire MySQL cho IAM / membership / authorization.
