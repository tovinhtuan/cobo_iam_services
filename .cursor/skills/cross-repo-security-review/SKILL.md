---
name: cross-repo-security-review
description: Dùng khi một thay đổi chạm cả cobo_web_design và cobo_iam_services, cần review trust boundary, API authorization, tenant isolation, token flow và permission-to-UI mapping.
---

# Cross-Repository Security Review

Use this skill together with `backend-security-audit`, `frontend-security-audit`
from the sibling repository, and `integration-cross-repo`.

## Workflow

1. Draw the request flow: browser -> frontend client -> API boundary -> IAM
   checks -> service/repository -> database/cache/worker.
2. Verify that browser-controlled identifiers, company IDs, roles, actions,
   statuses, and URLs are revalidated by the backend where security-relevant.
3. Compare frontend permission assumptions with backend authorization rules.
   Check IDOR, privilege escalation, confused deputy, stale permission cache,
   token replay, and cross-tenant data exposure.
4. Verify consistent 401/403/404/409 handling, refresh failure, timeout, retry,
   logout, and session expiry behavior.
5. Review sensitive data in responses, client state, browser storage, logs,
   audit events, metrics, and errors.
6. Report findings with evidence from both repositories and add focused tests
   for the highest-risk boundary.

Stop and request clarification when tenant boundary, permission owner, token
authority, or internal API caller cannot be established from evidence. Do not
infer authorization from UI behavior alone.

## Verification

For a changed full-stack flow, run the relevant frontend checks, `go test ./...`,
and the Docker API build when backend image behavior is affected. Report any
unavailable check as `BLOCKED:` with its concrete reason.
