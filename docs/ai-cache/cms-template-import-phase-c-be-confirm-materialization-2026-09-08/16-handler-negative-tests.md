# HTTP Confirm Handler Negative Tests

## Handler Test Suite
Suite: `TestCmsConfirmTemplateImport_E2EAndHandlerMatrix` in `internal/disclosure/transport/http/import_template_handler_test.go`.

## Scenarios Tested & Proven
1. **H1 Real Token E2E**:
   - `POST /import/validate` with valid periodic template -> 200 OK.
   - Extracts real token and normalized template.
   - `POST /import/confirm` -> 201 Created.
   - Asserts root status "active", version 1, is_active false, is_released false, portal_state "not_active".
   - Verifies audit log entry.
2. **Sequential Duplicate Replay**:
   - `POST /import/confirm` with same `target_type_id` -> 409 Conflict.
   - Verifies audit event count does not increase.
3. **Malformed JSON Body**:
   - Sends invalid JSON text -> 400 Bad Request.
4. **Missing / Invalid Auth Token**:
   - Omits `Authorization` header -> 401 Unauthorized.
5. **Tampered Validation Token**:
   - Appends invalid characters to token -> 400 Bad Request.
6. **Payload Hash Mismatch**:
   - Alters `Description` after validation -> 400 Bad Request.
7. **Target Name Mismatch**:
   - Submits `TargetName` differing from template name -> 400 Bad Request.
- Verdict: `HTTP_CONFIRM_POSITIVE_TESTS = PASS`, `HTTP_CONFIRM_NEGATIVE_TESTS = PASS`.
