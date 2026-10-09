---
name: cross-repo-security-review
description: Dùng khi thay đổi chạm cả cobo_web_design và cobo_iam_services cần review trust boundary - API authorization, tenant isolation (company_id), IDOR, token flow, permission-to-UI mapping, dữ liệu nhạy cảm trong response/storage/log.
---

# Cross-Repository Security Review

Use together with `frontend-security-audit` [web], `backend-security-audit` [iam]
(iam), `secrets-cors-cookie-review`, `cobo-admin-role-guard` and
`integration-cross-repo`. Read sibling skills from
`../<sibling>/.claude/skills/<name>/SKILL.md`.

## Workflow

1. Draw the request flow: browser → `createXApi`/`createApiClient` → API
   handler → token inspection (`TokenInspector.InspectAccessToken`) →
   `GetEffectiveAccess` permission check → service/repository → MySQL/Redis/
   worker.
2. Verify every browser-controlled identifier (company ID, membership ID,
   record ID, role, status, URL) is revalidated server-side against the token's
   company scope.
3. Compare frontend gates (`routePermissionMatrix`, `RequirePermission`,
   `RequirePlatformAccess`, `hasCmsRoutePermission` aliases) with backend
   checks. Look for hidden-UI-only protection, IDOR, privilege escalation,
   confused deputy, stale permission cache (Redis effective access TTL),
   token replay, cross-tenant exposure.
4. Verify consistent 401/403/404/409 handling, refresh failure, timeout, retry,
   logout and session expiry. Note raw `fetch` calls that bypass the client.
5. Review sensitive data in responses, localStorage/sessionStorage, logs,
   audit events, metrics and errors.
6. Report findings with evidence from both repos; add focused tests for the
   highest-risk boundary.

## Stop conditions

Stop and ask when the tenant boundary, permission owner, token authority or
internal API caller cannot be established from code and docs. Never infer
authorization from UI behavior alone.

## Verification

Changed full-stack flow: `npm test` + `npm run build` (web), `go test ./...`
(iam), plus `docker compose -f docker-compose.dev.yml build api` when backend
image behavior is affected. Report unavailable checks as `BLOCKED:`.
