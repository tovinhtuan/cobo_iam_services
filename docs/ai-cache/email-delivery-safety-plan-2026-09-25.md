# Email Delivery Safety — start gate for schema/claim/reaper (2026-09-25)

- task type: plan only
- skill: system-design-feature
- code/migration/commit/push: none

## Start slice

Schema, claim, and reaper may be implemented after a live `SHOW COLUMNS` and `SELECT file_name FROM schema_migrations`. That slice keeps both workflow-department flags false and does not send on the binding path.

## Locked decisions

- Gate is `workflowdept.EmailBindingEnabled()` (`flags.go` 22). Reminder has no caller. Do not read the email flag alone.
- Empty SMTP host returns `mock-no-smtp` (`smtp_sender.go` 87–89). Binding path must record `PERMANENT_FAILED` with a null provider id and must not call the legacy sender.
- Retry on the resolution: 1m, 3m, 6m, 10m, then attempt 5 is `PERMANENT_FAILED`. Claim does not increment `attempt_count`.
- `0146` is schema-only and re-runnable. `0147` copies keys only when still null and creates the unique index only after a clean report.
- No recipient-key env exists in `config.go` or `configs/config.example.env`. Other secrets (JWT, CMS media, login RSA) are not this key. Email runtime stays NO-GO until a 32-byte key is loaded from outside the business DB.
- Reconcile command stays unexposed. `platform.cms.view` is system-only and broad (`rbac_grant_policy.go` 45). No existing permission is scoped to delivery reconciliation. Product/Security acceptance is UNVERIFIED. `admin_doanh_nghiep` cannot change delivery.

## Still blocking flag enable

Lease above the proven 10s+30s SMTP ceiling plus buffer, encryption key loader, operator permission, mock fail-closed on the binding path, and the migration/race tests.
