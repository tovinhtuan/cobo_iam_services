# Materialization Mapper

## Responsibility
`materializeImportUpsert` in `internal/disclosure/app/template_import_confirm.go` maps a validated `ConfirmTemplateImportRequest` and resolved workflow steps into an authoritative `UpsertTypeVersionRequest`.

## Properties of Materialized Request
- `Subject`: Authenticated CMS admin context from bearer token.
- `TypeID`: `req.TargetTypeID` (URL slug / explicit identifier).
- `Scope`: `templateScopeGlobal` ("global").
- `GroupID`: `norm.GroupID` (defaulted to `group-001` periodic or `group-002` irregular).
- `Name`: `req.TargetName` (verified equal to `norm.Name`).
- `CreateOnly`: `true` (strictly creates a new template root; fails if root exists).
- `ChangeNote`: `"Imported template definition v1.0"`.
- `ApplicabilityRules`: `applicability.DefaultGlobalRules(isPeriodic)`.
- `Blocks`: 6 canonical mandatory blocks generated freshly (`legal_basis`, `disclosure_content`, `deadline`, `channels_and_format`, `legal_risks`, `enterprise_workflow`).
- `DeadlineConfig`: Transferred from normalized config, preserving `ApplicableFromMode`, `ApplicableFromSlot`, `ApplicableTo`, `FrequencyUnit`, and cycle anchors.
- `DisplayGroupCodes`: Transferred from normalized display groups.
- `ClearWorkflow`: `false` when steps are present.
- `SkipPublicationMatrix`: `false`.
