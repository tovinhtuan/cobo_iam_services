# Exact file / package impact map

| Phase | File/package | Change | Why | Risk | Tests |
|-------|--------------|--------|-----|------|-------|
| 1 | **NEW** `internal/subscription/companyplan/types.go` | `Plan` DTO + source enum | Shared contract | Low | unit |
| 1 | **NEW** `internal/subscription/companyplan/reader.go` | `Reader` interface `PlanForCompany(ctx, companyID) (Plan, error)` | Single semantics | Med | unit |
| 1 | **NEW** `internal/subscription/companyplan/entitlement_reader.go` | Wrap `CompanyTierResolver` → Plan | Case B interim | Med — must set `source` | unit |
| 1 | **NEW** `internal/subscription/companyplan/mysql_company_subscription_reader.go` | Optional Case C | After migration | High | repo IT |
| 1 | `internal/subscription/entitlement/company_tier_mysql.go` | **Reuse, do not semantic-rename** | Keep entitlement meaning | Low | existing |
| 2 | `internal/companyaccess/app/platform_company.go` | Add `Plan *companyplan.PlanWire` json `plan,omitempty` | GetOwnCompany response | Low additive | mapper tests |
| 2 | `internal/companyaccess/app/admin_service.go` | `GetOwnCompany` attach plan via Reader | Expose A | Med | `admin_service_owncompany_test.go` |
| 2 | `internal/companyaccess/app/admin_options.go` | `WithCompanyPlanReader` | DI | Low | wiring |
| 2 | `internal/httpserver/server.go` | Wire Reader into admin + me | Composition root | Med | compile |
| 2 | `internal/iam/transport/http/me_handler.go` | Attach `plan` on companies items | Option B | Med N+1 | `me_handler_companies_test.go` |
| 2 | Possibly batch helper in memberships repo | `PlansForCompanies(ctx, ids)` | Avoid N+1 | Med | IT |
| 3 | `internal/companyaccess/transport/http/admin_handler_owncompany_test.go` | Assert plan JSON | Contract | Low | handler |
| 3 | **NEW** `internal/subscription/companyplan/*_test.go` | Matrix | Correctness | — | unit |
| 4 | **NEW** `migrations/0xxx_company_subscriptions.up.sql` | Only if Case C | SoT | High | migrate dry-run |
| 7 FE | `companyProfileApi.ts`, `types.ts` AuthorizedCompany, `PersonalOpsScreen.tsx`, Company overview | Consume + remove personal Premium | After BE DEV | Med | vitest |

**Do not change** in Backend implement of badge SoT alone: CMS payment handlers until billing SoT decided.
