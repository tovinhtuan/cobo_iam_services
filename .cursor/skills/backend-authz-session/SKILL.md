---
name: backend-authz-session
description: Dùng cho các thay đổi liên quan auth, session, token, password, quyền truy cập, cache xác thực hoặc IAM policy
---

# Backend Authz Session

## When to use
Use this skill when:
- touching login/session/token/password flows
- changing permission checks
- changing Redis-backed auth/session behavior
- implementing IAM-sensitive features

## Mandatory review areas
- credential handling
- password hashing/verification path
- token issue/refresh/revoke path
- expiry and clock skew
- role/permission escalation paths
- cache consistency after auth change
- replay / duplicate submit risks
- audit/security logging

## Workflow
1. Xác định actor, permission boundary, protected resources.
2. Liệt kê exact state transitions cho auth/session lifecycle.
3. Kiểm tra race conditions giữa DB và Redis state.
4. Định nghĩa revoke/invalidate behavior.
5. Đảm bảo secrets không bị log hoặc trả ra ngoài.
6. Thêm tests cho unauthorized, expired, revoked, insufficient privilege.

## Output format
- Security-sensitive surface
- State transitions
- Risks found
- Safeguards added
- Tests added
