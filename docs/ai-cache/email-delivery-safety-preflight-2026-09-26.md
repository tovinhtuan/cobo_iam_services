# Email delivery safety — preflight (2026-09-26)

## Live preflight

DEV MySQL `8.0.46`, database `cobo_iam`, host documented as `88.216.208.0` (`avi-server1`). `/healthz` and `/readyz` were 200. Local `.env` was not loaded.

`schema_migrations` for this database includes `0140` through `0143` and does not include `0144`, `0145`, `0146`, or `0147`. `reminder_dispatch_resolutions` does not exist, so there are no delivery rows, duplicate groups, provider mismatches, or stale `SENDING` leases to report. No recipient text was selected.

```text
LIVE_PREFLIGHT=PASS
```

Recipient plaintext was not read and is not recorded here. A dotenv file exists in the repo directory and was not loaded, so this report does not treat that file as a live connection.

## Static schema (from `migrations/0144_workflow_department_binding.up.sql`)

Table `reminder_dispatch_resolutions` already has:

- `resolution_id` PK
- `occurrence_id` UNIQUE (`uk_reminder_dispatch_occurrence`)
- `recipient_email_ciphertext` BLOB NULL
- `email_key_version` INT UNSIGNED NULL
- `send_status` VARCHAR(32) NOT NULL
- `provider_message_id` VARCHAR(255) NULL
- index `idx_reminder_dispatch_send (send_status, resolved_at)`

Not present before `0146`:

- `lease_id`
- `lease_until`
- `updated_at`
- `attempt_count`
- `next_retry_at`
- `last_error_code`

`migrations/run_dev_migrations.sh` lists files through `0145_catalog_department_code_guard.up.sql`. `0146` and `0147` are absent until this implementation adds them.

## Duplicate / unique index

Static schema already rejects a second row with the same `occurrence_id`. A live duplicate count was not run. `0147` must count duplicate `occurrence_id` values inside the migration and must not create its own unique index when that count is greater than zero.

## Status

Static preflight is enough to name the missing columns. It is not a PASS for production data volume, stale `SENDING` rows, ciphertext without `email_key_version`, or provider ids that are not `SENT`.
