# Phase 2 — COBO-SEC-001 Fix

## Code changes
- `internal/reminder/transport/http/handler.go`
  - Added strict `authorizeInternalRequest()` for internal endpoints.
  - Enforced `X-Internal-Token` for both:
    - `POST /internal/reminders/dispatch`
    - `POST /internal/dev/reminders/seed-occurrence`
  - Missing/empty/wrong token now returns 401 before business payload handling.
  - Comparison now uses constant-time `hmac.Equal`.
- `internal/httpserver/server.go`
  - `reminderhttp.NewHandler(..., cfg.InternalReminderToken, cfg.Env)` now receives real configured token.
- `internal/platform/config/config.go`
  - Added `InternalReminderToken` and `InternalAuthAllowEmptyForTest`.
  - Added validation: empty internal token rejected outside local/test runtime.

## Runtime retest (DEV)
- No token: `401`
- Wrong token: `401`
- Valid token (masked): request reaches handler (`400` business validation, not auth block)

## Result
COBO-SEC-001 moved from **Open** to **Fixed (verified on runtime)**.
