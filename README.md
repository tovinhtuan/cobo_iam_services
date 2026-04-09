# cobo_iam_services

Dịch vụ IAM / phân quyền / company context cho nền tảng Cobo (Go). Hai process: **API** (`cmd/api`) và **Worker** (`cmd/worker`).

## Kiến trúc tóm tắt

- **Transport HTTP**: `/api/v1` (client), `/internal/v1` (authorize nội bộ).
- **IAM**: login, refresh, logout, select/switch company, JWT/opaque token (in-memory bootstrap).
- **Authorization**: resolver + checker, effective access, projection cache (in-memory / Redis tùy cấu hình).
- **Audit + Outbox**: audit in-memory; outbox **MySQL** khi có `MYSQL_DSN`, không thì in-memory (chỉ dev).
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
3. `migrations/0004_p1_business_tables.up.sql` (disclosure / workflow / notification)

Ví dụ:

```bash
mysql -u user -p cobo_iam < migrations/0001_init_core.up.sql
mysql -u user -p cobo_iam < migrations/0003_effective_access_projection.up.sql
mysql -u user -p cobo_iam < migrations/0004_p1_business_tables.up.sql
```

Rollback: chạy tương ứng file `*.down.sql` (theo thứ tự ngược).

## Biến môi trường

Xem `configs/config.example.env`. Các biến thường dùng:

| Biến | Ý nghĩa |
|------|---------|
| `MYSQL_DSN` | Kết nối MySQL; bật outbox durable + `/readyz` ready khi ping OK |
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

- IAM session/credential/audit: in-memory, mất khi restart.
- **Notification enqueue** khi có MySQL + outbox MySQL: `notification_jobs` + `outbox_events` trong **một transaction** (`notificationapp.WithTransactionalEnqueue`). Các module khác vẫn autocommit từng lệnh.
- Resolver/checker authorization: in-memory fixture, chưa đọc full từ DB production.
- MySQL outbox cần **8.0+** (`FOR UPDATE SKIP LOCKED`).

## Tài liệu thêm

- `docs/implementation-step-by-step.md` — lộ trình và DoD.
- `docs/api-contracts-json.md` — ví dụ JSON API.
