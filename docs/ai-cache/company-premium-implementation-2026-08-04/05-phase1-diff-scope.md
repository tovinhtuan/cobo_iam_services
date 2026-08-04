# Phase 1 diff scope

| Path | Classification |
|------|----------------|
| `internal/subscription/companyplan/*` | REQUIRED_PHASE_CHANGE + REQUIRED_TEST |
| `migrations/0125_company_subscriptions.*` | REQUIRED_MIGRATION |
| `migrations/0126_dev_company_subscription_fixtures.*` | REQUIRED_MIGRATION (DEV fixture) |
| `docs/ai-cache/company-premium-implementation-2026-08-04/03-05*` | REQUIRED_EVIDENCE |
| `docs/ai-cache/README.md`, `reusable-task-updates.md` | REQUIRED_EVIDENCE |
| handlers / FE / deploy-artifacts | **unchanged** |

No `CompanyTierResolver` / `user_subscription_tiers` references in new package.
