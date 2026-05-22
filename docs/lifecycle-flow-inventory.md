# Lifecycle Flow Inventory

> Updated: 2026-05-22
> Scope: end-to-end project behavior grouped by operational lifecycle
> Focus lifecycles: `auth`, `disclosure`, `reminder`, `ops`

## Purpose

This document answers a different question than route or module inventories:
instead of "which screen calls which handler", it asks "what is the end-to-end lifecycle of the domain behavior".

## Lifecycle Map

| Lifecycle | Start point | Main backend modules | Main FE surfaces |
|---|---|---|---|
| Auth | public login/register/invite/reset entry points | `iam`, `authorization`, `audit` | `/login`, `/register`, `/accept-invitation`, `/reset-password`, company selection, portal shell |
| Disclosure | disclosure type selection or record creation | `disclosure`, `workflow`, `companyaccess`, `authorization` | `/app/disclosures/*`, `/app/deadlines`, `/app/history`, `/cms/templates` |
| Reminder | workflow/deadline config or pending due work | `disclosure`, `workflow`, `reminder`, `notification`, worker/outbox | portal deadline/history shells, workflow previews, async email delivery |
| Ops | admin action, audit event, runtime health change | `platformcms`, `companyaccess`, `iam`, `audit` | `/app/admin/audit`, `/cms/ops/*`, `/cms/admin/*` |

## Auth Lifecycle

### 1. Entry and credential submission

- FE entry points:
  - `/login`
  - `/register`
  - `/accept-invitation`
  - `/reset-password`
- BE entry points:
  - `GET /api/v1/auth/login-password-key`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/register`
  - `GET /api/v1/auth/invitations/validate`
  - `POST /api/v1/auth/invitations/accept`
  - `POST /api/v1/auth/forgot-password`
  - `POST /api/v1/auth/reset-password`

### 2. Identity establishment

- login can return either:
  - a company-bound access token
  - a `pre_company_token` that still requires tenant selection
- invitation and reset flows end by restoring an account into a valid login-capable state

### 3. Company context binding

- FE surface:
  - company selection flow
  - `/app/no-company`
- BE endpoints:
  - `POST /api/v1/auth/select-company`
  - `POST /api/v1/auth/switch-company`
  - `POST /api/v1/me/active-company`

### 4. User bootstrap and permission hydration

- FE bootstrap:
  - portal shell
  - route guards
  - navigation/landing decision
- BE endpoints:
  - `GET /api/v1/me`
  - `GET /api/v1/me/profile`
  - `GET /api/v1/me/companies`
  - `GET /api/v1/me/authorized-companies`
  - `GET /api/v1/me/effective-access`
  - `GET /api/v1/me/capabilities`
  - `GET /api/v1/me/membership`

### 5. Ongoing session lifecycle

- refresh/logout:
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
- session operations:
  - `GET /api/v1/sessions`
  - `POST /api/v1/sessions/{session_id}/revoke`
- verification loop:
  - `POST /api/v1/auth/resend-verification-email`
  - `POST /api/v1/auth/verify-email`

### Current status notes

- Auth is one of the clearest FE->BE lifecycles in the workspace.
- The main branching complexity is multi-company login and current-company rebinding after authentication.

## Disclosure Lifecycle

### 1. Type discovery and template setup

- user-facing type discovery:
  - `GET /api/v1/disclosure-groups`
  - `GET /api/v1/disclosure-types/display-groups`
  - `GET /api/v1/disclosure-types`
  - `GET /api/v1/disclosure-types/{type_id}`
- platform/admin template setup:
  - `GET /api/v1/admin/disclosure-types/reference-data`
  - `PUT /api/v1/admin/disclosure-types/{type_id}`
  - `GET /api/v1/admin/disclosure-types/{type_id}/versions`
  - `GET /api/v1/admin/disclosure-types/{type_id}/versions/{version_no}`
  - `POST /api/v1/admin/disclosure-types/{type_id}/activate`
  - `GET /api/v1/admin/disclosure-types/{type_id}/config`
  - `PUT /api/v1/admin/disclosure-types/{type_id}/config`

### 2. Company-specific workflow adaptation

- endpoints:
  - `GET /api/v1/company/disclosure-types/{type_id}/workflow-override`
  - `PUT /api/v1/company/disclosure-types/{type_id}/workflow-override/draft`
  - `POST /api/v1/company/disclosure-types/{type_id}/workflow-override/approve`
  - `DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/{version_no}`
  - `DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/active`
  - `GET /api/v1/company/disclosure-types/{type_id}/workflow-override/versions`
  - `GET /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/reminder-preview`
  - `PUT /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/steps/{step_id}/groups`
  - `GET /api/v1/disclosure-types/{type_id}/effective-workflow`
  - `GET /api/v1/company/disclosure-types/{type_id}/preferences`
  - `PATCH /api/v1/company/disclosure-types/{type_id}/preferences`

### 3. Record creation and editing

- FE surfaces:
  - `/app/disclosures`
  - `/app/disclosures/new`
  - `/app/disclosures/:recordId`
- BE endpoints:
  - `POST /api/v1/disclosures`
  - `GET /api/v1/disclosures`
  - `GET /api/v1/disclosures/{record_id}`
  - `PATCH /api/v1/disclosures/{record_id}`

### 4. Submission and workflow execution

- record-level actions:
  - `POST /api/v1/disclosures/{record_id}/submit`
  - `POST /api/v1/disclosures/{record_id}/confirm`
- workflow-level actions:
  - `POST /api/v1/workflows/instances`
  - `GET /api/v1/workflows/instances/{instance_id}`
  - `GET /api/v1/workflows/instances/{instance_id}/tasks`
  - `POST /api/v1/workflows/tasks/{task_id}/review`
  - `POST /api/v1/workflows/tasks/{task_id}/approve`
  - `POST /api/v1/workflows/tasks/{task_id}/confirm`
  - `POST /api/v1/workflows/tasks/{task_id}/reject`
  - `POST /api/v1/workflows/tasks/{task_id}/actions/{action}`

### 5. Completion, history, and dashboard exposure

- related FE surfaces:
  - `/app/dashboard`
  - `/app/deadlines`
  - `/app/history`
- expected outputs:
  - current status of records
  - pending tasks/reminders
  - completed/confirmed history

### Current status notes

- Disclosure is the deepest business lifecycle in the workspace and spans both portal and CMS surfaces.
- Template/version management and company workflow override are already modeled as first-class lifecycle stages, not just admin side panels.

## Reminder Lifecycle

### 1. Reminder source definition

- reminder generation depends on:
  - disclosure type deadline config
  - company type preferences
  - company workflow override draft/active state
  - workflow instance state
- visible setup endpoints:
  - `GET /api/v1/admin/disclosure-types/{type_id}/config`
  - `PUT /api/v1/admin/disclosure-types/{type_id}/config`
  - `GET /api/v1/company/disclosure-types/{type_id}/preferences`
  - `PATCH /api/v1/company/disclosure-types/{type_id}/preferences`
  - `GET /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/reminder-preview`

### 2. Assignee and recipient resolution

- endpoints and support:
  - `POST /api/v1/workflows/resolve-assignees`
  - company groups and membership data from admin/disclosure domains
  - notification rules under company admin

### 3. Reminder materialization on workflow instances

- workflow endpoints:
  - `GET /api/v1/workflows/instances/{instance_id}/reminders`
  - `GET /api/v1/workflows/instances/{instance_id}/tasks`
- domain expectation:
  - reminders are attached to workflow steps and due-state evaluation

### 4. Dispatch pipeline

- backend runtime participants:
  - outbox write path
  - worker process
  - notification/reminder infrastructure
- system flow:
  1. business action creates reminder-worthy event
  2. event is persisted/outboxed
  3. worker picks up dispatch job
  4. notification channel sends or retries

### 5. Delivery and operational follow-up

- current visible product surfaces:
  - deadline/history shells
  - alert-channel settings shell
  - admin notification rules
- likely operational states:
  - pending
  - queued/dispatching
  - sent
  - failed/retry

### Current status notes

- Reminder lifecycle is architecturally present, but the FE route-to-handler trace is more distributed than auth or disclosure.
- Email is the clearest implemented channel; broader multi-channel behavior remains more spec-driven.

## Ops Lifecycle

### 1. Administrative action triggers

- tenant-level actions:
  - enterprise admin users/memberships/roles/rules/company profile
- platform-level actions:
  - CMS companies/users/rules/content/reviews/schedules

### 2. Authorization and audit capture

- supporting modules:
  - `authorization`
  - `audit`
- visible FE/BE ops surfaces:
  - `/app/admin/audit`
  - `/cms/ops/audit`
  - `/cms/ops/sessions`
  - `GET /api/v1/platform/cms/ops/audit`
  - `GET /api/v1/platform/cms/ops/sessions`
  - `POST /api/v1/platform/cms/ops/sessions/{session_id}/revoke`
  - `GET /api/v1/sessions`
  - `POST /api/v1/sessions/{session_id}/revoke`

### 3. Runtime health observation

- platform ops endpoints:
  - `GET /api/v1/platform/cms/ops/health`
  - `GET /api/v1/platform/cms/ops/metrics`
- supporting runtime pieces:
  - API container
  - worker container
  - MySQL
  - Redis
  - web proxy

### 4. Calendar and operational configuration

- FE surface:
  - `/cms/settings/holiday-calendar`
- BE endpoints:
  - `GET /api/v1/platform/cms/holiday-calendars/{year}`
  - `POST /api/v1/platform/cms/holiday-calendars/{year}/preview`
  - `PUT /api/v1/platform/cms/holiday-calendars/{year}`

### 5. Incident handling and recovery

- practical activities seen in current workspace:
  - migration bootstrap and tracking
  - deploy artifact validation
  - session revoke and health verification
  - environment/runtime inspection via logs and health checks

### Current status notes

- Ops is split between user-facing admin operations and infrastructure/runtime stabilization work.
- CMS ops is the clearest consolidated runtime surface; tenant admin audit/session behavior exists but is more distributed across modules.
