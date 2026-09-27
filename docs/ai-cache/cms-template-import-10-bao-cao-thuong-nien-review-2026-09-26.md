# CMS template import review — 10_bao-cao-thuong-nien.json (2026-09-26)

## Findings

1. The pasted file is not a JSON document as uploaded. It contains the filename, a progress marker (`100%`) before the root object, and a trailing display line after the closing brace. The import parser uses strict JSON decoding, so this fails as `INVALID_JSON_PAYLOAD` before domain validation.
2. After extracting only the object between the opening and closing braces, the payload still has `template.periodicity = "annually"`. CMS accepts `daily`, `weekly`, `monthly`, `quarterly`, or `yearly`; `annually` produces `INVALID_PERIODICITY` and blocks import.
3. `deadline_config.applicable_from_mode = "NEXT_SLOT"` with `applicable_from_slot = "12-31"` is normalized by the importer: the slot is cleared for NEXT mode. It is not the current parse blocker, but the source should omit `applicable_from_slot` or use it only with `SPECIFIC_SLOT`.
4. Department references (`corporate_secretary`, `legal`, `bod`) are portable. If the tenant CMS catalog does not contain matching codes/names, import returns required department mappings and `can_confirm=false`; this is environment-dependent, not a JSON syntax error.

## Corrective actions

- Upload a file containing only the JSON object; remove the filename/progress/trailing UI text.
- Change `template.periodicity` from `annually` to `yearly`.
- Prefer removing `applicable_from_slot` while using `NEXT_SLOT`.
- Confirm that the target CMS department catalog contains the three department codes or provide mappings during import confirmation.

## Docs consulted

- `docs/schema/template-import-v1.schema.json`
- `internal/disclosure/app/template_import_service.go`
- `internal/disclosure/app/template_import_validator.go`
- `internal/disclosure/app/template_import_normalizer.go`
- `internal/disclosure/app/applicability/validate.go`

**Cached for:** Team reuse and future CMS import troubleshooting.
