# Feature Implementation Status

> Updated: 2026-05-22
> Scope: workspace-level view of `implemented`, `partial`, and `planned/spec-driven` functionality

## Status Rules

- `Implemented`
  FE route/service and BE endpoint/module are both present in current code.
- `Partial`
  A feature is clearly designed and partially wired, but some backend surface, runtime path, or supporting behavior is incomplete or only indirectly traceable.
- `Planned`
  The feature appears in specs/docs/review notes, but a concrete current FE+BE implementation surface is not fully present in code.

## Implemented

- Public auth lifecycle:
  - login
  - register
  - refresh
  - logout
  - invitation validate/accept
  - reset password
  - email verification and resend
- Company-context lifecycle:
  - select company
  - switch company
  - current user bootstrap
  - effective access lookup
  - membership/capabilities lookup
- Portal disclosure core:
  - disclosure type list/detail service surfaces
  - company workflow override draft/approve/version flows
  - disclosure create/detail/edit service surfaces
  - disclosure submit/confirm action surfaces
- Workflow transport:
  - create instance
  - list tasks/reminders
  - task review/approve/confirm/reject
  - resolve assignees
- Ad-hoc proposal flow:
  - create/list/detail
  - submit
  - focal approve
  - admin approve
  - reject/cancel
- Enterprise admin core:
  - admin hub summary
  - users/memberships management
  - department management
  - title management
  - company profile get/update
  - audit/session admin surfaces
  - role/rule admin surfaces
- CMS core:
  - dashboard
  - collections
  - entries
  - media
  - review queue
  - schedules
  - releases
  - platform company management
  - platform user management
  - platform roles/rules list/validate
  - ops audit
  - ops sessions
  - ops health/metrics
  - holiday calendar
- Authorization internals:
  - internal authorize
  - internal authorize batch
  - effective access cache wiring
- Async/runtime:
  - outbox writer path
  - worker process
  - reminder/notification dispatch infrastructure

## Partial

- Dashboard data aggregation:
  - FE route exists
  - final dashboard endpoint contract is implied but not cleanly centralized as a dedicated current backend module surface
- Recipients module:
  - FE route exists in portal
  - full explicit BE endpoint inventory is not cleanly visible in the current handler scan
- Deadlines/history views:
  - FE routes exist
  - backend support is distributed across disclosure/reminder/runtime behavior rather than a single obvious route group
- Alert channels:
  - FE route exists
  - notification transport endpoints are present, but portal-level settings contract appears broader than the visible backend route set
- `/app/settings`:
  - route exists
  - current backend settings surface is not clearly isolated
- CMS taxonomy:
  - FE route/spec exists
  - current BE taxonomy endpoints are not visible in platform CMS route registration
- CMS general/localization/integration settings:
  - FE routes/specs exist
  - matching BE endpoints are not visible in current platform CMS handler registration
- Worker/runtime readiness:
  - architectural path is present
  - current deploy/runtime logs in this session show operational issues still being stabilized

## Planned / Spec-Driven

- Full reminder-channel expansion beyond email:
  - SMS
  - Zalo
  - in-app notifications
- Subscription-tier server-side quota hardening across all premium/enterprise boundaries
- SSO and MFA hooks expansion
- Broader CMS settings CRUD beyond currently visible ops/holiday surfaces
- Deeper analytics/reporting dashboards beyond current portal summary shells
- Additional operational hardening:
  - migration bootstrap ergonomics
  - deploy/runtime stabilization
  - environment parity improvements

## Notes

- `Partial` here does not mean unusable. It means the cross-repo trace from route -> service -> handler is incomplete, indirect, or still evolving.
- `Planned` items are intentionally conservative. This section lists only areas that are clearly present in active specs/docs but not fully evidenced in current route/handler wiring.
