# Audit Event Atomicity & Behavior

## Audit Architecture
- **Model**: `AUDIT_ATOMICITY_MODEL = POST_COMMIT`.
- In `cobo_iam_services`, audit logging (`h.auditLog`) occurs after the database transaction has successfully committed.
- `IMPORT_AND_AUDIT_ATOMIC = false`: Best-effort post-commit call.
- `AUDIT_FAILURE_CAN_LEAVE_MATERIALIZED_TEMPLATE = true`: If audit logging fails after commit, the materialized template remains in the database.
- `AUDIT_FAILURE_POLICY = BEST_EFFORT_NON_BLOCKING`: Post-commit audit errors are logged via slog but do not transform a successful 201 creation into a 500 error, avoiding client ambiguity upon retry.

## Audit Event Contracts
- **Action**: `IMPORT_AUDIT_ACTION = "disclosure.type.import"`
- **ResourceType**: `"disclosure_type"`
- **ResourceID**: `resp.TypeID`
- **Metadata**:
  - `creation_mode`: `"TEMPLATE_IMPORT"`
  - `target_type_id`: `resp.TypeID`
  - `version_no`: 1
  - `schema_version`: `"1.0"`
  - `payload_hash`: Canonical SHA-256 payload hash
  - `actor_id`: Authenticated user ID (`sub.UserID`)
- **Counts**:
  - `SUCCESS_AUDIT_EVENT_COUNT = 1`
  - `FAILED_CONFIRM_SUCCESS_AUDIT_COUNT = 0`
  - `DUPLICATE_REPLAY_SUCCESS_AUDIT_COUNT = 0` (no additional audit on 409 Conflict)
