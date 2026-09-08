# Phase C.1 Evidence — 02 Applicability Rules Source Trace

## Source Trace Pipeline
1. **JSON Schema**: `template-import-v1.schema.json` -> added `applicability_rules` property with `$ref` to `TemplateApplicabilityRulesV1`.
2. **Go Contracts**: `TemplateImportDefinitionV1` in `internal/disclosure/app/template_import_contracts.go` -> added `ApplicabilityRules *applicability.TemplateApplicabilityRules`.
3. **Normalization**: `NormalizeTemplateImportV1` in `internal/disclosure/app/template_import_normalizer.go` -> deep copies company classes, business sectors, deadline structure map, and normalizes `DeadlineDayType`.
4. **Validation**: `ValidateImportTemplate` in `internal/disclosure/app/template_import_validator.go` -> calls `applicability.ValidateRules`.
5. **Confirm Request**: Carries `norm.ApplicabilityRules`.
6. **Materialization Mapper**: `materializeImportUpsert` -> if `norm.ApplicabilityRules != nil`, preserves it; otherwise falls back to `applicability.DefaultGlobalRules(isPeriodic)`.
7. **Authoritative Write Authority**: Passed into `UpsertTypeVersionRequest.ApplicabilityRules`.
8. **Persistence**: Persisted to MySQL `applicability_rules_json` column.
