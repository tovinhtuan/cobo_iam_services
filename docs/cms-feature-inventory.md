# CMS Feature Inventory

> Updated: 2026-05-22
> Scope: platform/web admin surface under `/cms`

## Access Model

- FE route prefix: `/cms`
- Primary guard: `platform.cms.view`
- Purpose:
  - platform-wide administration
  - disclosure/content management
  - operational oversight
  - company and platform user administration

## CMS Route Inventory

| Route | Goal | Main backend endpoints |
|---|---|---|
| `/cms` | dashboard summary | `GET /api/v1/platform/cms/dashboard/summary` |
| `/cms/content/collections` | list collections | `GET /api/v1/platform/cms/collections` |
| `/cms/content/collections/:collectionId` | collection detail | `GET /api/v1/platform/cms/collections/:collectionId` |
| `/cms/content/entries` | list entries | `GET /api/v1/platform/cms/entries` |
| `/cms/content/entries/:entryId` | create/edit entry | `GET /api/v1/platform/cms/entries/:entryId`, `POST /api/v1/platform/cms/entries`, `PUT /api/v1/platform/cms/entries/:entryId` |
| `/cms/content/media` | upload/manage media | `POST /api/v1/platform/cms/media/upload`, `PUT /api/v1/platform/cms/media/upload/:asset_id`, `POST /api/v1/platform/cms/media/:asset_id/complete`, `GET /api/v1/platform/cms/media`, `DELETE /api/v1/platform/cms/media/:asset_id` |
| `/cms/publishing/review` | approve/reject content | `GET /api/v1/platform/cms/reviews`, `POST /api/v1/platform/cms/reviews/:entry_id` |
| `/cms/publishing/schedule` | schedule publication | `GET /api/v1/platform/cms/schedules`, `POST /api/v1/platform/cms/schedules`, `DELETE /api/v1/platform/cms/schedules/:entry_id` |
| `/cms/publishing/releases` | release history | `GET /api/v1/platform/cms/releases` |
| `/cms/taxonomy` | taxonomy management | FE route/spec present; BE route currently not confirmed in handler registration |
| `/cms/templates` | disclosure type template management | `GET /api/v1/disclosure-types`, `GET /api/v1/disclosure-types/:typeId`, `PUT /api/v1/admin/disclosure-types/:typeId`, `GET /api/v1/admin/disclosure-types/:typeId/versions` |
| `/cms/admin/companies` | companies list/create | `GET /api/v1/platform/cms/admin/companies`, `POST /api/v1/platform/cms/admin/companies` |
| `/cms/admin/companies/:companyId` | company detail/activation/status update | `GET /api/v1/platform/cms/admin/companies/:companyId`, `PATCH /api/v1/platform/cms/admin/companies/:companyId`, `POST /deactivate`, `POST /activate` |
| `/cms/admin/users` | platform users and memberships | `GET /api/v1/platform/cms/admin/users`, `POST /api/v1/platform/cms/admin/users`, `POST /invite`, `POST /resend-invitation`, `POST /assign-company`, `POST /companies/:company_id/members`, `POST /request-password-reset` |
| `/cms/admin/roles` | roles and permissions | `GET /api/v1/platform/cms/admin/roles` |
| `/cms/admin/rules` | rule validation/publish | `GET /api/v1/platform/cms/admin/rules`, `POST /api/v1/platform/cms/admin/rules/validate` |
| `/cms/ops/audit` | audit log operations view | `GET /api/v1/platform/cms/ops/audit` |
| `/cms/ops/sessions` | active sessions/revoke | `GET /api/v1/platform/cms/ops/sessions`, `POST /api/v1/platform/cms/ops/sessions/:session_id/revoke` |
| `/cms/ops/health` | health and metrics | `GET /api/v1/platform/cms/ops/health`, `GET /api/v1/platform/cms/ops/metrics` |
| `/cms/settings/general` | general platform settings | FE route/spec present; BE endpoint not confirmed in handler registration |
| `/cms/settings/holiday-calendar` | holiday calendar management | `GET /api/v1/platform/cms/holiday-calendars/{year}`, `POST /preview`, `PUT /api/v1/platform/cms/holiday-calendars/{year}` |
| `/cms/settings/localization` | localization settings | FE route/spec present; BE endpoint not confirmed |
| `/cms/settings/integrations` | integration settings | FE route/spec present; BE endpoint not confirmed |

## CMS Functional Groups

### Content management

- collections
- entries
- media assets
- reviews
- schedules
- releases

### Disclosure template administration

- list disclosure types
- inspect type detail
- update disclosure type config
- inspect version history
- edit advanced template blocks/workflow-oriented structures from FE CMS screens

### Platform administration

- create/manage companies
- activate/deactivate companies
- update company status and verification metadata
- create/invite platform-side users
- assign users into companies
- request password reset for managed users

### Governance and operations

- audit logs
- active sessions
- revoke sessions
- system health
- metrics
- holiday calendar maintenance

## Current CMS Status Notes

- CMS is one of the most fully traceable FE->BE surfaces in the workspace.
- Taxonomy and several settings screens are clearly designed in FE/specs, but their backend route presence is not currently confirmed in the platform CMS handler registration.
- CMS templates intentionally bridge CMS FE screens with a mix of platform and admin disclosure-type endpoints.
