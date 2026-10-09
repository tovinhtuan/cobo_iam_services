---
name: backend-feature-delivery
description: Dùng khi triển khai hoặc thay đổi feature trong cobo_iam_services (Go) có thể liên quan đồng thời API handler, IAM, MySQL, Redis, outbox, worker, migration, feature flag hoặc frontend contract.
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
- `backend-performance-reliability` for hot paths, concurrency, pools, timeouts;
- `api-compatibility-rolling-deploy` when response shape, enum or error codes change;
- `cobo-admin-role-guard` for platform CMS or tenant admin endpoints;
- `premerge-system-review` before declaring a substantial change complete.

## Project facts (verify before relying on them)

- Router: stdlib `net/http.ServeMux` with Go 1.22 method patterns, wired in
  `internal/httpserver/`. Middleware is only CORS + request ID; auth and
  permission checks happen **inside each handler** (`TokenInspector` →
  `GetEffectiveAccess`). A new handler that forgets this is unauthenticated.
- Layering per module: `transport/http → app (service) → infra (mysql/inmemory) → domain`.
- Errors: `internal/platform/errors` codes + `internal/platform/httpx` JSON
  envelope `{"error":{"code","message","details"}}`.
- SQL: raw `database/sql` with `?` placeholders; inline `BeginTx` + deferred
  rollback; outbox rows can join a tx via `outbox/mysql` `InsertTx`.
- Idempotency: `internal/platform/idempotency` (`Idempotency-Key`, 24h).
- No `MYSQL_DSN` → in-memory repositories. Keep both paths working.
- Feature flags are env vars in `internal/platform/config`; also add them to
  `.env.example`, `docker-compose.dev.yml` and `docker-compose.artifacts.yml`.
- Migrations must be appended to the ordered list in
  `migrations/run_dev_migrations.sh`, otherwise they never deploy.

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
