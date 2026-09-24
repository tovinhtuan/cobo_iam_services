# CMS Feature Inventory

> Updated: 2026-09-24
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
| `/cms/content/entries` | list entries (**nav hidden** unless `VITE_CMS_LEGACY_ENTRIES_NAV=true`) | `GET /api/v1/platform/cms/entries` |
| `/cms/content/entries/:entryId` | create/edit entry | `GET/POST/PUT /api/v1/platform/cms/entries…` |
| `/cms/records/:recordId` | **Global CMS Record** detail (direct publish / archive / materialize) | `GET/PUT /api/v1/platform/cms/records/:id`, `POST …/publish`, `…/archive`, `…/materialize`, `GET …/company-records` |
| `/cms/content/media` | upload/manage media | `POST /api/v1/platform/cms/media/upload`, … |
| `/cms/publishing/review` | **LEGACY deep-link** — Company Workflow Queue; **not in CMS sidebar** | `GET/POST /api/v1/platform/cms/reviews…` |
| `/cms/publishing/schedule` | **LEGACY deep-link** — company `planned_date`; **not in CMS sidebar** | `GET/POST/DELETE /api/v1/platform/cms/schedules…` |
| `/cms/publishing/releases` | **LEGACY deep-link** — company Published/Completed/Confirmed; **not in CMS sidebar** | `GET /api/v1/platform/cms/releases` |
| `/cms/taxonomy` | taxonomy management | FE route/spec present; BE not confirmed in handler registration |
| `/cms/templates` | disclosure templates + **Lịch sử bản ghi CMS** | disclosure-types APIs + `GET/POST /api/v1/platform/cms/templates/:id/records` |
| `/cms/admin/companies` | companies list/create | platform CMS admin companies APIs |
| `/cms/admin/companies/:companyId` | company detail/activation | platform CMS admin company APIs |
| `/cms/admin/users` | platform users and memberships | platform CMS admin users APIs |
| `/cms/admin/roles` | roles and permissions | `GET /api/v1/platform/cms/admin/roles` |
| `/cms/admin/rules` | rule validation/publish | rules validate APIs |
| `/cms/ops/audit` | audit log operations view | `GET /api/v1/platform/cms/ops/audit` |
| `/cms/ops/sessions` | active sessions/revoke | sessions APIs |
| `/cms/ops/health` | health and metrics | health/metrics APIs |
| `/cms/settings/general` | general platform settings | FE present; BE not confirmed |
| `/cms/settings/holiday-calendar` | holiday calendar management | holiday-calendars APIs |
| `/cms/settings/localization` | localization settings | FE present; BE not confirmed |
| `/cms/settings/integrations` | integration settings | FE present; BE not confirmed |

## Navigation policy (2026-09-24)

| Thành phần | Quyết định |
|---|---|
| Sidebar group **“Xuất bản”** | **Ẩn mặc định** (`VITE_CMS_LEGACY_PUBLISHING_NAV` unset/false) |
| Global CMS publish | Template → Lịch sử bản ghi CMS / `/cms/records/:id` → **Phát hành** |
| Company Workflow Queue | Company-scoped PendingReview; Portal / legacy deep-link `/cms/publishing/review` (deprecation banner) |
| Schedule / Releases routes | Keep deep-link + deprecation banner; **not** Global CMS scheduling/history |
| Backend `reviews` / `schedules` / `releases` APIs | **Kept** (no delete this phase) |

Opt-in: `VITE_CMS_LEGACY_PUBLISHING_NAV=true` restores the Publishing sidebar group for DEV regression only.

## CMS Functional Groups

### Content management

- collections
- entries (legacy nav optional)
- media assets
- Global CMS Records (template history + detail)

### Legacy publishing (APIs / deep-links only — not default CMS nav)

- reviews (company queue)
- schedules (company planned_date)
- releases (company publish history)

### Disclosure template administration

- list/update disclosure types and versions
- Global CMS Record history tab

### Platform administration

- companies, users, memberships

### Governance and operations

- audit, sessions, health, holiday calendar

## Current CMS Status Notes

- CMS is one of the most fully traceable FE->BE surfaces in the workspace.
- Taxonomy and several settings screens are designed in FE/specs; backend presence not always confirmed.
- **2026-09-24:** CMS sidebar “Xuất bản” removed by default after Global CMS Record direct-publish; smoke: `docs/ai-cache/cms-publishing-nav-removal-smoke-qa-2026-09-24/`.
