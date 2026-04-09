# JWT Access Token Migration - Technical Implementation Plan (1-2 PRs)

Tai lieu nay la ban ky thuat chi tiet de team implement migration:

- Access token: JWT signed
- Refresh token: giu nguyen opaque + hash SHA-256 trong DB

Muc tieu: co the ship trong 1-2 PR, rollback an toan.

---

## 1) Target architecture

## 1.1 Contracts giu nguyen

Khong can doi signature service app IAM:

- `iamapp.TokenIssuer`
- `iamapp.TokenInspector`

Chi doi implementation concrete phia infra/wiring.

## 1.2 New token backends

- `internal/iam/infra/token/opaque` (refactor tu in-memory hien tai)
- `internal/iam/infra/token/jwt` (new)
- `internal/iam/infra/token/dual` (optional adapter issue/verify theo mode)

## 1.3 Claims access token

JWT access token claims toi thieu:

- `sub` (user_id)
- `session_id`
- `membership_id`
- `company_id`
- `iat`, `exp`
- `iss`, `aud`
- `jti`
- `typ=access`

Pre-company token:

- Giai doan dau de giu nhe, co the giu opaque (khong buoc chuyen JWT ngay).

---

## 2) Package structure de xuat

```text
internal/iam/infra/token/
  opaque/
    manager.go          # migrated from current inmemory/tokens.go
  jwt/
    manager.go          # IssueAccessToken + InspectAccessToken
    claims.go           # typed claims struct + validation
    keys.go             # parse keys, kid select, verify set
  dual/
    manager.go          # mode dual: issue jwt, inspect jwt+opaque
```

```text
internal/platform/jwtx/
  signer.go             # Sign(claims) with chosen alg
  verifier.go           # Verify(token) + standard checks
  jwk.go                # optional helper for key-set/JWKS
```

---

## 3) Config changes

## 3.1 File: `internal/platform/config/config.go`

Them fields:

- `AccessTokenMode string` (`opaque|jwt|dual`)
- `JWTIssuer string`
- `JWTAudience string`
- `JWTAlg string` (`EdDSA|ES256|HS256`)
- `JWTPrivateKeyPEM string` (sign)
- `JWTPublicKeysJSON string` (verify key set with kid) hoac `JWTJWKSURL string`
- `JWTClockSkewSec int`
- `AccessTokenTTL time.Duration` (default 15m)

Cap nhat `Load()` parse env + validate:

- mode hop le
- mode `jwt|dual` bat buoc co key verify/sign hop le

## 3.2 File: `configs/config.example.env`

Them env mau:

- `ACCESS_TOKEN_MODE=opaque`
- `ACCESS_TOKEN_TTL=15m`
- `JWT_ISSUER=cobo_iam_services`
- `JWT_AUDIENCE=cobo_clients`
- `JWT_ALG=EdDSA`
- `JWT_SIGNING_PRIVATE_KEY_PEM=...`
- `JWT_VERIFY_PUBLIC_KEYS_JSON=...`
- `JWT_CLOCK_SKEW_SEC=60`

---

## 4) Wiring changes theo file

## 4.1 File: `internal/httpserver/server.go`

Hien tai:

- `tokenManager := iaminmem.NewTokenManager(id)`

Can doi thanh builder theo config:

- `tokenIssuer, tokenInspector := buildTokenManager(d.Config, id)`

Builder logic:

- `opaque` -> opaque manager
- `jwt` -> jwt manager
- `dual` -> dual manager (issue JWT, verify JWT truoc, fallback opaque)

## 4.2 File: `cmd/api/main.go`

Khong doi flow lon.
Chi can pass config da mo rong (tu `config.Load`).

---

## 5) Infra token implementation checklist

## 5.1 PR1 minimum viable (recommended)

Muc tieu PR1: ship duoc JWT access token trong mode `dual`, khong break old clients.

File checklist:

- [ ] `internal/iam/infra/inmemory/tokens.go`
  - tach logic opaque sang package moi `internal/iam/infra/token/opaque/manager.go`
- [ ] `internal/iam/infra/token/jwt/claims.go`
  - struct claims + validate standard fields
- [ ] `internal/iam/infra/token/jwt/manager.go`
  - implement `IssueAccessToken` (JWT signed)
  - implement `InspectAccessToken` (verify sig + iss/aud/exp/skew)
  - `IssueRefreshToken` giu opaque nhu cu (`rtk_<uuid>`) de compatibility
  - `IssuePreCompanyToken` tam thoi co the delegate opaque
- [ ] `internal/iam/infra/token/dual/manager.go`
  - issue access token = JWT
  - inspect access token: JWT first, fallback opaque
  - pre-company + refresh giu hanh vi cu
- [ ] `internal/platform/config/config.go`
  - add fields/env parse
- [ ] `configs/config.example.env`
  - add env docs
- [ ] `internal/httpserver/server.go`
  - replace direct `NewTokenManager` with config-based builder
- [ ] `README.md`
  - section migration mode + env guide

Tests bat buoc PR1:

- [ ] Unit test JWT sign/verify happy path
- [ ] Unit test exp/iss/aud fail
- [ ] Unit test dual mode fallback opaque
- [ ] Smoke `go test ./...`

## 5.2 PR2 hardening + cleanup

Muc tieu PR2: production hardening va deprecation opaque.

File checklist:

- [ ] metrics/logging cho token verify (`jwt_ok`, `opaque_ok`, `jwt_fail_reason`)
- [ ] key rotation support by `kid`
- [ ] optional remove/freeze opaque issue path
- [ ] integration tests (login -> protected API) o `jwt` va `dual` mode
- [ ] runbook rollback mode trong docs

---

## 6) Behavior compatibility matrix

| Mode | Issue access token | Inspect access token | Rollback |
|---|---|---|---|
| `opaque` | Opaque | Opaque only | N/A |
| `dual` | JWT | JWT first, fallback opaque | doi mode ve `opaque` |
| `jwt` | JWT | JWT only (optional grace fallback) | ve `dual`/`opaque` |

---

## 7) Security checklist

- [ ] Khong log raw access/refresh token
- [ ] Verify alg allowlist (khong chap nhan `none`)
- [ ] Verify `iss`, `aud`, `exp`, `iat` (+ leeway)
- [ ] Rotate key theo lich va co rollback key set
- [ ] Access token TTL ngan (10-15m)
- [ ] Refresh token rotation + revoke giu nguyen

---

## 8) QA checklist cho migration

- [ ] Login tra access token JWT trong mode `dual`/`jwt`
- [ ] API protected verify duoc token JWT
- [ ] Token het han -> 401
- [ ] Token sai issuer/audience -> 401
- [ ] Dual mode van chap nhan opaque token cu
- [ ] Refresh flow khong thay doi contract JSON

---

## 9) Rollout plan de team thuc thi

1. Merge PR1, deploy staging (`dual`), test E2E.
2. Canary prod `dual` voi mot ty le traffic.
3. Theo doi 401/403 regression 24-72h.
4. Chuyen `jwt` toan bo.
5. Sau grace period, tat opaque verify.

