# JWT migration file-checklist summary

- Tai lieu ky thuat chi tiet: `docs/jwt-migration-implementation-plan.md`.
- Bao gom:
  - Interface strategy (giu `TokenIssuer`/`TokenInspector`)
  - Package structure de xuat (`opaque`, `jwt`, `dual`)
  - Config/env changes
  - Checklist code change theo file cu the
  - Tach scope 1-2 PR (PR1 MVP + PR2 hardening)
  - Security checklist, QA checklist, rollout plan

- Muc tieu: team co the chia task implement song song ma khong mat context.

## Update trạng thái

- Đã thêm integration E2E cho mode `dual` tại `internal/httpserver/server_test.go`:
  - login trả JWT access token
  - gọi endpoint protected (`/api/v1/me/effective-access`) bằng JWT thành công
  - fallback verify opaque cũ thành công trên cùng server dual-mode
  - negative cases: JWT sai audience -> 401, opaque invalid -> 401
- Đã thêm integration E2E cho mode `jwt`:
  - login trả JWT
  - protected endpoint chấp nhận JWT
  - opaque token cũ bị từ chối 401 (không fallback)
  - token JWT hết hạn bị từ chối 401 ở HTTP layer (E2E)
