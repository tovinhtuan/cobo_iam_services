# Phase B Handoff & Source Trace

## Baseline Inherited from Phase B
- Endpoint: `POST /api/v1/platform/cms/templates/import/validate` (multipart/form-data, max 2 MiB, strict JSON decode).
- Normalization: `NormalizeTemplateImportV1` produces deterministic `TemplateImportDefinitionV1`.
- Token: Stateless HMAC SHA-256 token binding canonical payload hash, actor ID, purpose (`template_import`), and schema version (`1.0`).
- Token Secret: `CMS_TEMPLATE_IMPORT_SIGNING_SECRET` (fail closed if missing).
- State: Zero DB writes during Validate.

## Trace to Phase C Entry Points
- Contracts in `internal/disclosure/app/contracts.go`:
  - `ConfirmTemplateImport(ctx context.Context, req ConfirmTemplateImportRequest) (*ConfirmTemplateImportResponse, error)`
- HTTP Route in `internal/disclosure/transport/http/handler.go`:
  - `mux.HandleFunc("POST /api/v1/platform/cms/templates/import/confirm", h.cmsConfirmTemplateImport)`
- Handler Implementation in `internal/disclosure/transport/http/import_template_handler.go`:
  - `cmsConfirmTemplateImport(w http.ResponseWriter, r *http.Request)`
- Service Implementation in `internal/disclosure/app/template_import_confirm.go`:
  - `ConfirmTemplateImport(ctx context.Context, req ConfirmTemplateImportRequest)`
  - `materializeImportUpsert(ctx context.Context, req ConfirmTemplateImportRequest, resolvedSteps []WorkflowStepDTO)`
