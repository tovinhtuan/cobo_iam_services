---
name: backend-feature-delivery
description: Dùng khi triển khai hoặc thay đổi feature trong cobo_iam_services có thể liên quan đồng thời API, IAM, MySQL, Redis, outbox, worker, migration hoặc frontend contract.
---

# Backend Feature Delivery

## When to use

Use this skill when a backend task is larger than a single isolated function
or touches more than one of API, authz, persistence, cache, worker, migration,
observability, or frontend integration.

Route to the narrower skill when applicable:

- `backend-api-contract` for request/response and error changes;
- `backend-authz-session` for login, token, session, password, or permission changes;
- `backend-db-migration-safe` for schema or migration work;
- `backend-worker-idempotent` for worker, outbox, retry, or asynchronous work;
- `integration-cross-repo` when `cobo_web_design` also changes;
- `premerge-system-review` before declaring a substantial change complete.

## Workflow

1. Read the repository README, relevant `docs/ai-cache/` context, local rules,
   and the affected packages before coding.
2. Establish current behavior from code and tests. Separate confirmed behavior
   from inferred business intent.
3. Define the feature contract:
   - endpoint or event shape;
   - authentication and authorization requirements;
   - validation and normalized error model;
   - transaction and consistency boundaries;
   - idempotency, retry, timeout, and concurrency behavior;
   - cache read/write/invalidation behavior;
   - audit, metrics, and structured logging expectations.
4. Map the change to layers: handler, service/usecase, repository, database,
   cache, external system, worker, and migration. Keep business logic out of
   transport and persistence layers where existing architecture supports that.
5. Implement the smallest vertical slice. Preserve both in-memory development
   behavior and MySQL-backed behavior unless the task explicitly changes the
   supported modes.
6. Add tests for the success path and the highest-risk failure paths: invalid
   input, permission denial, not found/conflict, dependency failure, duplicate
   execution, transaction failure, or stale cache as applicable.
7. Review migration order, rollback implications, backward compatibility,
   operational visibility, and frontend contract impact.
8. Run focused tests, then the broader verification required by the changed
   packages and Docker image.

## Quality guardrails

- Never expose raw database models or internal error details as public API
  contracts without an explicit decision.
- Never weaken authorization because a route is internal; verify the caller
  boundary and service-to-service assumptions.
- Never add a migration without checking ordering, existing data, rollback or
  forward-only rationale, and deployment compatibility.
- Never assume a worker runs once. Protect every retriable side effect with an
  idempotency or deduplication strategy.
- Do not invalidate or populate Redis without identifying stale-data behavior
  and failure fallback.
- Do not log passwords, tokens, credentials, DSNs, private payloads, or secret
  headers.
- Keep API error codes stable and map new errors to actionable client behavior.
- Do not mix unrelated refactors into a feature change.

## Required output

```text
Current behavior:
Target behavior:
Contract:
Authorization and validation:
Affected layers/files:
Transaction/cache/worker behavior:
Migration and compatibility impact:
Test matrix:
Verification results:
Remaining risks:
```

## Verification baseline

Run from `cobo_iam_services` as relevant:

```text
go test ./...
go vet ./...
docker compose -f docker-compose.dev.yml build api
```

Add worker/full-stack Compose checks when those components are affected. If a
required check cannot run, report `BLOCKED:` with the concrete reason and still
report checks that did run.
