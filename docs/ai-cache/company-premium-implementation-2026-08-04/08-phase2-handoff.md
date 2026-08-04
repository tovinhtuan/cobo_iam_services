# Phase 2 handoff — await confirmation before Phase 3

## Verdict

**PHASE_2_SHARED_READER_READY** with open risk:

`MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5`

Lock strategy is implemented (parent `companies` `FOR UPDATE` + occupying row locks + overlap reject). Concurrency proof against live MySQL was **not** obtained in this environment — do **not** claim overlap concurrency PASS until Phase 5 re-runs `TestMySQLCreate_ConcurrentOverlap_EmptyCompany` with DEV MySQL + `0125` applied.

## Contract unchanged (Phase 0–1)

- SoT: `company_subscriptions` by `company_id`
- Wire later: `plan` object or `plan: null`; source `COMPANY_SUBSCRIPTION`
- Badge (FE later): `PREMIUM` + `ACTIVE` + `COMPANY_SUBSCRIPTION` only
- No user-tier / CompanyTierResolver fallback

## Delivered this phase

1. Retracted numbered DEV fixture `0126` → `seed_dev_company_subscriptions.sql` + `run_dev_migrations.sh` whitelist
2. Shared `companyplan.Service` Reader façade + batch semantics tests
3. Create path gap-lock fix via parent company row
4. Evidence `06`–`08`

## Next (Phase 3) — requires user confirmation

- Expose additive `plan` on GetOwnCompany + `/me/companies` only
- Map via shared Reader; authz unchanged
- Still no FE / no deploy until later phases

## Rollback notes

- Source: revert Phase 2 commits
- Seed: `DELETE FROM company_subscriptions WHERE origin='dev_fixture'` (only if seed was applied; Phase 2 did **not** apply migrate)
- Schema `0125` untouched by this phase's seed move
