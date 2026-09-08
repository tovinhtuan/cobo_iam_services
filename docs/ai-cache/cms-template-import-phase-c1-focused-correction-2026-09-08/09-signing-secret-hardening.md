# Phase C.1 Evidence — 09 Signing Secret Hardening

## Hardened Implementation
1. **Fallback Removed**:
   `ResolveTemplateImportSigningSecret` now only reads `CMS_TEMPLATE_IMPORT_SIGNING_SECRET`. Zero fallback to `CMS_MEDIA_UPLOAD_SIGNING_SECRET`.
2. **Fail-Closed Guarantees**:
   If `CMS_TEMPLATE_IMPORT_SIGNING_SECRET` is unset:
   - `NewTemplateImportSigner` constructs an empty signer.
   - `IssueToken` returns an error: `"CMS template import signing secret is not configured: fail closed"`.
   - `VerifyToken` returns an error: `"CMS template import signing secret is not configured: fail closed"`.
   - Confirm service halts immediately and writes 0 database records.
3. **Dedicated Test**:
   `TestTemplateImportSigningSecret_NoMediaFallback` in `template_import_confirm_test.go` verifies:
   - Media secret ignored when import secret unset.
   - Fail closed behavior verified.
   - Status: **PASS**.
