# Phase C.1 Evidence — 12 Rollback Test Harness Audit

## Harness Reality Check
1. **Service Layer Test**:
   `TestTemplateImportConfirm_RollbackAndZeroPartialWrite` in `template_import_confirm_test.go`:
   - Repository: `failingSpyRepo` wrapping `inmemory.Repository`.
   - Injected error: Simulated DB transaction failure during materialization.
   - Proves: On repository error, service returns error, zero records remain in memory, and retry with the exact same token succeeds (proving stateless token is not burned).
2. **Persistence Layer Verification**:
   - Repository: `internal/disclosure/infra/mysql/repository.go`.
   - Mechanism: Explicit `BeginTx(ctx, nil)` with deferred `tx.Rollback()`. Only commits upon `tx.Commit()` at the end.
