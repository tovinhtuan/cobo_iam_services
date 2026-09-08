# Phase C.1 Evidence — 14 Regression Tests

## Regression Test Results
1. **Targeted Tests**:
   - `TestTemplateImportConfirm_ApplicabilityRulesRoundTrip`: PASS (A1-A5).
   - `TestTemplateImportConfirm_TokenHTTPContract`: PASS (12 cases).
   - `TestTemplateImportConfirm_FullImportableFieldRoundTrip`: PASS (34 fields).
   - `TestTemplateImportSigningSecret_NoMediaFallback`: PASS.
   - `TestUpsertTypeVersion_MySQLTransactionalIntegrityAndConcurrencyProof`: PASS.
2. **Phase C Security & Materialization Regression**:
   - `TestTemplateImportConfirm_HappyPathVariants` (H1-H8): PASS.
   - `TestTemplateImportConfirm_TokenAndPayloadTamperTests` (TP2-TP8): PASS.
   - `TestTemplateImportConfirm_ReplaySemantics`: PASS.
   - `TestTemplateImportConfirm_DepartmentMappingMatrix` (M4-M9): PASS.
   - `TestTemplateImportConfirm_DisplayGroupRevalidation`: PASS.
3. **Disclosure Module Regression**:
   - Command: `go test -v ./internal/disclosure/...`
   - Result: 100% PASS (0 failures across all disclosure subpackages).
