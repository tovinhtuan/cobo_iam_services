# CMS Template Import History Check

Created: 2026-09-26
Scope: `cobo_iam_services` + `cobo_web_design`, branch `fixbug/test-9`

## Verdict

There is no dedicated UI tab or API for file-import history.

The CMS editor has a tab labeled `Lịch sử bản ghi CMS`, implemented by
`cobo_web_design/src/features/cms-core/templates/TemplateRecordHistoryTab.tsx`.
It lists global CMS records for a template through the records API; it does
not list import attempts, uploaded filenames, validate results, mapping
failures, or confirm attempts.

The import UI currently supports upload, validate, mapping, and confirm only:

- `POST /api/v1/platform/cms/templates/import/validate`
- `POST /api/v1/platform/cms/templates/import/confirm`

The backend writes an audit event `disclosure.type.import` only after a
successful confirm. A validate-only or mapping-blocked attempt does not create
an import-history record visible in the CMS UI.

## Evidence

- `TemplatesFeatureScreen.tsx` routes to `ImportTemplateScreen` for import.
- `templateEditorViewModel.ts` exposes `history` as `Lịch sử bản ghi CMS`.
- `TemplateRecordHistoryTab.tsx` calls `listByTemplate`, which is record
  history, not import history.
- `importTemplateApi.ts` exposes only validate, confirm, and example download.
- `import_template_handler.go` emits `disclosure.type.import` on confirm.
- `/api/v1/platform/cms/ops/audit` exists as a general audit endpoint, but no
  dedicated import-history tab/filter was found in the frontend import flow.

## Current DEV case

The annual-report file was validated successfully but blocked before confirm by
unresolved department mappings. Therefore no Draft and no successful import
audit event should exist for that attempt.

**Cached for:** Team reuse, CMS reviews, and follow-up product planning.
