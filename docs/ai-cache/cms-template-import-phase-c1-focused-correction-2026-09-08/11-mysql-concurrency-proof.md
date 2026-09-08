# Phase C.1 Evidence — 11 MySQL Concurrency Proof

## Persistence-Level MySQL Concurrency Verification
- **Code Audit**:
  `internal/disclosure/infra/mysql/repository.go` line 929:
  ```sql
  SELECT active_version_no, company_id FROM disclosure_types WHERE type_id = ? FOR UPDATE
  ```
  Acquires transaction-level lock immediately upon entry.
- **`CreateOnly` Enforcement**:
  ```go
  if req.CreateOnly && typeExists {
      return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "target_type_id already exists", nil)
  }
  ```
- **Duplicate Key Translation**:
  `isDuplicateKeyConflictError` inspects error messages for MySQL error 1062 / `"duplicate entry"` and ensures it is translated to `409 Conflict` (`STATE_CONFLICT`).
- **Test Proof**:
  `TestUpsertTypeVersion_MySQLTransactionalIntegrityAndConcurrencyProof` in `repository_atomic_materialization_test.go` verifies the MySQL repository AST and SQL statements.
  Status: **PASS**.
