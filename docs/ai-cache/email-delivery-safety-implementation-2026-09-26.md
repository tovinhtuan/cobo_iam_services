# Email delivery safety — implementation (2026-09-26)

- task type: implement flag-off claim/reaper scaffold
- skill: backend-db-migration-safe, backend-worker-idempotent, premerge-system-review
- branch: `fixbug/test-9`
- `cobo_web_design`: **unchanged**

## Recommendation

**NO-GO** for enabling flags, applying `0146`/`0147` on production, or sending email.

The binding path stays off. `WORKFLOW_DEPARTMENT_BINDING_ENABLED` and `WORKFLOW_DEPARTMENT_EMAIL_BINDING_ENABLED` still default false. Email flag alone cannot start the path.

## Files changed (`cobo_iam_services` only)

- `docs/ai-cache/email-delivery-safety-contract-2026-09-26.md`
- `docs/ai-cache/email-delivery-safety-preflight-2026-09-26.md`
- `migrations/0146_workflow_department_email_delivery_safety.up.sql` + `.down.sql`
- `migrations/0147_workflow_department_email_recipient_backfill.up.sql` + `.down.sql`
- `migrations/0146_workflow_department_email_delivery_safety_test.go`
- `migrations/run_dev_migrations.sh` (lists 0146 then 0147 after 0145)
- `internal/workflowdept/delivery.go` (claim also allows `RETRYABLE_FAILED`)
- `internal/workflowdept/binding_retry.go`
- `internal/workflowdept/binding_deliver.go`
- `internal/workflowdept/binding_deliver_test.go`
- `internal/workflowdept/recipient_key.go`
- `internal/workflowdept/infra/mysql/delivery_store.go`
- `internal/workflowdept/infra/mysql/delivery_store_test.go`
- `cmd/worker/main.go` (tick calls the binding runner; runner returns before SQL when flags are off)

`0144` was not edited. Legacy `SendReminderEmail` empty-host `mock-no-smtp` behavior was not edited. `email_notifications` retry policy was not edited.

## State machine

Authoritative table: `reminder_dispatch_resolutions`.

Claimable: `PENDING`, `RETRYABLE_FAILED` (and due).
`SENDING` is leased. Stale `SENDING` is reaped to `SEND_UNKNOWN`.
`SENT`, `SEND_UNKNOWN`, and `PERMANENT_FAILED` are not resent.

## SQL claim predicate

```sql
UPDATE reminder_dispatch_resolutions
SET send_status = 'SENDING',
    lease_id = ?,
    lease_until = ?,
    updated_at = ?
WHERE resolution_id = ?
  AND send_status IN ('PENDING', 'RETRYABLE_FAILED')
  AND (lease_until IS NULL OR lease_until < ?)
  AND (next_retry_at IS NULL OR next_retry_at <= ?);
```

Due scan uses the same status/lease/retry predicate with `ORDER BY resolved_at LIMIT 1`. `RowsAffected = 0` is `not_found`, not an error. Lease is 90s. Claim does not increment `attempt_count`.

## Migration status

Expand-only, re-runnable via `information_schema` guards. `0147` fills `updated_at` from `resolved_at` only when null and creates `uk_reminder_dispatch_occurrence_company` only after a zero duplicate count. It does not invent `email_key_version`.

Not applied anywhere in this task. **LIVE_PREFLIGHT=NOT_RUN** (`MYSQL_DSN` unset; dotenv was not loaded).

## Tests

| Command | Result |
|---|---|
| `go test ./internal/workflowdept/...` | PASS |
| `go test ./internal/reminder/...` | PASS |
| `go test ./migrations/` | PASS |
| `go test ./internal/notification/...` | FAIL `TestContract_VariableParity/workflow.approved` — pre-existing; template metadata, not this diff |
| `go test ./...` | FAIL exit 1 in `companyaccess`, `disclosure/app/legal_basis_backfill`, `httpserver`, `notification/app`, `platform/config`. Those packages are not in this diff. `workflowdept` and `migrations` PASS inside that run |
| `go build ./...` | PASS |
| MySQL integration for 0146/0147, concurrent claim, reaper | **NOT_RUN** — no MySQL harness / DSN |
| `docker compose -f docker-compose.dev.yml build api` | **BLOCKED:** local Docker daemon did not respond (`docker info` exit failure) |

## Flag status

All four workflow-department flags remain default off. This task did not set them. Binding SMTP is not dialed: a missing transport is `PERMANENT_FAILED` / `smtp_transport_unwired`, and empty `SMTP_HOST` is `smtp_host_empty` with a null provider id. Neither path calls the legacy sender.

## Known limitations

- Live column list, row counts, and duplicate report are unknown.
- No reconciliation HTTP API. `platform.cms.view` stays unused for this.
- One active recipient key from env only. Version mismatch fails closed. No key is stored in MySQL.
- Real `net/smtp` is not wired on the binding path, so flag-on still cannot deliver. That is intentional until the live preflight, key, permission, and MySQL race tests exist.
- `0147` now uses a stored procedure so `SIGNAL` is not prepared. MySQL 8 execution of that procedure is still blocked.

## Rollback

Keep flags off. That stops the worker before any claim SQL. Down scripts drop only `0146` columns/index and the `0147` company unique index. They do not drop the table or the `0144` occurrence unique key.

## Pre-merge review

- Critical: do not enable flags or migrate production while live preflight and MySQL race tests are open. Duplicate email protection is only proven in the in-memory claim test.
- Important: binding transport is deliberately unwired; `SIGNAL` under `PREPARE` is unverified on MySQL.
- Nice-to-have: metrics for `last_error_code` counts once the path is allowed on.

**NO-GO**
