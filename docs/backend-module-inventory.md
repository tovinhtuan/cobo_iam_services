# Backend Module Inventory

> Updated: 2026-05-22
> Scope: backend-facing inventory split by primary business modules in `cobo_iam_services`
> Focus modules: `iam`, `companyaccess`, `disclosure`, `workflow`, `platformcms`

## Purpose

This document groups the current project design by backend module ownership instead of by screen.
Use it when tracing where a behavior lives in backend code, what FE surfaces consume it, and which endpoints define the contract.

## Module Map

| Module | Primary responsibility | Main FE surfaces |
|---|---|---|
| `internal/iam` | authentication, account lifecycle, session lifecycle, tenant selection, current-user bootstrap | public auth screens, company selection, portal shell, profile/settings |
| `internal/companyaccess` | company-scoped admin operations, memberships, org structure, RBAC, company profile | `/app/admin/*`, company onboarding/admin setup |
| `internal/disclosure` | disclosure records, type catalog, type versions, deadline config, company workflow overrides | `/app/disclosures/*`, `/app/history`, `/app/deadlines`, `/cms/templates` |
| `internal/workflow` | workflow instances, task actions, reminder lookup, assignee resolution | record review flows, disclosure approvals/confirms, reminder/task views |
| `internal/platformcms` | platform CMS content, platform admin, ops dashboards, holiday calendar | `/cms/*` |

## `internal/iam`

### Responsibility

- public authentication entry points
- account lifecycle and recovery
- company-bound token establishment
- current-user/session bootstrap for FE

### Main FE surfaces

- `/login`
- `/register`
- `/accept-invitation`
- `/reset-password`
- company selection flow after login
- `/app/verify-email`
- `/app/profile`
- `/app/settings`

### Main endpoints

- `GET /api/v1/auth/login-password-key`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/forgot-password`
- `POST /api/v1/auth/reset-password`
- `POST /api/v1/auth/resend-verification-email`
- `POST /api/v1/auth/verify-email`
- `GET /api/v1/auth/invitations/validate`
- `POST /api/v1/auth/invitations/accept`
- `POST /api/v1/auth/select-company`
- `POST /api/v1/auth/switch-company`
- `POST /api/v1/me/active-company`
- `GET /api/v1/me`
- `GET /api/v1/me/profile`
- `GET /api/v1/me/companies`
- `GET /api/v1/me/authorized-companies`
- `GET /api/v1/me/effective-access`
- `GET /api/v1/me/capabilities`
- `GET /api/v1/me/membership`
- `GET /api/v1/sessions`
- `POST /api/v1/sessions/{session_id}/revoke`

### Flow ownership

- login -> pre-company or company-bound token
- multi-company selection/switch
- reset-password and invitation acceptance
- email verification
- session listing and revoke
- portal bootstrap for current user and active company context

## `internal/companyaccess`

### Responsibility

- tenant-scoped admin center
- user and membership administration
- company RBAC, rules, departments, teams, titles
- company profile and ownership/admin operations

### Main FE surfaces

- `/app/admin`
- `/app/admin/hub`
- `/app/admin/users`
- `/app/admin/roles`
- `/app/admin/rules`
- `/app/admin/audit`
- `/app/admin/company`
- `/app/admin/departments`
- `/app/admin/titles`

### Main endpoints

- `GET /api/v1/admin/hub/summary`
- `GET /api/v1/admin/users`
- `POST /api/v1/admin/users`
- `POST /api/v1/admin/users/invite`
- `POST /api/v1/admin/users/{user_id}/resend-invitation`
- `POST /api/v1/admin/memberships`
- `PATCH /api/v1/admin/memberships/{membership_id}`
- `DELETE /api/v1/admin/memberships/{membership_id}`
- `GET /api/v1/admin/companies/{company_id}/memberships`
- `POST /api/v1/admin/memberships/{membership_id}/roles`
- `DELETE /api/v1/admin/memberships/{membership_id}/roles/{role_id}`
- `POST /api/v1/admin/memberships/{membership_id}/departments`
- `DELETE /api/v1/admin/memberships/{membership_id}/departments/{department_id}`
- `POST /api/v1/admin/memberships/{membership_id}/titles`
- `DELETE /api/v1/admin/memberships/{membership_id}/titles/{title_id}`
- `GET /api/v1/admin/memberships/{membership_id}/permissions`
- `POST /api/v1/admin/memberships/{membership_id}/permissions`
- `DELETE /api/v1/admin/memberships/{membership_id}/permissions/{permission_code}`
- `GET /api/v1/admin/permissions`
- `GET /api/v1/admin/roles`
- `POST /api/v1/admin/roles/{role_id}/permissions`
- `DELETE /api/v1/admin/roles/{role_id}/permissions/{permission_id}`
- `POST /api/v1/admin/resource-scope-rules`
- `POST /api/v1/admin/workflow-assignee-rules`
- `POST /api/v1/admin/notification-rules`
- `GET /api/v1/admin/notification-rules`
- `PATCH /api/v1/admin/notification-rules/{notification_rule_id}`
- `DELETE /api/v1/admin/notification-rules/{notification_rule_id}`
- `GET /api/v1/admin/account/settings`
- `PATCH /api/v1/admin/account/settings`
- `GET /api/v1/admin/company`
- `PATCH /api/v1/admin/company`
- `GET /api/v1/admin/invite-roles`
- `GET /api/v1/admin/departments`
- `POST /api/v1/admin/departments`
- `PATCH /api/v1/admin/departments/{department_id}`
- `DELETE /api/v1/admin/departments/{department_id}`
- `POST /api/v1/admin/departments/{department_id}/members`
- `DELETE /api/v1/admin/departments/{department_id}/members/{membership_id}`
- `GET /api/v1/admin/departments/{department_id}/teams`
- `POST /api/v1/admin/departments/{department_id}/teams`
- `PATCH /api/v1/admin/teams/{team_id}`
- `DELETE /api/v1/admin/teams/{team_id}`
- `POST /api/v1/admin/teams/{team_id}/members`
- `DELETE /api/v1/admin/teams/{team_id}/members/{membership_id}`
- `GET /api/v1/admin/titles`
- `POST /api/v1/admin/titles`
- `PATCH /api/v1/admin/titles/{title_id}`
- `DELETE /api/v1/admin/titles/{title_id}`
- `POST /api/v1/admin/titles/{title_id}/members`
- `DELETE /api/v1/admin/titles/{title_id}/members/{membership_id}`
- `POST /api/v1/admin/company/admins`
- `DELETE /api/v1/admin/company/admins/{membership_id}`
- `POST /api/v1/admin/company/transfer-ownership`
- `POST /api/v1/company/initialize`
- `POST /api/v1/company/create` (self-service Nth company; `COMPANY_SELF_CREATE_ENABLED`)

### Flow ownership

- company bootstrap/initialize after account creation
- enterprise admin hub and member management
- org-structure management: departments, teams, titles
- company-admin assignment and ownership transfer
- RBAC/rule editing inside a company boundary

## `internal/disclosure`

### Responsibility

- disclosure record CRUD and record state actions
- disclosure type catalog and type-group organization
- admin disclosure type versioning/config
- company workflow override and deadline preference handling

### Main FE surfaces

- `/app/dashboard`
- `/app/disclosures`
- `/app/disclosures/new`
- `/app/disclosures/:recordId`
- `/app/deadlines`
- `/app/history`
- `/cms/templates`

### Main endpoints

- `POST /api/v1/disclosures`
- `GET /api/v1/disclosures`
- `GET /api/v1/disclosures/{record_id}`
- `PATCH /api/v1/disclosures/{record_id}`
- `POST /api/v1/disclosures/{record_id}/submit`
- `POST /api/v1/disclosures/{record_id}/confirm`
- `GET /api/v1/disclosure-groups`
- `GET /api/v1/disclosure-types/display-groups`
- `GET /api/v1/disclosure-types`
- `GET /api/v1/disclosure-types/{type_id}`
- `GET /api/v1/admin/disclosure-types/reference-data`
- `PUT /api/v1/admin/disclosure-types/{type_id}`
- `GET /api/v1/admin/disclosure-types/{type_id}/versions`
- `GET /api/v1/admin/disclosure-types/{type_id}/versions/{version_no}`
- `POST /api/v1/admin/disclosure-types/{type_id}/activate`
- `GET /api/v1/admin/disclosure-types/{type_id}/config`
- `PUT /api/v1/admin/disclosure-types/{type_id}/config`
- `GET /api/v1/company/disclosure-types/{type_id}/workflow-override`
- `PUT /api/v1/company/disclosure-types/{type_id}/workflow-override/draft`
- `POST /api/v1/company/disclosure-types/{type_id}/workflow-override/approve`
- `DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/{version_no}`
- `DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/active`
- `GET /api/v1/company/disclosure-types/{type_id}/workflow-override/versions`
- `GET /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/reminder-preview`
- `PUT /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/steps/{step_id}/groups`
- `GET /api/v1/company/groups`
- `GET /api/v1/disclosure-types/{type_id}/effective-workflow`
- `GET /api/v1/company/disclosure-types/{type_id}/preferences`
- `PATCH /api/v1/company/disclosure-types/{type_id}/preferences`

### Flow ownership

- disclosure creation, save, submit, confirm
- disclosure type browsing and selection
- platform template versioning/activation
- company-specific workflow override drafting and approval
- deadline preference tuning and reminder preview

## `internal/workflow`

### Responsibility

- workflow execution primitives
- task action dispatch
- reminder visibility per workflow instance
- assignee resolution for dynamic workflow routing

### Main FE surfaces

- disclosure detail review actions
- task-oriented approval surfaces
- reminder lists tied to workflow instances
- workflow builder previews and assignee checks

### Main endpoints

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

### Flow ownership

- instantiate workflow from disclosure/business action
- step-level human review/approve/confirm/reject
- fetch pending tasks/reminders for a workflow
- resolve assignment targets from group/rule-driven config

## `internal/platformcms`

### Responsibility

- platform-facing CMS content operations
- platform company and platform-user administration
- ops dashboards and observability views
- holiday calendar management

### Main FE surfaces

- `/cms`
- `/cms/content/collections`
- `/cms/content/entries`
- `/cms/content/media`
- `/cms/publishing/review`
- `/cms/publishing/schedule`
- `/cms/publishing/releases`
- `/cms/templates`
- `/cms/admin/companies`
- `/cms/admin/users`
- `/cms/admin/roles`
- `/cms/admin/rules`
- `/cms/ops/audit`
- `/cms/ops/sessions`
- `/cms/ops/health`
- `/cms/settings/holiday-calendar`

### Main endpoints

- `GET /api/v1/platform/cms/dashboard/summary`
- `GET /api/v1/platform/cms/collections`
- `GET /api/v1/platform/cms/collections/{collection_id}`
- `GET /api/v1/platform/cms/entries`
- `GET /api/v1/platform/cms/entries/{entry_id}`
- `POST /api/v1/platform/cms/entries`
- `PUT /api/v1/platform/cms/entries/{entry_id}`
- `POST /api/v1/platform/cms/media/upload`
- `PUT /api/v1/platform/cms/media/upload/{asset_id}`
- `POST /api/v1/platform/cms/media/{asset_id}/complete`
- `GET /api/v1/platform/cms/media`
- `DELETE /api/v1/platform/cms/media/{asset_id}`
- `GET /api/v1/platform/cms/reviews`
- `POST /api/v1/platform/cms/reviews/{entry_id}`
- `GET /api/v1/platform/cms/schedules`
- `POST /api/v1/platform/cms/schedules`
- `DELETE /api/v1/platform/cms/schedules/{entry_id}`
- `GET /api/v1/platform/cms/releases`
- `GET /api/v1/platform/cms/admin/users`
- `POST /api/v1/platform/cms/admin/users`
- `POST /api/v1/platform/cms/admin/users/invite`
- `POST /api/v1/platform/cms/admin/users/{user_id}/resend-invitation`
- `POST /api/v1/platform/cms/admin/users/{user_id}/assign-company`
- `POST /api/v1/platform/cms/admin/users/{user_id}/request-password-reset`
- `GET /api/v1/platform/cms/admin/companies`
- `GET /api/v1/platform/cms/admin/companies/{company_id}`
- `PATCH /api/v1/platform/cms/admin/companies/{company_id}`
- `POST /api/v1/platform/cms/admin/companies/{company_id}/deactivate`
- `POST /api/v1/platform/cms/admin/companies/{company_id}/activate`
- `POST /api/v1/platform/cms/admin/companies`
- `POST /api/v1/platform/cms/admin/companies/{company_id}/members`
- `GET /api/v1/platform/cms/admin/roles`
- `GET /api/v1/platform/cms/admin/rules`
- `POST /api/v1/platform/cms/admin/rules/validate`
- `GET /api/v1/platform/cms/ops/audit`
- `GET /api/v1/platform/cms/ops/sessions`
- `POST /api/v1/platform/cms/ops/sessions/{session_id}/revoke`
- `GET /api/v1/platform/cms/ops/health`
- `GET /api/v1/platform/cms/ops/metrics`
- `GET /api/v1/platform/cms/holiday-calendars/{year}`
- `POST /api/v1/platform/cms/holiday-calendars/{year}/preview`
- `PUT /api/v1/platform/cms/holiday-calendars/{year}`

### Flow ownership

- CMS dashboard and content collections/entries
- media upload lifecycle
- content review/schedule/release flows
- platform company and platform-user operations
- ops audit/sessions/health/metrics
- holiday calendar preview and replace

## Cross-Module Notes

- `iam` owns identity/session context, but many FE screens only become usable after `me/effective-access` bootstrap feeds route guards.
- `companyaccess` and `platformcms` both expose admin surfaces, but they operate at different scopes:
  - `companyaccess`: current tenant only
  - `platformcms`: platform-wide or multi-tenant administration
- `disclosure` defines content/business state, while `workflow` defines approval execution and assignee resolution around that state.
- Reminder, notification, audit, and authorization behaviors are supporting modules that cut across all five primary modules rather than being best read as a single FE surface.
