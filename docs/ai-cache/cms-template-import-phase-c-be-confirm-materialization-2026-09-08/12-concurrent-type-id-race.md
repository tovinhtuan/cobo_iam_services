# Concurrent Type ID Race Safety

## Concurrency Protection Architecture
- `TYPE_ID_PRECHECK = PASS`: Initial check via `TypeExists(target_type_id)`.
- `DB_UNIQUE_CONSTRAINT_FINAL_AUTHORITY = true`: Primary key on `disclosure_types.type_id` is the final authoritative concurrency arbiter.
- `DUPLICATE_DB_ERROR_TRANSLATED_TO_409 = true`: Duplicate entry errors (MySQL Error 1062) are mapped into `409 Conflict` (Code `STATE_CONFLICT`), preventing 500 error leak.
- No application-level mutexes or in-memory locking used.

## Concurrency Race Test Execution
- Suite: `TestTemplateImportConfirm_ConcurrencyRace` in `internal/disclosure/app/template_import_confirm_test.go`.
- Configuration:
  - `CONCURRENT_REQUEST_COUNT = 10`
  - Same token, same payload, same actor, identical `target_type_id: "concurrent-target-race"`.
- Results:
  - `CONCURRENT_SUCCESS_COUNT = 1`
  - `CONCURRENT_CONFLICT_COUNT = 9`
  - `CONCURRENT_OTHER_ERROR_COUNT = 0`
  - Persisted database state: exactly 1 root in `disclosure_types`, exactly 1 version in `disclosure_type_versions`.
- Verdict: `CONCURRENT_TYPE_ID_RACE_TEST = PASS`.
