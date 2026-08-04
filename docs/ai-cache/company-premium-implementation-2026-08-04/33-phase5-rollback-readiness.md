# Phase 5 — Rollback readiness

## Backend API

- Previous binary sha256: `c827dc4e56e6ea03a0fed9be582f6d65af9be2de83edfb09e78faaca84529de8`
- Redeploy previous artifact via approved `make deploy-be` from prior commit, or restore binary + `docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker`
- Health: `/healthz` ok, `/readyz` ready

## Seed cleanup (DEV only)

```sql
SELECT COUNT(1) FROM company_subscriptions WHERE origin='dev_fixture';
DELETE FROM company_subscriptions WHERE origin='dev_fixture';
```

Do not delete non-fixture rows.

## Migration down (only if required)

`0125_company_subscriptions.down.sql` drops `company_subscriptions`. Only after fixture cleanup and when no non-fixture data. **Not executed** after successful Phase 5.

## Partial matrix

| Stack | Status |
|-------|--------|
| New BE + old FE | Supported (additive `plan`) |
| Old BE + old FE | Supported |
| New BE + future FE | Phase 6 |
| Old BE + future FE | FE must fail closed; do not deploy FE first |

## Worker / FE / DB

- Worker recreated by approved Makefile (no Phase 5 worker logic change).
- FE not redeployed.
- MySQL not restarted.
