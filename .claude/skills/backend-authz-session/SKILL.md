---
name: backend-authz-session
description: Dùng cho thay đổi auth/session/token/password/permission trong cobo_iam_services - login, refresh rotation, opaque/JWT token, select/switch company, effective access cache Redis, revoke, replay, leo quyền.
---

# Backend Authz Session

## When to use
- Touching login/session/token/password flows.
- Changing permission checks or effective-access computation/cache.
- IAM-sensitive features (invites, recovery, break-glass, delegation).

## Project facts (verify)
- `ACCESS_TOKEN_MODE`: `opaque` (default, deployed) stores access tokens in an
  in-process map with no enforced expiry; `jwt`/`dual` issue JWT access tokens
  (EdDSA default; ES256/HS256 allowed) via `iam/infra/token/jwt/manager.go`.
  All wrapped by `sessionbound` (DB session check per request).
- Refresh tokens: SHA-256 hash in `sessions`, rotated on refresh, TTL 720h
  hardcoded in `internal/httpserver/server.go`.
- Login password may be RSA-OAEP encrypted (`/api/v1/auth/login-password-key`).
- Effective access: Redis key `cobo_iam:effective_access:{companyID}:{membershipID}`,
  TTL `EFFECTIVE_ACCESS_CACHE_TTL`, fail-open to DB. Invalidated only from
  `companyaccess/app/config_versioning.go`; role/permission writes elsewhere
  may leave it stale.
- `login_attempts` is recorded but not enforced.

## Mandatory review areas
- Credential handling, password hashing/verification.
- Token issue/refresh/revoke; expiry and clock skew; opaque token lifetime.
- Role/permission escalation, platform vs tenant admin (`cobo-admin-role-guard`).
- Cache consistency after auth/permission change.
- Replay / duplicate submit.
- Audit and security logging without secrets.

## Workflow
1. Actor, permission boundary, protected resources.
2. Exact state transitions of the auth/session lifecycle.
3. Races between DB and Redis/in-memory state.
4. Revoke/invalidate behavior (session revoke, permission change, company switch).
5. No secrets in logs or responses.
6. Tests: unauthorized, expired, revoked, insufficient privilege, wrong company.

## Output format
- Security-sensitive surface
- State transitions
- Risks found
- Safeguards added
- Tests added
