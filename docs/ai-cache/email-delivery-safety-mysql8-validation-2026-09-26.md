# Email delivery safety — MySQL 8 validation (2026-09-26)

SMTP wiring is recorded in `email-delivery-safety-smtp-dev-validation-2026-09-26.md`. Flags stayed off. No DEV SMTP smoke was sent.

## DEV result

Documented target is host `88.216.208.0` port `21239`, path `/root/cobo_project`, hostname `avi-server1`. API `/healthz` and `/readyz` returned 200 before and after the backend deploy. Local `.env` was not loaded. Shared database name is `cobo_iam`. An isolated schema `cobo_email_delivery_safety_test` was created on that same MySQL 8 instance, used for migration and claim proof, then dropped. `cobo_iam` was not migrated.

```text
ENVIRONMENT=DEV
DEV_TARGET_VERIFIED=true
MYSQL_VERSION=8.0.46
MYSQL_DATABASE_REDACTED=cobo_iam
MYSQL_MIGRATION_VALIDATION=PASS
LIVE_PREFLIGHT=PASS
0146_RESULT=PASS
0147_DUPLICATE_GUARD_RESULT=PASS
0147_SUCCESS_PATH_RESULT=PASS
RERUN_RESULT=PASS
DOWN_MIGRATION_RESULT=PASS
CONCURRENT_CLAIM_RESULT=PASS
WORKER_GATE_SMOKE=PASS
FLAGS_ENABLED=false
SMTP_BINDING_WIRED=false
FRONTEND_CHANGED=false
PRODUCTION_TOUCHED=false
```

Shared `cobo_iam` ledger stops at `0143`. `reminder_dispatch_resolutions` is absent there, so `0146`/`0147` were not applied to the smoke schema. Applying them would first require `0144`/`0145`, which add catalog triggers outside this proof. Duplicate groups, provider mismatches, and stale `SENDING` counts on that schema are zero because the table does not exist.

Isolated proof on MySQL 8.0.46:

- Duplicate pair raised `ERROR 1644 (45000)` and created no `uk_reminder_dispatch_occurrence_company`. `updated_at` stayed null.
- Clean path filled only the null `updated_at` (`2026-01-01`) and left the existing timestamp (`2026-02-02 03:04:05.000`) unchanged.
- Indexes after success included both `uk_reminder_dispatch_occurrence` and `uk_reminder_dispatch_occurrence_company`. The procedure was dropped.
- Re-run of `0146` and `0147` did not change that timestamp.
- Down `0147` removed only the company unique and left the occurrence unique, the table, and both rows.
- Down `0146` removed the lease/retry columns and left the occurrence unique and the two rows.

`go test -count=20 -run TestMySQLConcurrentClaimAndTerminalGuards ./internal/workflowdept/infra/mysql` passed against the isolated schema through an SSH tunnel. `-race` was not run. No SMTP was called.

Worker and API were redeployed with `deploy-dev.ps1 -Mode be -SkipTests`. `WORKFLOW_DEPARTMENT_*` is unset in the worker. Recent worker logs have no binding line. Frontend was not deployed.

Actual runner filename remains `0147_workflow_department_email_recipient_backfill.up.sql`. The duplicate count is `(company_id, occurrence_id)`.

- branch: `fixbug/test-9`
- repos: `cobo_iam_services` changed; `cobo_web_design` clean
- flags: all `WORKFLOW_DEPARTMENT_*_ENABLED` defaults remain false
- binding SMTP: not connected (`smtp_transport_unwired`)
- recommendation: **GO FOR PRE-MERGE REVIEW**

## Files in this verification pass

- `migrations/0147_workflow_department_email_recipient_backfill.up.sql` — `SIGNAL` moved out of `PREPARE` into procedure `cobo_0147_email_delivery_safety`
- `migrations/0147_workflow_department_email_recipient_backfill.down.sql` — also drops that procedure
- `internal/workflowdept/binding_deliver_test.go` — illegal transitions and lease/retry claim
- `internal/workflowdept/infra/mysql/delivery_mysql_integration_test.go` — concurrent MySQL claim; skips without a test DSN

## Migration review

`0146` is expand-only. Each column and `idx_reminder_dispatch_claim` is added only when missing. Down drops that index and the columns `lease_id`, `lease_until`, `updated_at`, `attempt_count`, `next_retry_at`, `last_error_code`. It does not drop the table.

`0147` counts duplicate `occurrence_id` values and `SIGNAL`s before `ADD UNIQUE`. `updated_at` is set from `resolved_at` only inside the procedure and only after the duplicate count is zero, and only where `updated_at IS NULL`. Down drops `uk_reminder_dispatch_occurrence_company` and the procedure. It does not drop `0144`'s occurrence unique key.

The previous `PREPARE` + `SIGNAL` form is invalid on MySQL 8. The manual lists the statements `PREPARE` accepts, and `SIGNAL` is not one of them. Docker could not run the replacement:

```text
docker version
error during connect: open //./pipe/docker_engine: The system cannot find the file specified.
MYSQL_MIGRATION_VALIDATION=BLOCKED
```

No production database was used.

## Earlier local attempt (superseded by the DEV result above)

## Live preflight

```text
LIVE_PREFLIGHT=NOT_RUN
reason=MYSQL_DSN not configured; .env not loaded
```

## Concurrent claim

`TestMySQLConcurrentClaimAndTerminalGuards` **SKIP**. No `MYSQL_TEST_DSN` or `MYSQL_DSN`. In-memory tests cover one-winner claim, live lease, expired lease via reaper, and terminal rejection. That is not a MySQL race proof.

## Commands

| Command | Result |
|---|---|
| `go test ./internal/workflowdept/...` | PASS (MySQL integration skipped) |
| `go test ./internal/reminder/...` | PASS |
| `go test ./migrations/` | PASS |
| `go build ./...` | PASS |
| `go test ./internal/notification/...` | FAIL `TestContract_VariableParity/workflow.approved` — pre-existing, outside this diff |
| `go test ./...` | FAIL exit 1. `workflowdept` and `migrations` PASS. Failures also in `companyaccess`, `legal_basis_backfill`, `httpserver`, `platform/config` — outside this diff |
| MySQL 8 apply of 0146/0147 | **BLOCKED** |
| Docker | **BLOCKED** daemon pipe missing |

## Flag and rollback

Worker calls `RunBindingTick`, which returns before store, key load, decrypt, and SMTP unless both `WORKFLOW_DEPARTMENT_BINDING_ENABLED` and `WORKFLOW_DEPARTMENT_EMAIL_BINDING_ENABLED` are on. Default config passes `false` for every workflow-department flag. Rollback while flags stay off does not claim rows. Down scripts are reviewed and not executed.

## Decision

**GO FOR PRE-MERGE REVIEW.** Isolated MySQL 8.0.46 proof covers `0146`, duplicate `SIGNAL`, success path, rerun, down, and a 20-run concurrent claim. Shared `cobo_iam` still has no `0144` table, so flags must stay off there. No merge, push, or production deploy.
