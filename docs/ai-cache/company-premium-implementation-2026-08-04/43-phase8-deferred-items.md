# Phase 8 — Deferred items (non-blocking)

1. **DEFERRED_PRODUCT_DECISION_ADMINCENTER_PERSONAL_TIER_BADGE**  
   `AdminCenter.tsx` still renders `user.subscriptionTier`. Personal Ops Premium removed only; not all user-level Premium UI.

2. **NGINX_RATE_LIMIT_TUNING_MONITORING**  
   20r/s / burst 40 accepted for DEV switch storms; monitor limiting-requests / proxy 503 / burst size before Production hardening.

3. **VERIFIED_EMAIL_POSITIVE_RUNTIME_NOT_COVERED_FIXTURE_FALSE_UNIT_AUTHORITY_PASS**  
   DEV smoke accounts have `email_verified=false`; icon correctly absent. Positive tooltip/focus covered by Phase 6 unit tests, not DEV screenshots.

## Explicit non-claims

- Do **not** claim all personal Premium UI removed.
- Do **not** claim verified-email icon visible on DEV screenshots.
- Do **not** claim full `npm test` / `go test ./...` PASS.
