# Token & Hash Verification

## Verification Sequence
Before any database query or write, `ConfirmTemplateImport` verifies the validation token and payload hash:

1. **Token Structural Parse**:
   - Token must be formatted as `<claimsB64>.<signatureB64>`.
2. **Signature Verification**:
   - HMAC SHA-256 computed using secret resolved via `ResolveTemplateImportSigningSecret("")`.
   - Compares computed signature with token signature using `hmac.Equal`.
   - In case of signature alteration or tampering, verification fails immediately.
3. **Purpose Verification**:
   - Token claims `purpose` must strictly match `template_import`.
4. **Schema Version Verification**:
   - Token claims `schema_version` must equal `1.0`.
5. **Expiration Verification**:
   - Current time must not exceed `claims.ExpiresAt` (TTL is 15 minutes).
6. **Actor Binding Verification**:
   - Token claims `actor_id` must match authenticated `req.Subject.UserID`.
   - Prevents token theft across platform admin sessions.
7. **Canonical Payload Hash Verification**:
   - Strongly typed `req.NormalizedTemplate` is deterministically serialized and hashed with SHA-256 via `ComputeCanonicalTemplatePayloadHash`.
   - Compared against `claims.PayloadHash`.
   - Any tampering with business fields (name, description, workflow steps, documents, deadline rules, applicability dates, tags) invalidates the hash and results in HTTP 400 Bad Request with ZERO DB writes.
8. **Target Name Authority**:
   - `TARGET_NAME_CONFIRM_AUTHORITY = MUST_EQUAL_NORMALIZED_NAME`.
   - If `req.TargetName` differs from `req.NormalizedTemplate.Name`, request is rejected with HTTP 400 Bad Request.
