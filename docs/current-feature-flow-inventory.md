# Current Feature And Flow Inventory

> Updated: 2026-05-22
> Scope: `cobo_iam_services` + sibling FE repo `cobo_web_design`
> Basis: current code, route wiring, transport handlers, FE service clients, and product/spec documents

## Purpose

This document is a single inventory of the features and user/system flows that are currently designed in the project workspace.
It is intended for onboarding, scope review, and cross-repo tracing between FE screens and BE modules/endpoints.

## Sources Reviewed

- BE:
  - `README.md`
  - `internal/httpserver/server.go`
  - `internal/*/transport/http/*.go`
  - migration set under `migrations/`
- FE:
  - `src/App.tsx`
  - `src/services/**`
  - `src/features/admin-core/**`
  - `src/features/cms-core/**`
  - `SPEC.md`
  - `SPEC-account-management.md`
- Cross-repo summaries:
  - `docs/ai-cache/api-flow-sequence-summary.md`
  - `cobo_web_design/docs/ai-cache/api-endpoints-by-screen.md`
  - `cobo_web_design/docs/ai-cache/business-contract-and-tech-stack.md`

## Reading Guide

- FE route means a user-visible screen or shell route exists in the frontend.
- BE endpoint means the HTTP handler is currently registered in backend transport.
- "Designed in project" here means either wired in code or explicitly described by the active spec/docs and mirrored in service/route structure.

## 1. System Areas

| Area | FE Surface | BE Surface | Main Goal |
|---|---|---|---|
| Public marketing | public pages | static only | landing, docs, pricing, legal pages |
| IAM & account lifecycle | login/register/reset/invite/select company | `internal/iam` | authenticate user and establish tenant context |
| Portal shell & profile | `/app/*` | `me` + session APIs | bootstrap current user, company, permissions |
| Disclosure operations | portal disclosure screens | `internal/disclosure` | create/manage disclosure records |
| Disclosure type catalog | portal + CMS templates | `internal/disclosure`, `internal/platformcms` | manage disclosure type definitions and workflow templates |
| Workflow | record actions + task flows | `internal/workflow` | review/approve/confirm/reject workflow steps |
| Deadline / reminder / notification | deadlines, alert channels | `internal/reminder`, `internal/notification` | generate and dispatch deadline reminders |
| Ad-hoc alert | dedicated portal flow | `internal/adhoc` | submit/approve non-template alert proposals |
| Enterprise admin | `/app/admin/*` | `internal/companyaccess` | manage company users, org structure, RBAC, rules |
| Platform CMS admin | `/cms/*` | `internal/platformcms` | manage platform content, companies, users, roles, operations |
| Authorization | FE permission guards | `internal/authorization` | tenant boundary, permission, scope and responsibility checks |
| Audit & ops | audit/session screens | `internal/audit`, session handlers | trace privileged actions and manage sessions |
| Worker/outbox | no direct FE | `cmd/worker`, outbox/reminder infra | async email/reminder/event processing |

## 2. Public, Auth And Account Lifecycle Flows

### 2.1 Public pages

- FE routes:
  - `/`
  - `/terms`
  - `/privacy`
  - `/pricing`
  - `/docs`
- Purpose:
  - marketing/legal/static entry points
- BE:
  - no dedicated backend route required for these pages

### 2.2 Login

- FE route:
  - `/login`
- BE endpoints:
  - `GET /api/v1/auth/login-password-key`
  - `POST /api/v1/auth/login`
- Flow:
  1. user submits credentials
  2. backend authenticates and returns either direct `access_token` or `pre_company_token`
  3. FE routes user to CMS, portal dashboard, company selection, or no-company state
- Notes:
  - supports multi-company login branching
  - login password transport encryption is designed via RSA public key endpoint

### 2.3 Public registration

- FE route:
  - `/register`
- BE endpoint:
  - `POST /api/v1/auth/register`
- Flow:
  1. public user self-registers
  2. account is created
  3. company initialization / onboarding path is available via next actions and no-company handling

### 2.4 Invitation acceptance

- FE route:
  - `/accept-invitation`
- BE endpoints:
  - `GET /api/v1/auth/invitations/validate`
  - `POST /api/v1/auth/invitations/accept`
- Flow:
  1. user opens invitation token link
  2. FE validates token
  3. user accepts invitation and sets account credentials/context

### 2.5 Password reset

- FE route:
  - `/reset-password`
- BE endpoints:
  - `POST /api/v1/auth/forgot-password`
  - `POST /api/v1/auth/reset-password`
- Flow:
  1. user requests reset
  2. backend emits reset intent via outbox/email flow
  3. user opens tokenized reset page and sets new password

### 2.6 Email verification

- FE route:
  - `/app/verify-email`
- BE endpoints:
  - `POST /api/v1/auth/resend-verification-email`
  - `POST /api/v1/auth/verify-email`
- Flow:
  1. user requests/resends verification
  2. user submits OTP or verification payload
  3. profile/session state is updated

### 2.7 Session maintenance

- FE behavior:
  - token persistence in local storage
  - bootstrap and refresh via auth/me services
- BE endpoints:
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/sessions`
  - `POST /api/v1/sessions/{session_id}/revoke`

## 3. Company Context And Portal Shell Flows

### 3.1 Company selection after login

- FE route:
  - company selection page in auth flow
- BE endpoints:
  - `POST /api/v1/auth/select-company`
  - `POST /api/v1/auth/switch-company`
  - `POST /api/v1/me/active-company`
- Flow:
  1. user with multiple memberships receives pre-company context
  2. FE lists authorized companies
  3. user selects/switches active company
  4. backend issues company-bound access token

### 3.2 Current user bootstrap

- FE shell:
  - `PortalLayout`
  - auth bootstrap in `src/App.tsx`
- BE endpoints:
  - `GET /api/v1/me`
  - `GET /api/v1/me/profile`
  - `GET /api/v1/me/companies`
  - `GET /api/v1/me/authorized-companies`
  - `GET /api/v1/me/effective-access`
  - `GET /api/v1/me/capabilities`
  - `GET /api/v1/me/membership`
- Flow:
  1. FE loads current user
  2. FE loads company list and effective access
  3. FE chooses landing route and guards route visibility

### 3.3 No-company onboarding

- FE route:
  - `/app/no-company`
- Purpose:
  - explicit portal state for users without any company membership
- Related BE:
  - login/register/select-company outputs drive this route

### 3.4 Profile/settings

- FE routes:
  - `/app/profile`
  - `/app/settings`
- BE surface:
  - mainly `GET /api/v1/me` and surrounding IAM/session/account APIs
- Purpose:
  - current-user profile display and account-level settings shell

## 4. Portal Business Flows

### 4.1 Dashboard

- FE route:
  - `/app/dashboard`
- Designed data flow:
  - user summary cards
  - upcoming deadlines
  - recent disclosures / work items
- BE support:
  - current code/specs imply dashboard summaries, deadline lists, disclosure summaries
  - company context and effective access gate the visible content

### 4.2 Recipients

- FE route:
  - `/app/company/recipients`
- Domain goal:
  - manage disclosure/reminder recipients
- BE support:
  - recipient management is part of company/disclosure operational scope in current product design

### 4.3 Deadlines

- FE routes:
  - `/app/deadlines`
  - `/app/deadlines/:id`
  - `/app/history`
  - `/app/history/:id`
- Domain goal:
  - list active deadlines
  - inspect deadline detail
  - review historical disclosure/deadline execution
- BE modules involved:
  - reminder runtime
  - disclosure lifecycle
  - notification/reminder status

### 4.4 Disclosure type list and detail

- FE routes:
  - `/app/disclosure-types`
  - `/app/disclosure-types/:id`
- FE service surfaces:
  - disclosure type list/detail
  - workflow override draft/approve/active/history
  - config and reminder preview support
- BE module:
  - `internal/disclosure`
- Main designed functions:
  - list system/custom disclosure types
  - inspect type definition
  - manage company workflow override over disclosure type
  - read type versions/config

### 4.5 Disclosure record create/edit/view

- FE routes:
  - `/app/disclosures/new`
  - `/app/disclosures/:id`
  - `/app/disclosures/:id/edit`
- BE endpoints are registered under disclosure handlers and service clients for:
  - disclosure list
  - disclosure detail
  - create draft
  - update draft
  - submit
  - confirm
  - publish/completion-oriented transitions
- Main designed flow:
  1. choose disclosure type
  2. create or edit draft record
  3. assign workflow/reminders/evidence
  4. submit for workflow
  5. confirm/publish/complete according to workflow/status rules

### 4.6 Workflow task actions

- BE endpoints:
  - `POST /api/v1/workflows/instances`
  - `GET /api/v1/workflows/instances/{instance_id}`
  - `GET /api/v1/workflows/instances/{instance_id}/tasks`
  - `GET /api/v1/workflows/instances/{instance_id}/reminders`
  - `POST /api/v1/workflows/tasks/{task_id}/review`
  - `POST /api/v1/workflows/tasks/{task_id}/approve`
  - `POST /api/v1/workflows/tasks/{task_id}/confirm`
  - `POST /api/v1/workflows/tasks/{task_id}/reject`
  - `POST /api/v1/workflows/tasks/{task_id}/actions/{action}`
  - `POST /api/v1/workflows/resolve-assignees`
- FE relation:
  - disclosure/detail/template/admin screens consume workflow contracts
- Main designed functions:
  - create workflow instances
  - resolve assignees from RBAC/org model
  - execute named workflow actions
  - inspect tasks and reminders per workflow instance

### 4.7 Alert channels and notification settings

- FE route:
  - `/app/alert-channels`
- Designed goal:
  - manage channels and reminder/notification rules
- BE modules:
  - `internal/notification`
  - reminder/dispatch infra
- Registered notification endpoints:
  - `POST /api/v1/notifications/resolve-recipients`
  - `POST /api/v1/notifications/enqueue`
  - `POST /api/v1/notifications/dispatch`

### 4.8 Ad-hoc proposal flow

- FE routes:
  - `/app/ad-hoc-proposals`
  - `/app/ad-hoc-proposals/new`
  - `/app/ad-hoc-proposals/:proposalId`
- BE endpoints:
  - `GET /api/v1/company/ad-hoc-proposals/eligible-controllers`
  - `POST /api/v1/company/ad-hoc-proposals`
  - `GET /api/v1/company/ad-hoc-proposals`
  - `GET /api/v1/company/ad-hoc-proposals/{proposal_id}`
  - `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/submit`
  - `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/focal-approve`
  - `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/admin-approve`
  - `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/reject`
  - `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/cancel`
- Main designed flow:
  1. create proposal
  2. submit to controller/reviewer chain
  3. focal review
  4. admin review/final approval
  5. reject/cancel when needed

## 5. Enterprise Admin Flows

### 5.1 Admin shell

- FE routes:
  - `/app/admin`
  - `/app/admin/hub`
- FE services:
  - admin hub summary
- BE:
  - `/api/v1/admin/hub/summary`
- Goal:
  - tenant admin landing, quick stats and navigation

### 5.2 Users and memberships

- FE route:
  - `/app/admin/users`
- FE services:
  - `membershipAdminApi`
- Designed functions:
  - list company memberships/users
  - create user/member
  - invite user
  - activate/deactivate membership
  - inspect role/department/title assignments
- BE module:
  - `internal/companyaccess`

### 5.3 Roles and permissions

- FE route:
  - `/app/admin/roles`
- Designed functions:
  - role matrix
  - permission assignment
  - membership role assignment visibility
- BE module:
  - `internal/companyaccess`
  - `internal/authorization`

### 5.4 Rules builder

- FE route:
  - `/app/admin/rules`
- Designed functions:
  - manage authorization/resource scope/workflow/notification rules
  - validate and persist rules
- BE module:
  - `internal/companyaccess`

### 5.5 Audit and sessions

- FE route:
  - `/app/admin/audit`
- Designed functions:
  - read audit logs
  - inspect/revoke sessions
- Related BE surfaces:
  - audit repository/services
  - IAM session list/revoke APIs

### 5.6 Company profile

- FE route:
  - `/app/admin/company`
- FE service:
  - `companyProfileApi`
- Designed functions:
  - view and update own company profile
  - manage company metadata under tenant scope
- BE tests/routes indicate:
  - `GET /api/v1/admin/company`
  - `PATCH /api/v1/admin/company`

### 5.7 Departments

- FE route:
  - `/app/admin/departments`
- FE service:
  - `departmentApi`
- Designed functions:
  - list/create/update/delete departments
  - add/remove department members
- BE module:
  - `internal/companyaccess`

### 5.8 Titles

- FE route:
  - `/app/admin/titles`
- FE service:
  - `titleApi`
- Designed functions:
  - list/create/update/delete titles
  - assign/remove titles on memberships
- BE module:
  - `internal/companyaccess`

## 6. Platform CMS / Web Admin Flows

### 6.1 CMS shell and access gate

- FE route prefix:
  - `/cms`
- Access rule:
  - strict `platform.cms.view`
- Main purpose:
  - platform-wide admin and content management separated from tenant admin

### 6.2 CMS route inventory

The following CMS screens are explicitly wired in FE and mapped to BE endpoints:

| FE route | Purpose |
|---|---|
| `/cms` | dashboard summary |
| `/cms/content/collections` | collections list |
| `/cms/content/collections/:collectionId` | collection detail |
| `/cms/content/entries` | entries list |
| `/cms/content/entries/:entryId` | entry editor |
| `/cms/content/media` | media library |
| `/cms/publishing/review` | review queue |
| `/cms/publishing/schedule` | schedule manager |
| `/cms/publishing/releases` | release history |
| `/cms/taxonomy` | taxonomy management |
| `/cms/templates` | disclosure template management |
| `/cms/admin/companies` | companies list |
| `/cms/admin/companies/:companyId` | company detail |
| `/cms/admin/users` | platform users and memberships |
| `/cms/admin/roles` | role matrix |
| `/cms/admin/rules` | rules validation/publish |
| `/cms/ops/audit` | platform audit |
| `/cms/ops/sessions` | active sessions |
| `/cms/ops/health` | health overview |
| `/cms/settings/general` | general settings |
| `/cms/settings/holiday-calendar` | holiday calendar |
| `/cms/settings/localization` | localization settings |
| `/cms/settings/integrations` | integration settings |

### 6.3 CMS backend endpoint groups

- Dashboard/content/media/review/schedule/releases:
  - `/api/v1/platform/cms/dashboard/summary`
  - `/api/v1/platform/cms/collections...`
  - `/api/v1/platform/cms/entries...`
  - `/api/v1/platform/cms/media...`
  - `/api/v1/platform/cms/reviews...`
  - `/api/v1/platform/cms/schedules...`
  - `/api/v1/platform/cms/releases...`
- Platform admin:
  - `/api/v1/platform/cms/admin/companies...`
  - `/api/v1/platform/cms/admin/users...`
  - `/api/v1/platform/cms/admin/roles`
  - `/api/v1/platform/cms/admin/rules...`
- Operations/settings:
  - `/api/v1/platform/cms/ops/audit`
  - `/api/v1/platform/cms/ops/sessions...`
  - `/api/v1/platform/cms/ops/health`
  - `/api/v1/platform/cms/ops/metrics`
  - holiday calendar endpoints under `/api/v1/platform/cms/holiday-calendars/{year}`

## 7. Authorization, Audit, Async And Internal Flows

### 7.1 Internal authorization APIs

- BE endpoints:
  - `POST /internal/v1/authorize`
  - `POST /internal/v1/authorize/batch`
- Purpose:
  - service-to-service or internal authorization decision checks

### 7.2 Effective access projection and cache

- BE modules:
  - `internal/authorization`
  - projection store with Redis or in-memory fallback
- Purpose:
  - resolve effective permissions/departments/responsibilities per membership/company
  - cache authorization snapshots

### 7.3 Audit logging

- BE module:
  - `internal/audit`
- Designed behavior:
  - sensitive auth/admin/CMS operations generate audit events
  - audit data is queryable from CMS/enterprise admin ops surfaces

### 7.4 Outbox and worker

- Processes:
  - `cmd/api`
  - `cmd/worker`
- Designed flow:
  1. API writes outbox event in transaction-aware path when available
  2. worker polls outbox
  3. worker dispatches reminder/notification side effects

### 7.5 Reminder dispatch lifecycle

- BE modules:
  - `internal/reminder`
  - `internal/notification`
- Designed states:
  - pending
  - dispatching
  - sent
  - retry scheduled
  - failed

## 8. Current Design Boundaries And Status Notes

- Multi-tenant company context is a first-class design constraint across all portal/admin flows.
- CMS admin and enterprise admin are separate surfaces with different gates and responsibilities.
- Disclosure/workflow/reminder/notification are designed as interconnected modules, not isolated screens.
- A number of screens and contracts are fully wired in FE/BE routes but may still evolve in behavior as migrations, worker runtime, and backend detail endpoints mature.
- The inventory above is intentionally scoped to features/flows that are visible in current route wiring, handlers, service clients, specs, or active migration-backed domain design.
