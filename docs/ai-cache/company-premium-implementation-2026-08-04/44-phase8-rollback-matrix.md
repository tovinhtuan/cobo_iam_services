# Phase 8 — Rollback matrix (documented; not executed)

## FE source/UI

- Redeploy previous FE asset (e.g. `index--sUVChju.js`) / prior FE commit via `make deploy-fe`.
- New Backend + old FE supported (old FE ignores `plan`).
- After FE rollback, Personal Ops may show `user.subscriptionTier` again if that build included it.

## Nginx

- Restore `api_per_ip` **5r/s** burst **20** in `deploy-artifacts/web/nginx.conf`.
- Recreate **web** only (`--no-deps --force-recreate`).
- API/worker/MySQL unchanged.
- Risk: company-switch burst **503** returns.

## Backend API

- Redeploy previous API binary / `make deploy-be` from prior commit.
- Old FE + old Backend supported.
- New FE must fail closed if `plan` absent/null.

## DEV seed

```sql
DELETE FROM company_subscriptions WHERE origin = 'dev_fixture';
```

Only after count check; do not wipe non-fixture rows.

## Migration 0125

Only if no non-fixture rows remain, seed cleaned, and rollback explicitly required. Prefer leaving additive schema.

Phase 8 **does not** execute any rollback.
