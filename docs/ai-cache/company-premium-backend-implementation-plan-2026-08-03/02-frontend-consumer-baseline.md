# Frontend consumer baseline (read-only)

| Item | Value |
|------|-------|
| Repo | `cobo_web_design` |
| Branch | `recovery/lost-changes-audit-20260717-153324` |
| HEAD | `b6631498fe6b348bdb59e314c69199683293870b` |
| Working tree | clean |

## Current consumers
| Concern | Exact source |
|---------|----------------|
| Personal Premium badge | `PersonalOpsScreen.tsx` ← `user.subscriptionTier` ← `/api/v1/me` `user.subscription_tier` |
| Company Information | `CompanyProfilePage` → `useCompanyProfilePage` → `createCompanyProfileApi().getOwnCompany()` → `GET /api/v1/admin/company` |
| `CompanyProfile` type | `src/features/admin-core/services/companyProfileApi.ts` — **no plan field** |
| Selected companies | `GET /api/v1/me/companies` → `AuthorizedCompany` — **no plan** |
| Overview placement | Company name lives in overview cards; header is page title + code (`CompanyProfileHeader.tsx`) — badge should go next to company name in overview, not sidebar |

## Prior Phase B pack
`docs/ai-cache/company-premium-api-contract-plan-2026-08-03/` (FE) — revalidated, not contradicted.
