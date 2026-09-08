# Phase C.1 Evidence — 08 Signing Secret Source Trace

## Source Trace
- **File**: `internal/disclosure/app/template_import_token.go`
- **Function**: `ResolveTemplateImportSigningSecret(explicitSecret string) string`
- **Previous implementation**:
  Checked `CMS_TEMPLATE_IMPORT_SIGNING_SECRET`, then fell back to `CMS_MEDIA_UPLOAD_SIGNING_SECRET`.
- **Identified Risk**:
  Cross-domain security coupling: media upload signer could issue valid template import tokens if the same secret was reused.
- **Decision**:
  Enforce strict dedicated signing secret for template import.
