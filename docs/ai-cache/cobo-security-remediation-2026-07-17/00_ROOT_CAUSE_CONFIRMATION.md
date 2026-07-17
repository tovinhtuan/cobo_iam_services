# Checkpoint 0 — Root Cause Confirmation

## Audit evidence consulted
- `docs/ai-cache/cobo-security-logic-audit-2026-07-17/15_FINDINGS_SUMMARY.md`
- `docs/ai-cache/cobo-security-logic-audit-2026-07-17/16_DETAILED_FINDINGS.md`
- `docs/ai-cache/cobo-security-logic-audit-2026-07-17/20_FINAL_VERDICT.md`
- `docs/ai-cache/cobo-security-logic-audit-2026-07-17/qa-results.json`

## Source confirmation (10 required questions)
1. `/internal/reminders/dispatch` registered in `internal/reminder/transport/http/handler.go` (`Register()`).
2. No central internal-token middleware existed; token check was inline in handler method.
3. When token env was empty, old logic skipped auth check (`if h.internalToken != "" && token != h.internalToken`).
4. Expected header: `X-Internal-Token`.
5. Legitimate caller: internal reminder pipeline (worker/API internal dispatch path) and controlled DEV internal smoke flows.
6. `/internal/*` reachable via public API port because route registered on same mux behind `:8080`.
7. `/metrics` reachable from public API (`mux.Handle("/metrics", promhttp.Handler())`).
8. Login/refresh handlers are in `internal/iam/transport/http/handler.go` (`POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`) and registered by `internal/httpserver/server.go`.
9. CMS media default secret defined in `internal/platform/config/config.go` and fallback in `internal/platformcms/transport/http/media_security.go`.
10. Existing tests before remediation covered auth flows and metrics collectors broadly, but did not enforce:
   - mandatory internal token for dispatch
   - startup fail-fast when internal token empty in public runtime
   - `/metrics` public access restriction
   - reject default CMS media secret for non-local/test runtime.

## Root cause conclusion
- **COBO-SEC-001 confirmed:** authentication guard for internal dispatch failed open when token config missing.
- **Metrics exposure confirmed:** `/metrics` had no access control.
- **Config hardening gap confirmed:** CMS media secret default accepted in normal runtime.
- **Rate-limit finding confirmed (source):** login/refresh had no request throttling guard in transport path.
