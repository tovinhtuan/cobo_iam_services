# Deadline alert 504 performance fix — 2026-10-02

## Task type / objective

Backend bug fix for DEV `GET /api/v1/company/deadline-alerts` and
`GET /api/v1/company/dashboard/overview` returning 504. No route, HTTP
request/response contract, authorization policy, tenant boundary, deployment,
commit, or frontend source change is included.

## Confirmed source path

- The alert list read every matching row, plus company-wide workflow/ad-hoc/task
  metadata, before application-layer filtering and pagination.
- Dashboard invoked that path for four status buckets, two seven-day totals, and
  one or more DONE pages. The list failure then triggered the frontend fallback
  requests observed in DEV.

## Implemented

- Added an internal `ListDeadlineAlertSnapshot` service read. Dashboard resolves
  the accessible alert set once and derives its range/status aggregates locally;
  the public overview endpoint remains unchanged.
- Added an optional MySQL paged read for an unfiltered deadline-list request:
  `COUNT(*)` plus `LIMIT/OFFSET`. Workflow/ad-hoc/task enrichment is now batched
  only for record IDs in that page.
- Requests containing status, date, department, search, or display-group
  filters retain the established full-resolution path because dynamic due-date
  calculation is application-owned. This avoids changing exact filter and total
  semantics without a read model.
- Added regression coverage for a 10,000-row paged response boundary, one
  dashboard snapshot instead of repeated list calls, and stable error envelopes
  for both routes.

## Compatibility / operations

- No migration or index was added. Existing migration evidence has no current
  runtime EXPLAIN proving an index change is mandatory.
- Rollback is source-only: revert this change restores the prior list pipeline;
  no data conversion or cache invalidation is required.
- Remaining performance risk: filtered alert-list queries still need a future
  materialized/read-model design if their own production cardinality becomes
  high. Do not replace that path with SQL pagination until the dynamic due-date
  predicate and exact `total` semantics are represented safely.

## Verification

- Focused deadline-alert/dashboard app, MySQL-boundary, and HTTP tests: PASS.
- `go test ./...`: changed packages PASS; baseline unrelated failures remain in
  company-access feature-flag tests and Windows file-mode expectation.
- `go vet ./...`: BLOCKED by pre-existing `workflowfulfillment` test copies of
  `sync/atomic.noCopy`.
- `docker compose -f docker-compose.dev.yml build api`: BLOCKED because the
  local Docker daemon is not running.
- DEV browser read-only smoke: existing deployed `/app/deadlines` still reached
  `Request failed: 504` after about 55 seconds. The local source was not
  deployed by this task, so post-deploy verification remains required.
