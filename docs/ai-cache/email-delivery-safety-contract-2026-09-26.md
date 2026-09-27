# Email delivery safety — contract freeze (2026-09-26)

- task type: contract freeze before schema/claim/reaper code
- skill: backend-db-migration-safe + backend-worker-idempotent
- branch: `fixbug/test-9`
- flags stay **off**. This document does not enable sending.

## A. Authoritative source

`reminder_dispatch_resolutions` is the only state machine for workflow-department binding email.

`email_notifications` + `email_delivery_attempts` + `email.dispatch` stays a separate pipeline. Binding delivery must not enqueue that outbox and must not use `NextRetryDelay` from `internal/notification/app/email_delivery.go` (1m, 5m, 15m, 1h, 6h).

One row is keyed by `resolution_id`. `occurrence_id` is already unique (`uk_reminder_dispatch_occurrence` from `0144`). A second binding send for the same occurrence is not a second authoritative row.

## B. State machine

| State | SMTP | Claim | Auto-retry |
|---|---|---|---|
| `PENDING` | allowed when due | yes | n/a |
| `RETRYABLE_FAILED` | allowed when due | yes | yes, on schedule |
| `SENDING` | no (lease holder already owns it) | no while lease is valid | no |
| `SENT` | no | no | no |
| `SEND_UNKNOWN` | no | no | no |
| `PERMANENT_FAILED` | no | no | no |

Allowed:

- `PENDING` → `SENDING` (atomic claim)
- `RETRYABLE_FAILED` → `SENDING` (atomic claim, only when `next_retry_at` is due)
- `SENDING` → `SENT` (provider accepted, non-empty message id)
- `SENDING` → `SEND_UNKNOWN` (uncertain outcome, or stale lease reaper)
- `SENDING` → `RETRYABLE_FAILED` (proven transient, attempt 1–4)
- `SENDING` → `PERMANENT_FAILED` (proven permanent, empty SMTP, key/decrypt failure, or attempt 5)

Forbidden:

- `SENT` → any resend
- `SEND_UNKNOWN` → automatic resend
- `PERMANENT_FAILED` → retry
- `SENDING` → `PENDING`
- terminal → `SENDING`
- claim that increments `attempt_count`

`ClaimToSending` allows only `PENDING` and `RETRYABLE_FAILED`. The SQL claim is the authority. A lost race is `already_owned` or `already_terminal`, not a system error. Only outcome `claimed` may call SMTP.

## C. Atomic claim

Columns added by `0146` (absent on `0144`): `lease_id`, `lease_until`, `updated_at`, `attempt_count`, `next_retry_at`, `last_error_code`.

Specific id:

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

`next_retry_at` is an extra due-guard so a scheduled retry is not claimed early. Due-scan uses the same predicate with `ORDER BY resolved_at LIMIT 1`, then reads the row by `lease_id`. `RowsAffected = 0` is not an error.

Lease length is 90s, above the existing SMTP ceiling (dial 10s + operation 30s) plus buffer. Times are UTC.

## D. Retry (binding path only)

Claim does not increment `attempt_count`. The count increments only when a failed attempt is recorded.

| Failure just recorded | Next |
|---|---|
| attempt 1 | `RETRYABLE_FAILED` after 1m |
| attempt 2 | `RETRYABLE_FAILED` after 3m |
| attempt 3 | `RETRYABLE_FAILED` after 6m |
| attempt 4 | `RETRYABLE_FAILED` after 10m |
| attempt 5 | `PERMANENT_FAILED` |

## E. Empty SMTP host (binding path only)

When the binding tick runs and `SMTP_HOST` is empty:

- do not call `reminder/infra/email.Sender.SendReminderEmail`
- do not treat `mock-no-smtp` as success
- `send_status = PERMANENT_FAILED`
- `provider_message_id = NULL`
- `last_error_code = smtp_host_empty`

Legacy reminder sender stays `mock-no-smtp` while binding flags are off. That function is not edited.

## F. Encryption key

- Source: env `WORKFLOW_DEPARTMENT_EMAIL_RECIPIENT_KEY` (standard base64 of exactly 32 bytes). Not stored in MySQL. Not taken from JWT, CMS media, or login RSA keys.
- Version: env `WORKFLOW_DEPARTMENT_EMAIL_RECIPIENT_KEY_VERSION` (uint32). Ciphertext prefix and `email_key_version` must match it.
- Rotation in this slice: one active key. A blob whose version differs fails decrypt and the row becomes `PERMANENT_FAILED`. No second key is loaded and no key is written into the business DB.
- Missing or wrong-length key: fail closed, no SMTP, `last_error_code = recipient_key_missing`.
- Logs may include `resolution_id` and `last_error_code`. Logs must not include the key or recipient plaintext.

Runtime send stays NO-GO until a real 32-byte key is provisioned outside this repo. This task does not set that env.

## G. Permission

No reconciliation HTTP route. `platform.cms.view` is system-wide and too broad (`rbac_grant_policy.go`). No existing permission is scoped to delivery repair. `admin_doanh_nghiep` cannot change delivery state.

Internal repository/worker code only. Operator repair stays a blocker until Product/Security names a dedicated permission.

## Provider outcomes

| Outcome | State | Auto-retry |
|---|---|---|
| accepted + non-empty provider id | `SENT` | no |
| timeout / uncertain (message may exist) | `SEND_UNKNOWN` | no |
| proven transient | `RETRYABLE_FAILED` or attempt-5 `PERMANENT_FAILED` | schedule only |
| proven permanent SMTP | `PERMANENT_FAILED` | no |
| transport not wired | `PERMANENT_FAILED` (`smtp_transport_unwired`) | no |

Unwired transport is intentional in this slice so flag-on still cannot dial a real MTA. That keeps the flag-enable gate closed.

## Out of scope

- Enabling any `WORKFLOW_DEPARTMENT_*` flag
- Editing `0144`
- Frontend
- Production migration
- Replacing the global email pipeline
