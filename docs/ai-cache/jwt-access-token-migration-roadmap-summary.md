# JWT access token migration roadmap (signed) + opaque refresh giữ nguyên

## Mục tiêu

- Chuyển `access_token` từ opaque in-memory sang JWT ký số (stateless verify).
- Giữ `refresh_token` dạng opaque và chỉ lưu hash SHA-256 ở DB như hiện tại.
- Không downtime, có backward compatibility theo phase.

## Nguyên tắc thiết kế

- Access token chứa claim tối thiểu: `sub`, `session_id`, `membership_id`, `company_id`, `iat`, `exp`, `iss`, `aud`, `jti`, `typ=access`.
- Không nhét full permissions/roles vào JWT; authorization vẫn đọc runtime data.
- Refresh token tiếp tục rotate + revoke theo DB (`sessions.refresh_token_hash` unique).
- Ký bất đối xứng ưu tiên `EdDSA (Ed25519)` hoặc `ES256`; nội bộ đơn giản có thể `HS256` giai đoạn đầu.

## Lộ trình theo pha

### P0 - Prep (feature flag + config)

- Thêm config:
  - `ACCESS_TOKEN_MODE=opaque|jwt|dual`
  - `JWT_ISSUER`, `JWT_AUDIENCE`
  - key config (`JWT_SIGNING_PRIVATE_KEY_PEM`, `JWT_VERIFY_PUBLIC_KEYS_JSON` hoặc JWKS URL)
  - `JWT_CLOCK_SKEW_SEC` (leeway)
- Tạo interface token provider mới:
  - `IssueAccessToken(claims)` -> hỗ trợ opaque/jwt theo mode.
  - `InspectAccessToken(token)` -> verifier đa chế độ.

### P1 - Dual issue + dual verify

- API login/select/switch/refresh:
  - Mode `dual`: phát JWT access token (primary), vẫn hỗ trợ inspect opaque cũ.
- Inspector:
  - Nếu token có format JWT (`x.y.z`) -> verify JWT.
  - Ngược lại -> fallback opaque inspector cũ.
- Log metric tách bạch:
  - `%jwt_verify_success`, `%opaque_verify_success`, `%verify_error_by_reason`.

### Status hiện tại

- Da implement xong:
  - `internal/iam/infra/token/jwt/manager.go`: Issue/Inspect access token JWT ký số.
  - `internal/iam/infra/token/dual/manager.go`: verify JWT trước, fallback opaque.
  - `internal/httpserver/token_builder.go`: chọn `opaque|jwt|dual` theo config.
  - `internal/iam/infra/token/opaque/manager.go`: tách riêng legacy manager.
- Pre-company và refresh token vẫn đi opaque path (đúng scope PR1).
- Unit tests đã có cho JWT happy path, expired, invalid audience, dual fallback.

### P2 - Rollout canary

- Bật `dual` cho staging -> canary prod.
- Theo dõi:
  - lỗi 401/403 tăng bất thường
  - skew thời gian (`nbf/exp`)
  - latency verify token
- Test matrix:
  - token hết hạn, sai issuer/audience, kid không tồn tại, chữ ký sai.

### P3 - JWT primary

- Chuyển `ACCESS_TOKEN_MODE=jwt`.
- Vẫn giữ khả năng verify opaque thêm 1 khoảng grace period (ví dụ 1-2 tuần) nếu cần.
- Sau grace period: tắt opaque verify.

### P4 - Hardening

- Bật key rotation chuẩn:
  - Header `kid`, publish JWKS verify set.
  - Roll key theo lịch (ví dụ 30/60/90 ngày).
- Thêm blacklist tùy chọn theo `jti` cho incident revoke khẩn cấp (TTL bằng access token TTL).
- Bổ sung chaos test cho key rotation.

## Tác động code chính

- `internal/iam/infra/inmemory/tokens.go`
  - refactor thành 2 implementation:
    - opaque token manager (legacy)
    - jwt token manager (new)
- `internal/iam/app`:
  - không đổi contract lớn nếu giữ interface `TokenIssuer/TokenInspector`.
- `internal/httpserver/server.go`:
  - chọn implementation theo config + feature flag.
- `configs/config.example.env` + README:
  - bổ sung biến JWT.

## Bảo mật/tuân thủ

- Private key không hardcode; inject qua secret manager/env file bảo mật.
- Không log raw token.
- Verify bắt buộc: alg allowlist, issuer, audience, exp/iat, optional nbf.
- Hạn access token ngắn (gợi ý 10-15 phút), refresh token dài + rotate.

## Compatibility và rollback

- Rollback an toàn bằng cách set lại `ACCESS_TOKEN_MODE=opaque`.
- Vì refresh token giữ nguyên cơ chế DB hash, rollback không ảnh hưởng phiên refresh đang có.

## Definition of Done cho migration

- Có dual mode + integration test đầy đủ.
- Canary production không tăng lỗi auth đáng kể.
- JWT mode ổn định >= 1 release cycle.
- Có runbook key rotation + rollback.
