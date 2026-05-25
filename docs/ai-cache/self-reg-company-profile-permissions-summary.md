# Self-reg company profile permissions (unverified company)

## Problem

Public register with company assigns global role `self_reg_company_owner` (`r0000000-0001-4000-8000-000099999001`).

- Role permissions were cloned from `full_access` in migration **0026** (before `company.view` / `company.edit` existed in **0066**).
- **0066/0067** grant company profile permissions to `admin_doanh_nghiep`, `full_access`, etc., but **not** `self_reg_company_owner`.
- Self-reg user is **not** assigned tenant `admin_doanh_nghiep` membership role — only the global self-reg role.

Result: FE route `/app/admin/company` and API `GET/PATCH /api/v1/admin/company` deny access (missing `company.view` / `company.edit`). Company `verification_status=unverified` is **not** the blocker on BE/FE guards.

## Fix (two layers)

1. **Runtime — public register with company** (`RegisterPublicAccount` in `register_public.go`):
   - `grantRoleCompanyProfilePermissionsTx` on global `self_reg_company_owner` at register time.
   - `grantRoleCompanyProfilePermissionsTx` on new tenant `admin_doanh_nghiep` role inside `InsertCompanyWithDefaultRolesTx`.
   - Membership gets **both** roles: `self_reg_company_owner` + tenant `admin_doanh_nghiep` (registering user is company admin while `unverified`).

2. **Migration 0073** (backfill): grant `company.view` + `company.edit` on global `self_reg_company_owner` for accounts created before the code change.

After deploy: apply migration for existing users; re-login to refresh effective-access cache.

## Related code

| Layer | Path |
|-------|------|
| Register + role assign | `internal/iam/registrationmysql/register_public.go` |
| Global self-reg role | `migrations/0026_self_registration_owner_role.up.sql` |
| Company profile perms | `migrations/0066_dev_company_profile_permissions.up.sql` |
| FE route guard | `cobo_web_design/src/config/menuPermissionMatrix.ts` → `company` |
| FE page | `features/admin-core/pages/CompanyProfilePage.tsx` |
| BE | `companyaccess/app/admin_service.go` — `GetOwnCompany` / `PatchOwnCompany` |

## FE bug (2026-05-25): API OK but UI `Forbidden: PERMISSION_DENIED`

`CompanyProfilePage` uses `useGuard(..., permissions)` from `useCompanyContext`. That hook was a **stub** returning `permissions: []` always, while `GET /api/v1/admin/company` succeeded via bearer token.

**Fix (cobo_web_design):** `useCompanyContext` reads `user.permissions` and `selectedCompany` from `useAuth()` — same source as `RequirePermission` route guards.

## Intentionally unchanged

- Company remains `verification_status=unverified` after self-reg.
- No bypass of platform verification workflows; only opens tenant company profile CRUD for the registering owner.
