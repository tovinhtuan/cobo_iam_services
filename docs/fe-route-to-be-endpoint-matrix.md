# FE Route To BE Endpoint Matrix

> Updated: 2026-05-27
> Scope: `cobo_web_design` routes mapped to current `cobo_iam_services` HTTP surfaces

## Status Legend

- `Implemented`: FE route exists and matching BE endpoint(s) are currently wired
- `Partial`: FE route exists but BE support is indirect, incomplete, or only partially traceable from current code
- `Spec-only`: documented in specs/notes but not confirmed as a current FE route + BE endpoint pair

## Public And Auth

| FE route | Main purpose | BE endpoint(s) | Status |
|---|---|---|---|
| `/` | landing | none | Implemented |
| `/terms` | legal terms | none | Implemented |
| `/privacy` | privacy policy | none | Implemented |
| `/pricing` | pricing page | none | Implemented |
| `/docs` | product docs page | none | Implemented |
| `/login` | authenticate user | `GET /api/v1/auth/login-password-key`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/logout` | Implemented |
| `/register` | public registration | `POST /api/v1/auth/register` | Implemented |
| `/accept-invitation` | invitation acceptance | `GET /api/v1/auth/invitations/validate`, `POST /api/v1/auth/invitations/accept` | Implemented |
| `/reset-password` | reset password from token | `POST /api/v1/auth/reset-password` | Implemented |

## Company Context And Session Bootstrap

| FE route | Main purpose | BE endpoint(s) | Status |
|---|---|---|---|
| auth company selection flow | choose active company after login | `POST /api/v1/auth/select-company`, `POST /api/v1/auth/switch-company`, `POST /api/v1/me/active-company`, `GET /api/v1/me/companies` | Implemented |
| `/app/no-company` | first company self-service bootstrap | `POST /api/v1/company/initialize` (Bearer), `GET /api/v1/me`, `GET /api/v1/me/companies` | Implemented |
| Portal company switcher CTA | create Nth company (feature flag) | `POST /api/v1/company/create` (Bearer + `Idempotency-Key`) | Implemented |
| `/app/profile` (create nth link) | same as switcher CTA when `VITE_COMPANY_SELF_CREATE_ENABLED` | `POST /api/v1/company/create` | Implemented |
| `/app/verify-email` | verify account email | `POST /api/v1/auth/verify-email`, `POST /api/v1/auth/resend-verification-email` | Implemented |
| global portal bootstrap | current user/company/permissions | `GET /api/v1/me`, `GET /api/v1/me/profile`, `GET /api/v1/me/companies`, `GET /api/v1/me/authorized-companies`, `GET /api/v1/me/effective-access`, `GET /api/v1/me/capabilities`, `GET /api/v1/me/membership` | Implemented |

## Portal Business Routes

| FE route | Main purpose | BE endpoint(s) | Status |
|---|---|---|---|
| `/app/dashboard` | dashboard summary | `GET /api/v1/me`, `GET /api/v1/me/effective-access` plus dashboard/disclosure/deadline service surfaces inferred from FE | Partial |
| `/app/company/recipients` | manage recipients | recipient/company operational APIs inferred from FE design | Partial |
| `/app/deadlines` | deadline list/history shell | reminder/disclosure runtime APIs inferred from FE design | Partial |
| `/app/deadlines/:id` | deadline detail | reminder/disclosure detail APIs inferred from FE design | Partial |
| `/app/history` | history tab redirect | same deadline/history support as above | Partial |
| `/app/history/:id` | historical detail | disclosure/deadline detail support inferred | Partial |
| `/app/disclosure-types` | list disclosure types | `GET /api/v1/disclosure-types` and company override endpoints used by FE services | Implemented |
| `/app/disclosure-types/:id` | disclosure type detail | `GET /api/v1/disclosure-types/:typeId`, `/api/v1/company/disclosure-types/:typeId/workflow-override/*`, `/api/v1/admin/disclosure-types/:typeId/config`, `/api/v1/admin/disclosure-types/:typeId/versions` | Implemented |
| `/app/disclosures/new` | create disclosure | `POST /api/v1/disclosures`, `GET /api/v1/disclosure-types`, disclosure workflow/reminder support | Implemented |
| `/app/disclosures/:id` | disclosure detail | `GET /api/v1/disclosures/:id` | Implemented |
| `/app/disclosures/:id/edit` | edit disclosure | `GET /api/v1/disclosures/:id`, `PATCH /api/v1/disclosures/:id` | Implemented |
| `/app/ad-hoc-proposals` | proposal list | `GET /api/v1/company/ad-hoc-proposals` | Implemented |
| `/app/ad-hoc-proposals/new` | create proposal | `GET /api/v1/company/ad-hoc-proposals/eligible-controllers`, `POST /api/v1/company/ad-hoc-proposals` | Implemented |
| `/app/ad-hoc-proposals/:proposalId` | proposal detail/actions | `GET /api/v1/company/ad-hoc-proposals/{proposal_id}`, `POST /submit`, `POST /focal-approve`, `POST /admin-approve`, `POST /reject`, `POST /cancel` | Implemented |
| `/app/alert-channels` | channel/rule management | `POST /api/v1/notifications/resolve-recipients`, `POST /api/v1/notifications/enqueue`, `POST /api/v1/notifications/dispatch` plus portal notification settings contracts | Partial |
| `/app/profile` | user profile (identity hub) | `GET /api/v1/me`, `GET /api/v1/me/profile`, `GET /api/v1/me/companies`, `PATCH /api/v1/me/profile`, `POST /api/v1/me/change-password` | Implemented |
| `/app/settings` | user settings shell | current route exists; backend shape not explicitly isolated | Partial |

## Enterprise Admin Routes

| FE route | Main purpose | BE endpoint(s) | Status |
|---|---|---|---|
| `/app/admin` | admin center shell | `GET /api/v1/admin/hub/summary` and admin module APIs | Partial |
| `/app/admin/hub` | admin hub summary | `GET /api/v1/admin/hub/summary` | Implemented |
| `/app/admin/users` | users and memberships | admin membership/user endpoints via `membershipAdminApi` | Implemented |
| `/app/admin/roles` | roles and permissions | role/permission admin endpoints via admin/companyaccess + authorization | Implemented |
| `/app/admin/rules` | rules builder | admin rules endpoints via companyaccess | Implemented |
| `/app/admin/audit` | audit and sessions | audit/session admin APIs | Implemented |
| `/app/admin/company` | own company profile | `GET /api/v1/admin/company`, `PATCH /api/v1/admin/company` | Implemented |
| `/app/admin/departments` | department management | `/api/v1/admin/departments*`, member assignment endpoints | Implemented |
| `/app/admin/titles` | title management | title admin endpoints via companyaccess | Implemented |

## CMS Routes

| FE route | Main purpose | BE endpoint(s) | Status |
|---|---|---|---|
| `/cms` | CMS dashboard | `GET /api/v1/platform/cms/dashboard/summary` | Implemented |
| `/cms/content/collections` | collections list | `GET /api/v1/platform/cms/collections` | Implemented |
| `/cms/content/collections/:collectionId` | collection detail | `GET /api/v1/platform/cms/collections/:collectionId` | Implemented |
| `/cms/content/entries` | entries list | `GET /api/v1/platform/cms/entries` | Implemented |
| `/cms/content/entries/:entryId` | entry editor | `GET /api/v1/platform/cms/entries/:entryId`, `POST /api/v1/platform/cms/entries`, `PUT /api/v1/platform/cms/entries/:entryId` | Implemented |
| `/cms/content/media` | media library | `POST /api/v1/platform/cms/media/upload`, `PUT /api/v1/platform/cms/media/upload/:asset_id`, `POST /api/v1/platform/cms/media/:asset_id/complete`, `GET /api/v1/platform/cms/media`, `DELETE /api/v1/platform/cms/media/:asset_id` | Implemented |
| `/cms/publishing/review` | review queue | `GET /api/v1/platform/cms/reviews`, `POST /api/v1/platform/cms/reviews/:entry_id` | Implemented |
| `/cms/publishing/schedule` | schedules | `GET /api/v1/platform/cms/schedules`, `POST /api/v1/platform/cms/schedules`, `DELETE /api/v1/platform/cms/schedules/:entry_id` | Implemented |
| `/cms/publishing/releases` | releases | `GET /api/v1/platform/cms/releases` | Implemented |
| `/cms/taxonomy` | taxonomy management | FE route/spec exists; backend route not currently visible in `platformcms` handler list | Partial |
| `/cms/templates` | disclosure template management | `GET /api/v1/disclosure-types`, `GET /api/v1/disclosure-types/:typeId`, `PUT /api/v1/admin/disclosure-types/:typeId`, `GET /api/v1/admin/disclosure-types/:typeId/versions` | Implemented |
| `/cms/admin/companies` | company management | `GET /api/v1/platform/cms/admin/companies`, `POST /api/v1/platform/cms/admin/companies` | Implemented |
| `/cms/admin/companies/:companyId` | company detail and actions | `GET /api/v1/platform/cms/admin/companies/:companyId`, `PATCH /api/v1/platform/cms/admin/companies/:companyId`, `POST /deactivate`, `POST /activate` | Implemented |
| `/cms/admin/users` | platform users and memberships | `GET /api/v1/platform/cms/admin/users`, `POST /api/v1/platform/cms/admin/users`, `POST /invite`, `POST /resend-invitation`, `POST /assign-company`, `POST /companies/:company_id/members`, `POST /request-password-reset` | Implemented |
| `/cms/admin/roles` | platform roles | `GET /api/v1/platform/cms/admin/roles` | Implemented |
| `/cms/admin/rules` | platform rules | `GET /api/v1/platform/cms/admin/rules`, `POST /api/v1/platform/cms/admin/rules/validate` | Implemented |
| `/cms/ops/audit` | ops audit | `GET /api/v1/platform/cms/ops/audit` | Implemented |
| `/cms/ops/sessions` | session ops | `GET /api/v1/platform/cms/ops/sessions`, `POST /api/v1/platform/cms/ops/sessions/:session_id/revoke` | Implemented |
| `/cms/ops/health` | health view | `GET /api/v1/platform/cms/ops/health`, `GET /api/v1/platform/cms/ops/metrics` | Implemented |
| `/cms/settings/general` | general settings | FE route/spec exists; matching backend settings endpoints are not currently visible in `platformcms` handler list | Partial |
| `/cms/settings/holiday-calendar` | holiday calendar | `GET /api/v1/platform/cms/holiday-calendars/{year}`, `POST /preview`, `PUT /api/v1/platform/cms/holiday-calendars/{year}` | Implemented |
| `/cms/reference/listed-companies` | listed companies reference list (vnstock, read-only) | `GET /api/v1/platform/cms/market/listed-companies` | Implemented |
| `/cms/reference/listed-companies/:symbol` | listed company detail (vnstock profile / partial) | `GET /api/v1/platform/cms/market/listed-companies/{symbol}` | Implemented |
| `/cms/settings/localization` | localization settings | FE route/spec exists; backend endpoint not currently visible | Partial |
| `/cms/settings/integrations` | integration settings | FE route/spec exists; backend endpoint not currently visible | Partial |

## Internal And Service-Only Surfaces

| Surface | BE endpoint(s) | Status |
|---|---|---|
| internal authorization | `POST /internal/v1/authorize`, `POST /internal/v1/authorize/batch` | Implemented |
| worker/outbox | no FE route; `cmd/worker` + outbox/reminder modules | Implemented |
