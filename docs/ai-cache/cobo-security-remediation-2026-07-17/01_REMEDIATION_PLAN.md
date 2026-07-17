# Checkpoint 1 — Remediation Plan

## Verdict before coding
`READY_TO_FIX`

## Scope and priority
- **P0:** COBO-SEC-001 (`/internal/reminders/dispatch` must require auth before business logic).
- **P1:** `/metrics` protection from public unauthenticated access.
- **P1/P2:** Login/refresh rate-limit (implement or dated accepted plan).
- **P2:** CMS media secret fail-safe.
- **P2:** Token storage risk plan (frontend architecture decision).

## Planned implementation
1. Enforce internal token in reminder internal endpoints:
   - require `X-Internal-Token`
   - reject missing/empty/wrong with 401
   - constant-time compare
   - pass token from config instead of hardcoded empty value.
2. Add startup fail-fast for public runtime:
   - reject empty `INTERNAL_REMINDER_TOKEN` outside local/test runtime.
3. Protect `/metrics`:
   - allow loopback/private source IPs
   - allow explicit `X-Internal-Token` for trusted remote scrape
   - deny public unauthenticated requests.
4. CMS media secret hardening:
   - reject `dev-cms-media-secret` outside local/test runtime
   - remove signer fallback default.
5. Rate-limit:
   - document implementation plan for login/refresh with config-driven thresholds (follow-up PR if not landed in this cycle).
6. Retest:
   - unit/integration tests for internal dispatch, startup guards, metrics protection
   - runtime curl matrix after deploy.

## Risk controls
- No destructive data operations.
- No secret/token leakage in evidence.
- Keep existing business behavior unchanged except security gates.
