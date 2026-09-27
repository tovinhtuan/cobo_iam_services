# Email Delivery Safety — source review (2026-09-26)

## Verification follow-up (2026-09-26, same day)

**NO-GO — verification incomplete.**

Binding claim/reaper code and migrations `0146`/`0147` now exist. Flags stay default false. `cobo_web_design` has no diff. Binding SMTP is still unwired (`smtp_transport_unwired`).

`0147` no longer puts `SIGNAL` inside `PREPARE`. MySQL 8's prepared-statement list does not include `SIGNAL` ([SQL Syntax Permitted in Prepared Statements](https://dev.mysql.com/doc/refman/8.0/en/sql-prepared-statements.html)). The abort now lives in procedure `cobo_0147_email_delivery_safety`. That rewrite has **not** been executed on a server.

`docker version` failed: `open //./pipe/docker_engine: The system cannot find the file specified.`

```text
MYSQL_MIGRATION_VALIDATION=BLOCKED
LIVE_PREFLIGHT=NOT_RUN
reason=MYSQL_DSN not configured; .env not loaded
MYSQL concurrent claim=SKIP
```

Items 1 and 6 below are the earlier review. They are superseded for "no store" and "no migration files". The flag-enable NO-GO is not superseded.

## Verdict

**NO-GO for flag enable or production rollout.** The codebase has useful primitives, but the 2026-09-25 plan is not yet an executable implementation contract.

## Findings

1. `reminder_dispatch_resolutions` already stores encrypted recipient data, key version, `send_status`, and provider id (`migrations/0144_workflow_department_binding.up.sql`), but no DB repository/worker path currently claims, sends, or reaps these rows. `internal/workflowdept/delivery.go` is pure state-helper code only.
2. The existing durable email pipeline is separate: `email_notifications` + `email_delivery_attempts` + `email.dispatch` outbox. Its handler counts attempts and then calls an unconditional `MarkSending`; the SQL update does not constrain the previous state. Concurrent redelivery can therefore race unless the new binding path introduces an atomic claim/lease.
3. Retry policies conflict. The plan requires `1m, 3m, 6m, 10m` and attempt 5 permanent, while `internal/notification/app/email_delivery.go` currently uses `1m, 5m, 15m, 1h, 6h`.
4. Empty-SMTP behavior conflicts by path. The legacy reminder sender returns `mock-no-smtp` as success, while the plan requires binding delivery to become `PERMANENT_FAILED` with a null provider id. The binding path must be explicit and must not silently inherit legacy behavior.
5. Recipient encryption has unit-level AES-GCM helpers, but no production key loader/configuration or rotation/reconciliation path. The plan correctly keeps the flag disabled until this exists.
6. Migration `0146/0147` files are absent and the migration runner currently ends at `0145`; both migration files and runner ordering/tests are required.
7. No frontend change is required by this plan; the actual implementation scope is backend worker, MySQL migrations, config/secrets, authorization, and operational verification.

## Verification

- `go test ./internal/workflowdept/...` passed when run with a workspace-local Go build cache.
- The broader notification test group exposed an existing failure in `TestContract_VariableParity/workflow.approved` (`workflow_instance_id` declared but unused in the template metadata). The initial run also hit a Windows permission issue in the default Go cache/telemetry paths.

## Recommended next step

Before coding migrations, freeze a short contract covering: the authoritative table (`reminder_dispatch_resolutions` versus `email_notifications`), atomic claim/lease columns and predicates, exact retry schedule, empty-SMTP behavior, key source/rotation, reconciliation permission, and the worker's interaction with the outbox. Then run a live DB preflight for schema columns and `schema_migrations`. Only after that should `0146/0147` and the binding worker slice be implemented.

**Cached for:** Team reuse, code reviews, and implementation planning.
