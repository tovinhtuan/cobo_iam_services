# P1.2 Workflow Module Skeleton Summary

## Completed
- Added workflow module skeleton with clear boundaries:
  - `internal/workflow/domain/workflow.go`
  - `internal/workflow/app/contracts.go`
  - `internal/workflow/app/service.go`
  - `internal/workflow/infra/inmemory/repository.go`
  - `internal/workflow/transport/http/handler.go`

## Implemented service contracts
- `CreateWorkflowInstance`
- `ApproveTask`
- `ReviewTask`
- `ConfirmTask`
- `ResolveAssignees`

## Exposed HTTP endpoints (skeleton)
- `POST /api/v1/workflows/instances`
- `POST /api/v1/workflows/tasks/{task_id}/review`
- `POST /api/v1/workflows/tasks/{task_id}/approve`
- `POST /api/v1/workflows/tasks/{task_id}/confirm`
- `POST /api/v1/workflows/resolve-assignees`

## Authorization and tenant isolation
- Subject extracted from access token (`user_id`, `membership_id`, `company_id`).
- All workflow actions call central `authorization.Service`.
- Repository keys are scoped by `company_id` to preserve tenant boundary.

## Skeleton behavior
- Creating workflow instance creates an initial `review` task assigned to current membership.
- Task actions enforce:
  - assignee membership match
  - pending-state precondition
- `ResolveAssignees` currently returns current membership as baseline candidate.

## Verification
- `go build ./...` passed.
- `go test ./...` passed.
- IDE lints for changed workflow files: clean.

## Next
- Continue roadmap with P1.3 Notification module skeleton.
