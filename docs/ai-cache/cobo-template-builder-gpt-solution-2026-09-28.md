# Solution: CoBo Template Builder GPT

Created: 2026-09-28
Scope: raw report description -> CMS Template Import JSON
Status: Proposed

## Recommendation

Use a hybrid design:

1. Custom GPT handles extraction, normalization, missing-information questions,
   and JSON file generation.
2. CoBo backend remains the source of truth for schema and domain validation.
3. A GPT Action calls the DEV/staging validate endpoint only.
4. The GPT cannot confirm, publish, activate, or send email.

Do not make the GPT prompt the final authority. A JSON file is importable only
after the backend returns `parse_valid=true` and `domain_valid=true`.

## User flow

```text
Upload raw report file/text
        -> extract structured facts
        -> ask for missing/ambiguous facts
        -> generate JSON draft
        -> local syntax/schema checks
        -> Action: CMS validate
        -> resolve department mappings
        -> generate downloadable JSON + validation report
```

The GPT stops before confirm. The user uploads the generated JSON into the CMS
UI and completes mapping/confirm there.

## Custom GPT configuration

### Knowledge files

- `docs/schema/template-import-v1.schema.json`
- `cobo-template-import-guide-v1.0.md`
- canonical JSON example
- approved valid/invalid fixtures
- business error-code catalog
- department mapping policy, without secrets or tenant credentials

Rules and behavior belong in GPT Instructions; Knowledge files are reference
material only.

### Capabilities

- File upload.
- Code Interpreter/Data Analysis for parsing and writing JSON.
- No web search for legal facts unless explicitly approved.
- One Action only for validate; no confirm/publish Action.

Custom GPT Actions require an OpenAPI schema and allowed domain/authentication.
Actions and Apps are alternatives and must not be assumed to work together.

## Action contract

Expose a validate-only endpoint through an allowlisted DEV/staging domain:

`POST /api/v1/platform/cms/templates/import/validate`

The Action response should expose only:

- `parse_valid`;
- `domain_valid`;
- `activation_ready`;
- `mapping_required`;
- `required_mappings`;
- `errors`;
- `warnings`;
- normalized preview.

Do not return or reveal validation tokens, bearer tokens, cookies, raw request
headers, or internal database identifiers not needed for authoring.

The Action must not expose:

- confirm endpoint;
- publish endpoint;
- activate endpoint;
- catalog mutation endpoint;
- email/SMTP operation.

## Output contract

When backend validation passes, the GPT produces:

1. Downloadable `<safe-template-id>.json` containing only the JSON object.
2. Human-readable validation summary outside the JSON.
3. Mapping checklist if department mapping is still required.
4. Explicit warnings and assumptions.

When validation fails, the GPT must not present the JSON as import-ready. It
returns the error code, JSON field path, explanation, and correction request.

## Guardrails

- Never invent legal basis, deadline, department, role, or company class.
- Never use `annually`; use `yearly` for periodic templates.
- Periodic templates use only five periodicity values.
- Periodic `deadline_rule` may be omitted when valid `deadline_days` exists.
- Irregular templates require `deadline_rule`.
- Each `file_types` token is at most 64 runes.
- Department references remain portable code/name references.
- No raw input file, credentials, tokens, or PII is persisted by the GPT Action.
- The backend validator decides final validity.

## Backend additions

Recommended API support:

- existing validate endpoint;
- guide download endpoint;
- schema/example download endpoint;
- optional `source=gpt_template_builder` request metadata, if allowed by the
  audit/privacy policy.

Do not add a GPT-specific validation implementation. Reuse the existing CMS
validator, normalizer, error codes, and mapping semantics.

## Rollout phases

### Phase 1 — Offline authoring

Custom GPT with Knowledge and Code Interpreter only. Users manually upload the
result to CMS.

### Phase 2 — DEV validate Action

Connect Action to DEV validate endpoint. Keep confirm disabled. Test malformed
JSON, periodicity, deadline, file type length, and mapping-required cases.

### Phase 3 — Internal acceptance

Use approved raw report fixtures. Verify generated JSON hash, backend validation,
mapping summary, and no sensitive data leakage.

### Phase 4 — Production decision

Only after security/privacy review decide whether to expose an authenticated
staging/production validate Action. Keep confirm inside CMS UI.

## Decision

Proceed with Custom GPT + validate-only Action. Keep all materialization and
lifecycle operations in the existing CMS UI/backend. This gives users guided
JSON authoring without allowing a language model to create or publish templates
directly.

**Cached for:** Product design, API contract review, security review, and GPT
configuration.
