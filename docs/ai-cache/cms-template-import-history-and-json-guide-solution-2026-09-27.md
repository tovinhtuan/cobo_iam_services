# Solution: CMS Template Import History + JSON Authoring Guide

Created: 2026-09-27
Scope: `cobo_iam_services` + `cobo_web_design`, branch `fixbug/test-9`
Status: Proposed solution; no implementation or migration applied

## 1. Problem and goal

The current CMS import flow supports file upload, validate, department
mapping, and confirm. It does not expose a dedicated import history. The
existing `Lịch sử bản ghi CMS` tab is record history after a template exists;
it is not import-attempt history.

The CMS also offers a canonical example JSON, but does not provide a dedicated
downloadable authoring guide explaining the schema, domain rules, mapping
requirements, and common invalid-file cases.

Goals:

1. Add a platform-CMS import history view covering validate and confirm
   attempts, including blocked and failed attempts.
2. Add a versioned downloadable Markdown guide for creating valid JSON import
   files.
3. Keep raw JSON, validation tokens, credentials, and Authorization headers out
   of history and logs.
4. Preserve the current validate -> mapping -> confirm contract and existing
   `Lịch sử bản ghi CMS` behavior.

Non-goals:

- No re-import/retry action in the first version.
- No raw uploaded-file storage.
- No production rollout, flag enablement, or migration execution as part of
  this design.

## 2. Architecture decision

Use a dedicated append/update table for import attempts rather than deriving a
history screen from the generic audit feed.

Reason: the history needs validate-only states, mapping blockers, parse/domain
error summaries, file hash, filename, and lifecycle timestamps. The generic
audit feed currently records `disclosure.type.import` after successful confirm
and is not a stable import-attempt domain contract.

The successful confirm should continue writing the existing audit event. The
new import-attempt row is complementary, not a replacement for audit logs.

## 3. Import-attempt lifecycle

```text
VALIDATION_FAILED
        │
        ├── MAPPING_REQUIRED ──> VALIDATED ──> CONFIRMED
        │                            │             │
        │                            └─────────────┘
        └── VALIDATED ───────────────> CONFIRM_FAILED
```

Recommended statuses:

- `VALIDATION_FAILED`: malformed JSON, schema, or domain validation failed.
- `MAPPING_REQUIRED`: validation succeeded but one or more catalog mappings
  are unresolved.
- `VALIDATED`: validation succeeded and confirm prerequisites are satisfied.
- `CONFIRMED`: Draft/template was created successfully.
- `CONFIRM_FAILED`: confirm was attempted but failed; retain safe error code.

`activation_ready` must not be interpreted as `can_confirm`; the UI must show
mapping blockers separately.

## 4. Backend data model

Proposed table: `cms_template_import_attempts` (expand-only migration).

Fields:

- `id` — server UUID/ULID, primary key.
- `company_id` — tenant/platform scope used by existing CMS subject context.
- `actor_user_id`, `actor_membership_id` — initiating actor.
- `filename` — basename only, length-limited; never a path.
- `file_size_bytes` — bounded integer.
- `payload_sha256` — hash only; never persist raw payload.
- `schema_version` — nullable when parsing fails.
- `status` — constrained string/enum.
- `parse_valid`, `domain_valid`, `activation_ready`, `can_confirm` — snapshot of
  the validation response.
- `error_codes` — bounded JSON array of stable error codes, no raw payload.
- `required_mapping_count`, `resolved_mapping_count` — numeric summary.
- `target_type_id` — nullable; only set after confirm request validation.
- `created_type_id` — nullable; set only after successful confirm.
- `created_at`, `validated_at`, `confirmed_at`, `updated_at`.

Indexes:

- `(company_id, created_at, id)` for cursor pagination.
- `(company_id, status, created_at)` for filters.
- `(company_id, payload_sha256, created_at)` for forensic lookup without
  exposing content.

Retention should be configurable and documented. Do not add a unique constraint
  on payload hash because users may intentionally validate the same file more
  than once.

## 5. API contract

### 5.1 Validate

Keep:

`POST /api/v1/platform/cms/templates/import/validate`

Add a response field:

```json
{
  "import_attempt_id": "uuid",
  "parse_valid": true,
  "domain_valid": true,
  "activation_ready": true,
  "mapping_required": true,
  "can_confirm": false,
  "required_mappings": [],
  "errors": [],
  "warnings": []
}
```

The response remains backward compatible; `import_attempt_id` is additive.
The service creates/updates the attempt record without storing the payload.

### 5.2 Confirm

Keep:

`POST /api/v1/platform/cms/templates/import/confirm`

Require the existing validation token and bind it to `import_attempt_id`.
Update the attempt row in the same logical success/failure boundary as the
existing materialization result:

- success: `CONFIRMED`, `created_type_id` set;
- invalid token, stale validation, missing mapping, conflict, or transaction
  failure: `CONFIRM_FAILED` with stable error code and no partial import.

Do not expose token material in history.

### 5.3 List history

Add:

`GET /api/v1/platform/cms/templates/import-history`

Query parameters:

- `status` — optional enum filter;
- `filename` — optional safe basename search;
- `payload_sha256` — optional exact hash lookup;
- `from`, `to` — ISO-8601 date-time range;
- `cursor` — opaque cursor;
- `limit` — bounded, default 20, maximum 100.

Response envelope:

```json
{
  "data": {
    "items": [
      {
        "id": "uuid",
        "filename": "10_bao-cao-thuong-nien.fixed.json",
        "payload_sha256": "redacted-or-full-hash-by-product-policy",
        "status": "MAPPING_REQUIRED",
        "parse_valid": true,
        "domain_valid": true,
        "activation_ready": true,
        "can_confirm": false,
        "required_mapping_count": 3,
        "resolved_mapping_count": 0,
        "target_type_id": null,
        "created_type_id": null,
        "created_at": "2026-09-27T00:00:00Z",
        "updated_at": "2026-09-27T00:00:00Z"
      }
    ]
  },
  "meta": { "nextCursor": null },
  "error": null
}
```

Prefer returning a short hash fingerprint in the UI while allowing exact hash
filtering. Product/security review must decide whether the full hash is shown.

### 5.4 Detail

Add:

`GET /api/v1/platform/cms/templates/import-history/:id`

Return safe error-code summaries, mapping source IDs, timestamps, actor display
identity subject to current privacy policy, and linked `created_type_id` when
confirmed. Never return raw JSON, validation token, password, cookie, or bearer
token.

### 5.5 Download authoring guide

Add:

`GET /api/v1/platform/cms/templates/import/guide`

Response:

- `Content-Type: text/markdown; charset=utf-8`
- `Content-Disposition: attachment; filename="cobo-template-import-guide-v1.0.md"`
- auth-gated with the same CMS access policy as the canonical example;
- server-owned, versioned, embedded artifact;
- zero database writes.

Keep the existing example endpoint and add a visible link/button for both:

- `Download JSON example`;
- `Download JSON authoring guide`;
- optionally `Download JSON Schema` as a separate link.

## 6. Guide contents and exact authoring rules

The downloaded Markdown guide must state:

### Root format

- UTF-8 JSON.
- Exactly one root JSON object.
- No filename text, progress text, Markdown fences, comments, trailing comma,
  or extra text before/after the object.
- `schema_version` must be exactly `"1.0"`.
- Unknown fields are rejected; use the supplied schema/example as the base.

### Required fields

- `template.name`.
- `template.template_category` with `periodic` or `irregular`.
- `template.periodicity` for periodic templates using only:
  `daily`, `weekly`, `monthly`, `quarterly`, `yearly`, `event_based`, `ad_hoc`.

### Deadline rules

- Periodic templates may derive `deadline_rule` from
  `applicability_rules.deadline_days`.
- Periodic `deadline_days` must be greater than zero when applicability rules
  are supplied.
- Irregular templates require a non-empty `deadline_rule`.
- Dates use `YYYY-MM-DD`.
- `NEXT_SLOT` does not need an `applicable_from_slot`; omit it unless the mode
  specifically requires a slot.

### Applicability and mapping

- Valid company classes are `listed`, `large_public`, and `non_large_public`.
- Department references should use portable `code` + `name`, not tenant-specific
  opaque IDs.
- Import may require explicit tenant mapping before `can_confirm=true`.
- Never guess a department mapping.

### Workflow and references

- Workflow stage is required.
- `processing_days >= 1`.
- Assignee roles must be from the supported role registry.
- File-local IDs in legal bases/checklists are not server-owned persistent IDs.
- `type_id`, when supplied, must match `^[a-z0-9][a-z0-9_-]*$`.

### Recommended authoring loop

1. Download the guide, schema, and canonical example.
2. Copy the example and edit only allowed fields.
3. Run a local JSON parser and JSON Schema validator.
4. Upload to CMS and run Validate.
5. Resolve required department mappings.
6. Confirm Draft only after `can_confirm=true`.
7. Review Draft in CMS before publish/activate.

Include a table mapping common errors to fixes:

| Error | Fix |
|---|---|
| `INVALID_JSON_PAYLOAD` | Remove wrapper text/fences and ensure one JSON object |
| `INVALID_PERIODICITY` | Use the allowed periodicity enum |
| `INVALID_APPLICABILITY_RULES` | Use valid company classes and positive deadline days |
| `mapping_required` | Map every portable department code to a DEV/tenant catalog ID |
| `can_confirm=false` | Resolve mappings and revalidate before confirm |

## 7. Frontend solution

Add a new top-level CMS Templates sub-tab or route:

`/cms/templates?tab=import-history`

Do not rename or repurpose `Lịch sử bản ghi CMS`.

The Import History screen should contain:

- status filter;
- date range filter;
- filename/hash search;
- paginated table;
- status badge and validation summary;
- required/resolved mapping counts;
- created template link for `CONFIRMED` rows;
- detail drawer/page with safe error codes and timestamps;
- empty, loading, forbidden, error, and retry states.

On the import screen, place the guide download beside the existing example
download. Use the existing auth API client and envelope/error mapping.

Platform boundary:

- `/cms/...` only;
- route guard requires CMS access;
- history read requires `platform.cms.view` (plus existing CMS access);
- no tenant `/app/...` route should expose platform import history.

## 8. Implementation plan

### Task 1: Freeze contract and guide artifact

Acceptance:

- API DTOs, status enum, permission matrix, and retention policy documented.
- Markdown guide and canonical schema/example reviewed together.
- No code or migration yet.

Verification: contract review and schema/example parse tests.

### Task 2: Add expand-only persistence

Acceptance:

- migration creates `cms_template_import_attempts` and indexes;
- down migration only removes objects created by this migration;
- no raw payload/token columns;
- migration is idempotent according to repository conventions.

Verification: migration unit tests plus MySQL 8 DEV isolated-schema up/rerun/down.

### Task 3: Record validate lifecycle

Acceptance:

- validate creates a safe attempt record for parse/domain success and failure;
- response adds `import_attempt_id` without breaking existing clients;
- mapping-required state is persisted;
- no payload/token leakage.

Verification: handler/service/repository tests for success, malformed JSON,
domain failure, mapping required, permission denied, and DB failure.

### Task 4: Bind confirm lifecycle

Acceptance:

- token is bound to attempt ID;
- success changes attempt to `CONFIRMED` and retains existing audit event;
- failed confirm is visible as `CONFIRM_FAILED` without partial materialization;
- replay/conflict behavior remains unchanged.

Verification: existing confirm tests plus lifecycle and concurrency tests.

### Task 5: Add history list/detail API

Acceptance:

- tenant/platform scope is enforced;
- filters and cursor pagination are bounded;
- response exposes only safe metadata;
- not-found and forbidden errors follow standard envelope.

Verification: API contract tests for filters, pagination, authorization, and
data redaction.

### Task 6: Add guide download API and frontend links

Acceptance:

- guide downloads with stable filename/content type;
- auth and permission checks match canonical example download;
- no DB write;
- guide version is visible in content.

Verification: backend response/header tests and frontend download tests.

### Task 7: Build Import History screen

Acceptance:

- new import-history route/tab does not collide with record history;
- list/detail/filter/loading/error/forbidden/empty states work;
- confirmed entries link to the created template;
- no raw payload/token rendered.

Verification: Vitest component/API tests and DEV browser smoke with read-only
history checks.

### Checkpoint A

- backend targeted tests pass;
- frontend import/history tests pass;
- `go build ./...` and frontend build pass;
- migration validated on isolated MySQL 8 schema;
- no production access or migration.

### Checkpoint B — DEV QA

- validate a valid file;
- validate a malformed file;
- validate a mapping-required file;
- confirm one Draft only with explicit mapping;
- verify all attempts appear in history;
- verify no publish/activate/email side effects;
- verify frontend working tree is clean if no frontend change was intended.

## 9. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Storing raw JSON | High: PII/secrets/data leakage | Store hash and safe summaries only |
| History uses generic audit only | Medium: validate failures disappear | Dedicated import-attempt table |
| Confusing record history with import history | Medium: wrong operator workflow | Separate route/tab and explicit labels |
| Mapping IDs differ per tenant | High: wrong template workflow | Portable refs plus mandatory explicit mapping |
| Token appears in detail/logs | High: confirm forgery risk | Redaction tests and DTO allowlist |
| Large history query | Medium: slow CMS | Cursor pagination, bounded limit, indexes |
| Migration applied to live schema prematurely | High | Isolated MySQL 8 validation, flag-off rollout, explicit approval |

## 10. Recommendation

Implement in two vertical slices:

1. Guide download first: low-risk, zero-DB-write, immediately improves import
   correctness.
2. Import history second: migration + lifecycle persistence + API + UI,
   validated on isolated MySQL 8 before DEV rollout.

Do not extend `Lịch sử bản ghi CMS` to carry import attempts; the two histories
have different entities, lifecycle, permissions, and retention concerns.

**Cached for:** Team reuse, implementation planning, code review, and pre-merge
assessment.
