# Email delivery safety — SMTP wiring (2026-09-26)

## Decision

**GO FOR PRE-MERGE REVIEW**

**NO-GO** for production rollout.

**Not** `GO FOR DEV SMTP SMOKE`. Flags stayed off. No email was sent.

```text
ENVIRONMENT=DEV
DEV_TARGET_VERIFIED=true
SMTP_TRANSPORT=existing_adapter
SMTP_BINDING_WIRED=true
SMTP_TEST_SENT=NOT_RUN
APPROVED_TEST_RECIPIENT=false
DUPLICATE_SEND_GUARD=PASS
CONCURRENT_CLAIM=PASS
FLAGS_ENABLED=false
TASK_ROUTING_ENABLED=false
BACKFILL_WRITE_ENABLED=false
FRONTEND_CHANGED=false
PRODUCTION_TOUCHED=false
ROLLBACK_VERIFIED=true
SMTP_DEV_SMOKE=NOT_RUN
reason=no approved test recipient
```

DEV host is the documented `avi-server1` (`88.216.208.0`). `/healthz` and `/readyz` were 200 after `deploy-dev.ps1 -Mode be -SkipTests`. Worker environment has no `WORKFLOW_DEPARTMENT_*` variables. Frontend was not deployed. Shared `cobo_iam` still has no `reminder_dispatch_resolutions` table, so a live send was not attempted.

## Wiring

Binding mail uses `internal/notification/infra/smtp.BindingMailer`, which builds the message with the existing `BuildMessage` helper and the same `smtp.SendMail` transport as `Adapter`. It does not call `SendReminderEmail` and does not use `log_only` or `mock-no-smtp`.

Empty host or port `<= 0` returns `smtp_config_missing` and does not dial. The worker constructs this mailer only when `workflowdept.EmailBindingEnabled()` is true. That function still requires both parent and email flags. Code and config defaults stay false.

Timeout after DATA, or an error containing `timeout` / `deadline` / `uncertain`, maps to `SEND_UNKNOWN`. SMTP 550/551/553/554 maps to `PERMANENT_FAILED`. 421/450/451/452 stays `RETRYABLE_FAILED` until attempt 5.

## Duplicate guard

Nothing inserts binding rows today. Legacy dispatch and binding claim are different tables, so an occurrence could still be emailed by both once a resolution row exists. Before SMTP, `DispatchDueOccurrences` now skips the legacy sender when both flags are on and `BindingOwnsOccurrence` is true. A lookup error, including a missing table, does not suppress legacy mail. Flags off never calls the lookup.

## Tests

| Command | Result |
|---|---|
| `go test ./internal/workflowdept/...` | PASS |
| `go test ./internal/reminder/...` | PASS |
| `go test ./internal/notification/infra/smtp/` | PASS |
| `go test ./migrations/` | PASS |
| `go build ./...` | PASS |
| `go test ./internal/notification/...` | FAIL `TestContract_VariableParity/workflow.approved` — pre-existing |
| `go test ./...` | not re-run this pass; earlier run failed outside this diff in `companyaccess`, `legal_basis_backfill`, `httpserver`, `platform/config` |
| MySQL concurrent claim | PASS on the earlier isolated DEV schema (`-count=20`) |
| Real SMTP smoke | NOT_RUN — no approved test recipient, and the shared DEV table is absent |

## Rollback

Leave both binding flags unset. The deployed worker already has them unset, health stayed 200, and no binding SMTP call was made. Do not run down migrations to roll this back.
