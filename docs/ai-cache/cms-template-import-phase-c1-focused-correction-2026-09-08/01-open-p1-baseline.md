# Phase C.1 Evidence — 01 Open P1 Baseline

## Baseline Audit of Open Items
1. **P1-01 ApplicabilityRules Fidelity**:
   - Issue: Materialization mapper previously applied `applicability.DefaultGlobalRules(...)` unconditionally. `applicability_rules` was missing from JSON schema and import Go contracts.
   - Status: CLOSED in Phase C.1.
2. **P1-02 Import Token HTTP Contract Drift**:
   - Issue: Phase A documented 422 `INVALID_IMPORT_TOKEN` for invalid/tampered/expired tokens, while Phase C returned 400.
   - Status: CLOSED in Phase C.1 by standardizing on 422 `INVALID_IMPORT_TOKEN`.
3. **P1-03 Full Importable Field Roundtrip Proof**:
   - Issue: Previous tests only verified subsets of fields. A single comprehensive roundtrip test verifying all 34 V1 importable business fields was required.
   - Status: CLOSED in Phase C.1.
4. **P1-04 Signing Secret Fallback Contract**:
   - Issue: Previous code fell back to `CMS_MEDIA_UPLOAD_SIGNING_SECRET` if `CMS_TEMPLATE_IMPORT_SIGNING_SECRET` was empty.
   - Status: CLOSED in Phase C.1 by removing fallback and enforcing strict dedicated secret.
5. **GAP-01 MySQL Concurrency & Rollback Proof**:
   - Issue: Concurrency and rollback tests used in-memory repository; persistence-level MySQL transaction boundary and error 1062 translation needed verification.
   - Status: CLOSED in Phase C.1.
