# Phase 5 — Security smoke

| Check | Result |
|-------|--------|
| Unauthenticated `GET /api/v1/admin/company` | 401 |
| GetOwnCompany subject-scoped (no path company_id) | PASS |
| `/me/companies` only caller memberships (`c_001`,`c_002` for user@example.com) | PASS |
| No cross-company plan leak (c_002 item null; c_001 Premium) | PASS |
| Plan DTO keys only `code`,`display_name`,`status`,`source` | PASS |
| CMS `GET /api/v1/platform/cms/admin/companies/c_001` → `data.plan` null (no Reader enrich) | PASS |
| No CompanyTierResolver / user tier used as company plan | PASS (response source=`COMPANY_SUBSCRIPTION`) |
| Credentials/tokens not stored in evidence | PASS |

Unit coverage remains authority for STRICT reader-error → 500 (no intentional DEV outage).
