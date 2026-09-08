# Phase C Context — CMS Template Import Confirm & Transactional Materialization

- **Feature**: CMS Template Import
- **Phase**: Phase C (Backend Confirm + Transactional Materialization + Token/Hash Verification + Mapping Application + Concurrency Safety + Audit + Rollback / Zero-Partial-Write Proof)
- **Repo Affected**: `cobo_iam_services` (Backend Only)
- **Frontend Impact**: `cobo_web_design` — ZERO source modifications (`FE_SOURCE_CHANGED=false`)
- **Mode**: STRICT IMPLEMENTATION MODE — BACKEND ONLY — DELTA ONLY — MINIMAL BUSINESS CONTRACT DRIFT
- **Status**: COMPLETED / PASS

## Guarantees Enforced
1. **CMS Auth Gate**: Requires `platform.cms.view` + `cms.template.write` permissions.
2. **Stateless HMAC Token & Payload Hash Binding**:
   - Signature, purpose (`template_import`), actor, expiry, schema_version (`1.0`) verified before any DB operation.
   - Recomputes canonical normalized payload SHA-256 hash and strictly matches `token.payload_hash`.
   - Business fields are immutable after Validate.
3. **Target Name Confirm Authority**:
   - `TARGET_NAME_CONFIRM_AUTHORITY = MUST_EQUAL_NORMALIZED_NAME`.
   - Confirm cannot alter target template name independently of the token-bound template name.
4. **Signing Secret Fail-Closed**:
   - Resolved from `CMS_TEMPLATE_IMPORT_SIGNING_SECRET` or `CMS_MEDIA_UPLOAD_SIGNING_SECRET`.
   - Missing secret immediately fails closed with zero DB writes.
5. **Reference Revalidation**:
   - Current catalog of template departments and display group codes re-queried during Confirm.
   - Prevents stale references from token TTL window.
6. **Department Mapping Engine**:
   - Strict mapping validation: all required steps resolved, target codes exist, no blank targets, no unreferenced ghost mapping keys.
   - Mappings applied to fresh materialization DTO without mutating canonical normalized template payload.
7. **Write Authority Reuse**:
   - Reuses authoritative `s.UpsertTypeVersion(ctx, upsert)` with `CreateOnly=true`.
   - Materializes root in `disclosure_types` (`status="active"`, `active_version_no=0`) and Draft v1 in `disclosure_type_versions` (`version_no=1`, `is_released=false`, `activated_at=NULL`).
   - Portal state: `not_active`.
   - Zero runtime occurrences, zero company overrides, zero worker alerts.
8. **Concurrency & Replay Safety**:
   - `TypeExists` precheck + DB unique constraint + duplicate error translation into HTTP 409 Conflict.
   - In 10-goroutine concurrent race test with identical `target_type_id`: exactly 1 succeeds (201), 9 return 409 Conflict.
   - Replay with same token and same target returns 409 Conflict without duplicate version/audit creation.
   - Replay with same token and different target ID is allowed by stateless contract.
9. **Rollback & Zero Partial Writes**:
   - Proven via failure injection: when transaction fails, zero roots/versions/blocks/display groups remain in DB.
   - Failed confirm does not consume token; retry succeeds after rollback.
10. **Audit Logging**:
    - Audit action: `disclosure.type.import`.
    - Model: `POST_COMMIT` best-effort.
    - Zero sensitive tokens, secrets, or raw payloads in audit metadata.
