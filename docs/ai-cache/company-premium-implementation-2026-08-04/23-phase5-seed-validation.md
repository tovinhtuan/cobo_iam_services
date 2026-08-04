# Phase 5 — DEV seed validation

## Fixture contract

| Company | Expectation | Observed |
|---------|-------------|----------|
| c_001 | PREMIUM ACTIVE, origin=dev_fixture, open-ended window | PASS — row `cps_dev_c001_premium`, effective_from=2020-01-01, expires_at=NULL |
| c_002 | no company_subscriptions row | PASS — count 0 |
| origin=dev_fixture | only approved fixture | PASS — total rows 1, fixture count 1 |

## Side effects

- Companies table not mutated beyond existing c_001 / c_002 presence.
- Numbered 0126 not applied (absent).
- Seed tracked in `schema_migrations` as `seed_dev_company_subscriptions.sql`.

## Cleanup (documented, not executed)

```sql
DELETE FROM company_subscriptions WHERE origin = 'dev_fixture';
```

## Verdict

**SEED_VALIDATION_PASS**
