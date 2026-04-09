# P0.2 Migration 0001 Summary

## Completed
- Added `migrations/0001_init_core.up.sql` and `migrations/0001_init_core.down.sql`.
- `0001_init_core.up.sql` includes core tables for:
  - IAM: `users`, `credentials`, `sessions`, `login_attempts`
  - CompanyAccess: `companies`, `memberships`, `roles`, `membership_roles`, `departments`, `department_memberships`, `titles`, `membership_titles`
  - Authorization core: `permissions`, `role_permissions`, `assignments`, `resource_scope_rules`
  - Audit/Platform: `audit_logs`, `outbox_events`, `idempotency_keys`
- Implemented required hot-path indexes from planning docs:
  - memberships unique/index status paths
  - membership_roles active-window index
  - department_memberships active-window and department index
  - assignments assignee/resource lookup indexes
  - audit_logs company/actor/resource/request indexes

## Notes
- All tables use InnoDB + utf8mb4.
- JSON columns are used only for snapshots/metadata payloads.
- Foreign keys are included for critical relations where available in P0.

## Verification
- Project build remains green: `go build ./...`.
- Migration SQL files are syntactically structured and ready for integration with a migration runner in next step.

## Next
- P0.3: define domain/app contracts and repository interfaces (IAM, MembershipQuery, Authorizer, Audit, Outbox).
