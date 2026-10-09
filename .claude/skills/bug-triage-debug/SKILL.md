---
name: bug-triage-debug
description: Dùng khi điều tra và sửa bug trong Cobo (frontend, backend hoặc liên repo) - tái hiện lỗi, khoanh vùng FE/BE/DB/worker qua network, log slog, request id, ai-cache, tìm root cause, sửa nhỏ nhất và thêm test regression.
---

# Bug Triage & Debug

## Workflow
1. **Capture**: symptom, expected vs actual, environment, user role/company,
   time, request id (`X-Request-Id`), screenshots/console.
2. **Search history**: `docs/ai-cache/` (grep the feature/error code),
   `git log -S '<symbol>'`, related SPEC files.
3. **Reproduce** at the lowest level possible: Go unit/handler test,
   Vitest test, curl/Postman call, then browser (`e2e-playwright-cobo` [web]).
4. **Localize** the layer:
   - Network tab / `ApiError {status, payload.error.code}` → FE or BE?
   - BE: `slog` JSON logs by request id (`make dc-logs` / `make dev-logs`),
     handler → service → repository; in-memory vs MySQL difference?
   - Data: query MySQL state; migration listed and applied?
   - Async: outbox row status, reminder status, worker logs.
   - Cache: stale Redis effective access, old FE bundle, localStorage from
     another company/env.
   - Flags: `VITE_*` and backend env flags.
5. **Root cause** stated in one sentence with evidence (file:line, log,
   query). Distinguish confirmed from suspected.
6. **Fix** the root cause with the smallest safe change; no drive-by
   refactors.
7. **Regression test** that fails before and passes after.
8. **Blast radius**: other callers of the changed code, sibling repo
   contract, data already corrupted (needs a backfill?).

## Common Cobo pitfalls
- New handler missing token/permission check.
- Migration file not added to `run_dev_migrations.sh`.
- Outbox event type without handler (silently marked processed).
- Detached goroutine using a cancelled request context.
- Deadline math in UTC instead of `Asia/Ho_Chi_Minh`.
- Mojibake from wrong file encoding (`vi-text-encoding`).
- Raw `fetch` bypassing 401 refresh / 403 handling in FE.

## Output format
```text
Symptom:
Reproduction:
Root cause (evidence):
Fix:
Regression test:
Blast radius / data repair:
```
