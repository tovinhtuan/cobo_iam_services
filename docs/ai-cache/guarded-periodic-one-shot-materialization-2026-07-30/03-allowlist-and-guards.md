# Allowlist and guards

## Allowlist (hardcoded)

| Field | Value |
|-------|-------|
| type_id | qa-monthly-deadline-alert-202607-1785382733 |
| company_id | c_001 |
| period | 2026-07 |

## Environment

- environment = DEV
- database = cobo_iam
- host ∈ {127.0.0.1, localhost, mysql, cobo-iam-mysql}
- port = 3306

## Preconditions asserted

- template active, version 1, MONTHLY, PERIODIC, days=23 WORKING_DAYS
- company c_001 applicable
- calculator due = 2026-07-31
- refuse on drift / wrong scope / non-DEV
