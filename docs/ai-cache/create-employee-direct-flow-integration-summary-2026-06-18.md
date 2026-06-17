# Create Employee Direct Flow Integration Summary

- Created: 2026-06-18
- Updated: 2026-06-18
- Skill: integration-cross-repo
- Scope: `cobo_iam_services` + `cobo_web_design`

## Docs consulted

- `docs/api-contracts-json.md`
- `../cobo_web_design/docs/ai-cache/admin-center-staff-add-employee-implementation-plan.md`
- `docs/ai-cache/invite-user-title-assignment-integration-summary-2026-06-18.md`

## Summary

The backend direct-create path (`POST /api/v1/admin/users`) now supports creating a user and bootstrapping company membership data in the same request:

- initial role
- direct grantable permissions
- department assignment
- title assignment

Two integration fixes were completed at the same time:

- company-scoped admins no longer lose the requested `department_id` during create/invite scope resolution
- `GET /api/v1/admin/companies/{company_id}/memberships` now returns the flat payload shape expected by frontend and honors `page` / `page_size`

## Shared contract

Added optional fields on `CreateUserRequest` / HTTP decode:

- `role_id`
- `role_code`
- `permissions[]`
- `department_id`
- `title_id`

Create semantics:

- role is assigned inside the create transaction
- direct permissions are inserted after membership creation
- default read workflow permission is still granted automatically
- department and title are assigned in the same flow

## Backend impact

- `internal/companyaccess/app/admin.go`
  - extended `CreateUserRequest`
- `internal/companyaccess/app/admin_service.go`
  - resolved role before create
  - inserted direct permissions
  - assigned department/title after membership creation
- `internal/companyaccess/app/admin_service_invite_scope.go`
  - preserved explicit `department_id` for company-scoped admins
- `internal/companyaccess/infra/mysql/admin_repository.go`
  - assigned initial role in the same transaction as user + membership creation
- `internal/companyaccess/transport/http/admin_handler.go`
  - decoded new create fields
  - fixed memberships list response shape and pagination passthrough

## Verification notes

- PASS: `go test ./internal/companyaccess/app -run 'TestAdminService_CreateUser_|TestAdminService_DirectPermissions_DeniedWithoutRbacManage' -count=1`
- PASS: `go test ./internal/companyaccess/transport/http -run 'TestListMemberships_Handler_UsesFlatContractAndPagination' -count=1`
- Go test required `GOCACHE` redirected into workspace because the default machine cache path was blocked by environment permissions.

## Remaining gaps

- Full `go test ./...` and Docker build were not rerun in this turn.
- Frontend build/test still needs a less restricted environment for Vite/Vitest execution.
