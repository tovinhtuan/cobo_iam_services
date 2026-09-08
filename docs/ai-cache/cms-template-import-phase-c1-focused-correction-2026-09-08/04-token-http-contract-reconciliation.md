# Phase C.1 Evidence — 04 Token HTTP Contract Reconciliation

## Authoritative HTTP Contract Lock
- **400 Bad Request** (`INVALID_REQUEST`):
  - Malformed JSON syntax in Confirm body
  - Missing mandatory fields: `validation_token`, `target_type_id`, `target_name`
  - `target_name` mismatch against normalized template name
  - Unmapped department references or invalid display group codes
- **401 Unauthorized**: Missing session
- **403 Forbidden** (`PERMISSION_DENIED`): Missing CMS permission
- **422 Unprocessable Entity** (`INVALID_IMPORT_TOKEN`):
  - Invalid token structure
  - Tampered HMAC signature
  - Expired token (> 15 minutes)
  - Token actor mismatch
  - Wrong token purpose
  - Wrong schema version
  - Canonical payload hash mismatch
- **409 Conflict** (`STATE_CONFLICT`):
  - `target_type_id` collision in database

## Parity Proof
- `CodeInvalidImportToken = "INVALID_IMPORT_TOKEN"` defined in `internal/platform/errors/errors.go`.
- `TestTemplateImportConfirm_TokenHTTPContract` asserts exact status, exact perr.Code, and zero DB writes for every case.
- HTTP handler tests updated in `import_template_handler_test.go`.
- Phase A and Phase D documentation synchronized.
