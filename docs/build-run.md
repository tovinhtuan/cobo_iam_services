# Build | Run Guide

Tai lieu nay huong dan build va run `cobo_iam_services` theo 2 mode:

- local nhanh (khong MySQL)
- local day du (co MySQL + migration + seed)

---

## 1) Preconditions

- Go `1.22+`
- (Khuyen nghi) MySQL `8.x` neu can run day du
- Dang o root module: `cobo_iam_services/`

Kiem tra nhanh:

```bash
go version
```

---

## 2) Build

### Build toan bo package

```bash
go build ./...
```

### Build binary API + Worker

```bash
go build -o bin/api ./cmd/api
go build -o bin/worker ./cmd/worker
```

---

## 3) Run nhanh (khong MySQL)

Phu hop de smoke API co ban.

```bash
go run ./cmd/api
```

Kiem tra:

```bash
curl -sS http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8080/readyz
```

Expected:

- `/healthz` -> `200`
- `/readyz` -> `503` (vi khong cau hinh DB)

> Note: mode nay dung in-memory cho nhieu thanh phan (session, membership fixture, authz fixture...).

---

## 4) Run day du voi MySQL

## 4.1 Tao DB va export DSN

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS cobo_iam CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
export MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false'
```

## 4.2 Chay migration

```bash
mysql -u user -p cobo_iam < migrations/0001_init_core.up.sql
mysql -u user -p cobo_iam < migrations/0003_effective_access_projection.up.sql
mysql -u user -p cobo_iam < migrations/0004_p1_business_tables.up.sql
mysql -u user -p cobo_iam < migrations/0005_sessions_refresh_hash_unique.up.sql
mysql -u user -p cobo_iam < migrations/0006_admin_rules_tables.up.sql
```

## 4.3 Seed du lieu dev

```bash
mysql -u user -p cobo_iam < migrations/seed_dev_identity_authorization.sql
```

## 4.4 Run API + Worker

Terminal 1:

```bash
go run ./cmd/api
```

Terminal 2:

```bash
go run ./cmd/worker
```

Kiem tra:

```bash
curl -sS http://127.0.0.1:8080/readyz
```

Expected: `200 {"status":"ready"}`

---

## 5) Auth token mode (opaque / dual / jwt)

Config qua env:

```bash
export ACCESS_TOKEN_MODE=opaque   # or dual or jwt
export ACCESS_TOKEN_TTL=15m
export JWT_ISSUER=cobo_iam_services
export JWT_AUDIENCE=cobo_clients
export JWT_ALG=HS256
export JWT_SIGNING_PRIVATE_KEY_PEM='replace-me'
export JWT_CLOCK_SKEW_SEC=60
```

Goi y cho local:

- `opaque`: on dinh nhat
- `dual`: migration mode (issue JWT, verify JWT truoc roi fallback opaque)
- `jwt`: JWT-only (khong fallback opaque)

---

## 6) Test

### Unit + integration trong repo

```bash
go test ./...
```

### Focus test JWT migration

```bash
go test ./internal/iam/infra/token/... -v
go test ./internal/httpserver -v
```

---

## 7) Login smoke (dev seed)

Request:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"login_id":"single@example.com","password":"secret"}'
```

Expected:

- `200`
- co `session.access_token`
- co `session.refresh_token`

---

## 8) Troubleshooting nhanh

- `readyz = 503`:
  - kiem tra `MYSQL_DSN`
  - kiem tra MySQL co connect duoc khong
- login fail `INVALID_CREDENTIALS`:
  - xac nhan da seed (`seed_dev_identity_authorization.sql`)
- loi migration:
  - chay dung thu tu file `.up.sql`
- worker khong thay event:
  - API va Worker phai dung chung `MYSQL_DSN`

