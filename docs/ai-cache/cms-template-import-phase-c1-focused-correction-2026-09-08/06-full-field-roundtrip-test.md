# Phase C.1 Evidence — 06 Full Field Roundtrip Test

## Test Architecture
- **Test Function**: `TestTemplateImportConfirm_FullImportableFieldRoundTrip` in `internal/disclosure/app/template_import_confirm_test.go`
- **Execution Flow**:
  1. Construct rich JSON-level template definition exercising all 34 V1 fields.
  2. Normalize payload via `NormalizeTemplateImportV1`.
  3. Issue valid HMAC validation token.
  4. Confirm import with explicit department mappings.
  5. Canonical reload via `GetTypeVersionDetail`.
  6. Assert all 34 business fields match normalized expectations.

## Key Verifications
- Zero silent business field loss.
- Zero unexpected business defaulting.
- Proper handling of server-generated UUIDs (fresh block IDs, legal basis IDs, workflow step keys).
- TemplateFileID cleared to empty string (`""`) for document requirements.
- Status: **PASS**.
