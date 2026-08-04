# Phase 4 — Security regression

| Case | Result |
|------|--------|
| Unauthenticated GetOwnCompany | non-200 (token inspect fail) |
| Missing `company.view` (deny authz) | 403 |
| Subject-scoped company only (no path company_id) | unchanged; cross-company leak test on service |
| `/me/companies` membership IDs only | batch tests |
| Arbitrary company ID not resolved via GetOwnCompany | N/A path param absent |
| Plan A not on company B | `TestGetOwnCompany_Plan_NoCrossCompanyLeak` |
| CMS `GetPlatformCompany` does not call plan Reader | `TestGetPlatformCompany_DoesNotCallPlanReader` |
| Plan DTO keys only code/display_name/status/source | handler shape test |
| No invoice/payment/billing fields | asserted |
| CompanyTierResolver not used as plan source | plan wired via `companyplan.Service` only |

## Operational consequence (STRICT unchanged)

`companyplan` DB/read outage → **GetOwnCompany** and **`/me/companies` unavailable** (HTTP 500). No partial list with fabricated `plan:null`.

Observability/rollback: additive enrichment; unwind DI / revert Phase 3–4 commits; no schema required to disable reads if table empty (returns null). Outage still 500 when reader errors.

## Concurrency

`MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5` — MySQL unreachable locally; not elevated to PASS.
