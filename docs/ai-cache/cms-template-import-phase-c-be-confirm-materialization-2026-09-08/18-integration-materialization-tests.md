# Integration Materialization Test Matrix

## Happy-Path Variants
Suite: `TestTemplateImportConfirm_HappyPathVariants` in `internal/disclosure/app/template_import_confirm_test.go`.

| ID | Description | Verified Attributes | Result |
|---|---|---|---|
| H1 | Periodic template materialization | Quarterly frequency, NEXT_SLOT, 2028-12-31 applicable_to, draft v1 | PASS |
| H2 | Irregular template materialization | Event hours, irregular deadline rule, draft v1 | PASS |
| H3 | Workflow multi-step with exact department matches | 2 steps, dept-001 & dept-003 preserved, fresh distinct UUIDs | PASS |
| H4 | Workflow with explicit Confirm mapping | Unmatched source dept mapped to dept-001 via `DepartmentMappings` | PASS |
| H5 | Document metadata & empty template_file_id | Name & required preserved, template_file_id strictly empty, fresh UUID | PASS |
| H6 | Valid past ApplicableTo persists Draft v1 | ApplicableTo "2020-01-01" preserved in Draft v1, does not block import | PASS |
| H7 | WORKING_DAYS duration type preserved | Duration type "WORKING_DAYS" persisted accurately | PASS |
| H8 | SPECIFIC_SLOT preserved | Mode "SPECIFIC_SLOT", slot "2026-Q1" persisted without reset | PASS |

- Verdict: `MATERIALIZATION_TESTS = PASS`.
