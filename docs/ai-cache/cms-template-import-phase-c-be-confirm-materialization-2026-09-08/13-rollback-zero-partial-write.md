# Rollback & Zero-Partial-Write Proof

## Proof of Zero Partial Writes
- Suite: `TestTemplateImportConfirm_RollbackAndZeroPartialWrite`.
- Mechanism: Injected transient database error during `UpsertTypeVersion`.
- Assertions on Failure:
  - `FAILURE_BEFORE_WRITE_DB_DELTA = 0`
  - `FAILURE_DURING_ROOT_WRITE_DB_DELTA = 0`
  - `FAILURE_DURING_VERSION_WRITE_DB_DELTA = 0`
  - `FAILURE_DURING_BLOCK_WRITE_DB_DELTA = 0`
  - `FAILURE_DURING_DISPLAY_GROUP_WRITE_DB_DELTA = 0`
  - `FAILURE_DURING_AUDIT_DB_DELTA = 0`
  - `TypeExists(targetID)` returns `false`.
  - `ListTypeVersions(targetID)` returns 0 versions.
- Assertions on Retry:
  - `RETRY_AFTER_ROLLBACK = PASS`.
  - Same token and target retry succeeds with HTTP 201 Created.
  - Persisted database state: exactly 1 root and 1 version.
- Verdict: `ZERO_PARTIAL_WRITE_PROOF = PASS`.
