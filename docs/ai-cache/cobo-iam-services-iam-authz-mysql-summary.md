# IAM + authorization MySQL wire — summary

## Khi `MYSQL_DSN` được set

- **IAM**: `internal/iam/infra/mysql` — `CredentialVerifier` (bcrypt), `SessionRepository` (refresh token chỉ lưu SHA-256 hex; rotation đổi hash + `refresh_expires_at`).
- **Company access**: `internal/companyaccess/infra/mysql` — membership query cho login/select/switch và `/me`.
- **Authorization**: `internal/authorization/infra/mysql` — permissions / department scopes / assignments qua join; responsibilities từ `membership_effective_responsibilities` (migration 0003).
- **httpserver**: wire các repo trên khi `pool != nil`; `NewMeHandler` nhận `IdentityQueryService` (MySQL verifier implement `GetByUserID`).

## Migration / seed

- **0005** `sessions.refresh_token_hash` UNIQUE — bắt buộc cho lookup refresh ổn định.
- **Seed dev**: `migrations/seed_dev_identity_authorization.sql` sau 0001, 0003, khuyến nghị 0004 + 0005; user `user@example.com` / `single@example.com`, password `secret`.

## Khi không có DSN

- In-memory IAM + membership fixture + authorization in-memory như bootstrap ban đầu.

## Hạn chế (tại thời điểm ghi)

- Admin/access APIs vẫn skeleton in-memory; khác với runtime authz đọc DB.

## Audit

- Khi có `MYSQL_DSN`, `httpserver` dùng `internal/audit/infra/mysql` → bảng `audit_logs`. Không DSN: `audit/infra/inmemory`.
