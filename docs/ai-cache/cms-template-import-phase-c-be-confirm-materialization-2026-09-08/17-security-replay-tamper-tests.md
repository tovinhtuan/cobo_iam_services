# Security, Replay & Tamper Test Suite

## Test Coverage Matrix
Suite: `TestTemplateImportConfirm_TokenAndPayloadTamperTests` & `TestTemplateImportConfirm_SigningSecretFailClosed` in `internal/disclosure/app/template_import_confirm_test.go`.

| Test ID | Scenario | Expected Outcome | Result |
|---|---|---|---|
| TP1 | Original valid token + original valid payload | 201 Created | PASS |
| TP2 | Template name modified after Validate | 400 Bad Request / 0 DB writes | PASS |
| TP3 | Deadline rule modified after Validate | 400 Bad Request / 0 DB writes | PASS |
| TP4 | Workflow step assignee role modified after Validate | 400 Bad Request / 0 DB writes | PASS |
| TP5 | Workflow step document requirement removed | 400 Bad Request / 0 DB writes | PASS |
| TP6 | Token issued to Actor A, confirm attempted by Actor B | 400 Bad Request / 0 DB writes | PASS |
| TP7 | Expired token (> 15 minutes) | 400 Bad Request / 0 DB writes | PASS |
| TP8 | Tampered token signature | 400 Bad Request / 0 DB writes | PASS |
| TP9 | Wrong schema version | 400 Bad Request / 0 DB writes | PASS |
| TP10 | Empty / unconfigured HMAC signing secret | Fail closed / 0 DB writes | PASS |

- Verdict: `BUSINESS_PAYLOAD_TAMPER_TESTS = PASS`, `TOKEN_SECURITY_TESTS = PASS`, `SIGNING_SECRET_FAIL_CLOSED = true`.
