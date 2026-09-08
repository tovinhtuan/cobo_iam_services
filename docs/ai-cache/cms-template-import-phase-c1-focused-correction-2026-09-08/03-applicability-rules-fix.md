# Phase C.1 Evidence — 03 Applicability Rules Fix

## Implementation Details
1. **Schema Update**:
   - `cobo_iam_services/docs/schema/template-import-v1.schema.json`
   - `cobo_web_design/docs/schema/template-import-v1.schema.json`
   - Added `applicable_company_classes`, `applicable_sectors`, `deadline_by_structure`, `deadline_days`, `deadline_day_type`, `use_structure_deadline`.
2. **Go DTO**:
   - `TemplateImportDefinitionV1.ApplicabilityRules *applicability.TemplateApplicabilityRules`
3. **Normalizer**:
   - Handles `nil` gracefully.
   - Deep copies slices and structure deadline entries.
4. **Confirm Mapper**:
   - Preserves normalized rules if present.
   - Defaults only when omitted.
5. **Targeted Tests**:
   - `TestTemplateImportConfirm_ApplicabilityRulesRoundTrip` (A1 - A5 subtests: omitted defaults, explicit non-default preserved, periodic preserved, irregular preserved, malformed blocked).
   - All tests PASS.
