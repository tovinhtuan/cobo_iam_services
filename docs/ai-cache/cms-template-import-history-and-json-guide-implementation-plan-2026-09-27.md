# Implementation plan: CMS template import history + JSON authoring guide

Created: 2026-09-27
Decisions closed: 2026-09-27
Branch: `fixbug/test-9`
Status: **READY_FOR_IMPLEMENTATION_REVIEW**
Open decisions: **0**

Planning document only. No source edit, no runtime schema edit, no migration file, no DEV call.

Skills: `integration-cross-repo`, `planning-and-task-breakdown`. Mandatory README: đã áp dụng.

---

## 1. Executive summary

Two slices:

1. Download a Vietnamese Markdown authoring guide, the existing JSON example, and the IAM JSON Schema. These do not use the history flag and do not write the database.
2. Persist a redacted import-attempt row and show it at `/cms/templates?tab=import-history`, separate from `Lịch sử bản ghi CMS`.

`CMS_TEMPLATE_IMPORT_HISTORY_ENABLED` defaults off. Off means validate/confirm keep today’s behavior and history reads return HTTP 404 `FEATURE_DISABLED`. On means every accepted validate returns `import_attempt_id`, and confirm **requires** that id.

`import_attempt_id` stays in the confirm body (Option A). The row stores `file_sha256`, `canonical_payload_sha256`, and `validation_token_sha256`. It does not store the raw token or the raw JSON.

History is retained for **365 days**. This phase does not add a purge job or an archive table. List and detail skip rows older than 365 days. A follow-up operations task owns the purge.

Periodic templates accept only `daily|weekly|monthly|quarterly|yearly`. Irregular templates require `deadline_rule` and do not require `periodicity`. If an irregular file still has `periodicity`, only `event_based` or `ad_hoc` is valid. There is no single seven-value enum for every category.

---

## 2. Current-state evidence

| Fact | Evidence |
|---|---|
| Import routes | `POST .../import/validate`, `POST .../import/confirm`, `GET .../import/example` in `internal/disclosure/transport/http/handler.go`. No history route. No guide route. |
| Validate writes | `ValidateTemplateImport` performs zero DB writes today. Parse/domain failures are HTTP 200 with `errors[]`. Wrong `schema_version` is HTTP 422 `INVALID_REQUEST`. |
| Confirm | `ConfirmTemplateImport` verifies the HMAC token, canonical hash, mappings, display groups, and `TypeExists`, then `UpsertTypeVersion` with `CreateOnly=true`. Result is draft v1, not active, not released. The body does not resend the original file. |
| Token claims | `schema_version`, `payload_hash`, `actor_id`, `issued_at`, `expires_at`. Purpose `template_import`. TTL 15 minutes. No attempt id. No `target_type_id`. Issued when `domain_valid` is true. |
| Flag formula | `mapping_required = len(required_mappings) > 0` (includes auto-matched rows). `can_confirm = domain_valid && !mapping_required`. `activation_ready = len(activation_blockers) == 0`. The import screen does not treat `activation_ready` as the confirm switch. |
| Audit | `disclosure.type.import` only after a successful confirm, post-commit. Validate failures are absent from that feed. |
| Flag style in repo | `workflowdept/flags.go`: unset is off; on only for `true`, `1`, `yes`. Disabled reads that must not look empty use HTTP 404 `FEATURE_DISABLED` (`workflowstepcomments` mention candidates). |
| Deadline | `DeriveImportCompatibilityDeadlineRule`: periodic with `deadline_days > 0` **replaces** raw `deadline_rule` with `T+{days}` (`T+99` + days 20 becomes `T+20`). Irregular is never derived. Empty irregular rule fails `DEADLINE_RULE_REQUIRED`. |
| Periodicity today | Periodic validator allows only the five frequencies. `event_based` / `ad_hoc` fail `INVALID_PERIODICITY` for periodic templates. An irregular fixture already uses `event_based` and expects `DEADLINE_RULE_REQUIRED` when the rule is empty, not `INVALID_PERIODICITY`. The periodic check is not applied to irregular. A flat schema enum still lists all seven values for every category. |
| Schema drift | IAM schema does not require `deadline_rule`. Frontend schema requires it with `minLength: 1`. The import screen does not run that schema before POST. |
| Department catalog | `ListTemplateDepartments` reads `workflow_template_departments` with no `company_id` and no `is_active` column. Retired codes are `workflow_template_department_code_registry.retired_at`. “Active” in this plan means the code is in the current catalog list and `retired_at` is not set. This feature does not add an `is_active` column. |
| Record history | Editor tab label `Lịch sử bản ghi CMS` lists Global CMS records for one template. It is not an upload log. |
| Ledger | `run_dev_migrations.sh` ends at `0147_...`. No `0148` file at this review. That is an observation, not a reserved number. |

---

## 3. Final decisions

No item in this section is an open question.

| ID | Decision |
|---|---|
| D1 | `CMS_TEMPLATE_IMPORT_HISTORY_ENABLED` defaults off. Parse matches `workflowdept`: `true` / `1` / `yes` on; anything else off. |
| D2 | Flag off: no history insert or update. `import_attempt_id` optional and ignored. Validate/confirm HTTP behavior stays as today. History GET returns 404 `FEATURE_DISABLED`. The tab stays visible and shows “Tính năng chưa được bật”, not an empty table. Guide, example, and schema do not read the flag. |
| D3 | Flag on: validate always returns a non-empty `import_attempt_id`. Confirm requires it. Missing id → `IMPORT_ATTEMPT_REQUIRED`. Insert failure on validate does not return a success body without an id; it returns HTTP 503 `SERVICE_UNAVAILABLE` so the client cannot confirm. |
| D4 | Option A. Attempt id is a body field. HMAC claims are not extended. Store `validation_token_sha256`, `canonical_payload_sha256`, and `file_sha256`. Never store the raw token or raw JSON. |
| D5 | Retention is **365 days** (`retention_days = 365`). No purge job and no archive table in this phase. Reads filter `created_at >= now-365d`. Rows older than that may still sit in the table until the follow-up purge. Down migration does not delete rows by age; it only drops objects this migration created. |
| D6 | Same company and different actor, or a different company, is HTTP 403 `IMPORT_ATTEMPT_FORBIDDEN`. Unknown id is HTTP 404 `IMPORT_ATTEMPT_NOT_FOUND`. No takeover. |
| D7 | “No revalidate” means no second file upload. Confirm re-checks token, hashes, status, mappings, and live catalog targets. Client `can_confirm` is ignored. Confirm proceeds only when every snapshotted source has a live target, so resolved count equals required count. |
| D8 | `target_type_id` is not frozen to `suggested_type_id`. Policy: non-empty, pattern `^[a-z0-9][a-z0-9_-]*$`, length ≤ 64, `target_name` equals the normalized name, and the id is not already used. The operator may change it after validate. |
| D9 | Periodic periodicity is only `daily`, `weekly`, `monthly`, `quarterly`, `yearly`. `event_based` and `ad_hoc` are invalid for periodic. |
| D10 | Irregular: `deadline_rule` required; `periodicity` optional. If `periodicity` is present, only `event_based` or `ad_hoc`. The new guide does not tell authors to set irregular periodicity; it tells them to set `deadline_rule`. |
| D11 | No shared seven-value periodicity enum. Both schema copies use conditional schema (`if` / `then`). Periodic `deadline_rule` optional; if `deadline_days > 0`, normalizer overwrites it with `T+{days}`. Implementation adds a validator check for irregular `periodicity` when the field is present, so runtime matches D10. Existing periodic five-value rule and irregular deadline rule stay. |
| D12 | `activation_ready` is not a confirm gate. `disclosure.type.import` remains post-commit on success only. |
| D13 | History tab label is `Lịch sử import file`. Record-history label and component stay unchanged. |
| D14 | New error codes live next to `CodeInvalidImportToken` in `internal/platform/errors`. |

---

## 4. Feature flag semantics

Name: `CMS_TEMPLATE_IMPORT_HISTORY_ENABLED`.

Helper: `internal/disclosure/app/template_import_flags.go`, same `envOn` shape as `workflowdept/flags.go`. Read on each request so tests can `t.Setenv`. Do not cache it only at process boot.

| | Flag off | Flag on |
|---|---|---|
| Validate history write | No | Yes, including parse failure, domain failure, schema-version 422 after a file was accepted, `MAPPING_REQUIRED`, and `VALIDATED` |
| Validate response | Today’s DTO. No `import_attempt_id` | Same DTO plus non-empty `import_attempt_id` |
| Validate insert error | Not applicable | HTTP 503 `SERVICE_UNAVAILABLE`. No preview that the client could confirm without an id |
| Confirm history write | No | Yes: success `CONFIRMED`, failure `CONFIRM_FAILED` |
| `import_attempt_id` | Optional. Ignored if sent | Required. Missing → 400 `IMPORT_ATTEMPT_REQUIRED` |
| Existing token, mapping, draft, audit rules | Unchanged | Still applied, plus section 8 when the id is present |
| History GET | 404 `FEATURE_DISABLED`, message `FEATURE_DISABLED`, empty details. No env name, no table name, no rows | Company-scoped page, retention filter applied |
| Frontend history | Tab visible. Copy: tính năng chưa được bật. Not the empty state | List, filters, detail |
| Guide / example / schema | Work | Work |
| `disclosure.type.import` | Only after successful confirm, as today | Same. Not a substitute for the attempt row |

Transport rejects before a file is accepted (401, 403, missing part, wrong content-type, oversize) never insert a row, in both modes.

Observability: history 404 because the flag is off logs route and request id only. Insert/update errors log operation, company id, and attempt id if one exists. No payload, no token, no Authorization header.

Rollback of the flag: unset the variable and restart. Writes stop. Reads return 404 again. The table stays.

---

## 5. Import lifecycle state machine

```text
flag off → no row

flag on, file accepted
  ├─ parse, domain, or schema-version failure → VALIDATION_FAILED
  ├─ domain ok, unresolved snapshot mappings → MAPPING_REQUIRED
  └─ domain ok, unresolved count = 0 → VALIDATED

MAPPING_REQUIRED or VALIDATED
  ├─ binding ok and template commit ok → CONFIRMED
  └─ binding or materialize fails → CONFIRM_FAILED

CONFIRMED is terminal
CONFIRM_FAILED and VALIDATION_FAILED do not continue
a new upload is a new row
```

| From | To | Rule |
|---|---|---|
| none | `VALIDATION_FAILED`, `MAPPING_REQUIRED`, `VALIDATED` | Validate insert |
| `VALIDATED`, `MAPPING_REQUIRED` | `CONFIRMED` | After template commit only |
| `VALIDATED`, `MAPPING_REQUIRED` | `CONFIRM_FAILED` | Reject or materialize failure, no Draft |
| `CONFIRMED` | none | Replay is 409 `IMPORT_ATTEMPT_ALREADY_CONFIRMED` |
| `CONFIRM_FAILED` | none | No retry API |

`MAPPING_REQUIRED` may move straight to `CONFIRMED` when confirm supplies a live target for every source. That is not a second upload.

`resolved_mapping_count` and `required_mapping_count` are frozen at validate. Confirm does not rewrite them. The request is accepted only when the number of sources that resolve to a live target equals `required_mapping_count`.

Concurrency: claim `row_version` before `UpsertTypeVersion`. The loser does not insert. See section 6.

---

## 6. Import attempt data model

Table `cms_template_import_attempts`. Expand-only. No foreign key that would delete history when a template is removed.

`retention_days = 365` is a code constant, not a column. List and detail use `created_at >= UTC_TIMESTAMP(3) - INTERVAL 365 DAY` (or the application clock equivalent). A row older than 365 days is not returned: list omits it; detail by id returns 404 `IMPORT_ATTEMPT_NOT_FOUND`. Until the follow-up purge, the bytes can remain in the table. The internal data contract and the follow-up task state this. The author-facing JSON guide does not describe database retention.

| Column | Null | Mutability |
|---|---|---|
| `id` CHAR(36) PK | no | Immutable |
| `company_id` VARCHAR(64) | no | Immutable. Required on every read and write |
| `actor_user_id` VARCHAR(64) | no | Immutable |
| `actor_membership_id` VARCHAR(64) | yes | Immutable |
| `filename` VARCHAR(255) | no | Immutable basename. No path separators |
| `file_size_bytes` INT UNSIGNED | no | Immutable. Max 2097152 |
| `file_sha256` CHAR(64) | no | Immutable. Raw upload bytes at validate |
| `canonical_payload_sha256` CHAR(64) | yes | Immutable. Null if the canonical hash was not produced |
| `validation_token_sha256` CHAR(64) | yes | Immutable. SHA-256 of the token string. Null when no token was issued. Never the raw token |
| `schema_version` VARCHAR(16) | yes | Immutable |
| `status` VARCHAR(32) | no | Section 5 only |
| `row_version` BIGINT | no | Starts at 1. Increment on each successful update. Confirm claim uses equality |
| `parse_valid`, `domain_valid`, `activation_ready`, `can_confirm`, `mapping_required` TINYINT(1) | no | Immutable snapshots |
| `required_mapping_count`, `resolved_mapping_count`, `unresolved_mapping_count` | no | Immutable snapshots |
| `error_codes` JSON | yes | Immutable. Max 30 `{code, field_path, severity}`. No parser message text |
| `mapping_summary` JSON | yes | Immutable. Max 50 `{type, source_id, source_name, target_id, is_auto_matched}` |
| `target_type_id` VARCHAR(64) | yes | Set once on the confirm attempt that passes structural checks |
| `created_type_id` VARCHAR(128) | yes | Set once, only with `CONFIRMED` |
| `confirm_error_code` VARCHAR(64) | yes | Set once, only with `CONFIRM_FAILED` |
| `request_id` VARCHAR(64) | yes | Immutable request id |
| `created_at`, `validated_at` DATETIME(3) | validated nullable | Immutable |
| `confirmed_at` DATETIME(3) | yes | Set once on `CONFIRMED` or `CONFIRM_FAILED` |
| `updated_at` DATETIME(3) | no | Moves on update |

Indexes inside `CREATE TABLE`:

- primary key `id`
- `(company_id, created_at, id)`
- `(company_id, status, created_at, id)`
- `(company_id, file_sha256, created_at)`

No unique constraint on hashes.

Claim before insert:

```text
UPDATE ... SET row_version = row_version + 1
WHERE id = ? AND company_id = ? AND actor_user_id = ?
  AND status IN ('VALIDATED','MAPPING_REQUIRED')
  AND row_version = ?
  AND created_at >= retention cutoff
```

Zero rows → do not call `UpsertTypeVersion`. If the row is already `CONFIRMED`, return `IMPORT_ATTEMPT_ALREADY_CONFIRMED`. If another request claimed the version, return the same 409 so a second Draft is not started.

Ordering:

| Sequence | Outcome |
|---|---|
| Claim, then materialize rolls back | No Draft. Separate update to `CONFIRM_FAILED`. Status is not `CONFIRMED`. |
| Commit succeeds, then status update fails | HTTP 201. Draft exists. Audit may exist. Row not `CONFIRMED`. Error log. Do not roll back the Draft. |
| Status set to `CONFIRMED` before commit | Forbidden. A test injects an upsert error and asserts the row is not `CONFIRMED`. |
| Audit succeeds, history update fails | Same as the post-commit history failure. One audit event. |
| Audit fails, history update succeeds | Row `CONFIRMED`. HTTP 201. Existing audit error log. Do not flip the row backward. |

Not stored: raw JSON, normalized template document, raw validation token, bearer token, password, cookie, Authorization header, `metadata.exported_by`, arbitrary request maps.

PII: actor and membership ids only. No email lookup. Token hash never appears in list or detail. List shows a 12-hex fingerprint of `file_sha256`. Detail shows the full file hash and canonical hash.

---

## 7. Token, payload, and attempt binding

Claims stay: `schema_version`, `payload_hash`, `actor_id`, `issued_at`, `expires_at`, purpose `template_import`. Do not add `import_attempt_id` or `target_type_id` to the HMAC payload in this phase.

When the flag is on:

```text
body.import_attempt_id required
  → load by id
  → missing → 404 IMPORT_ATTEMPT_NOT_FOUND
  → company or actor mismatch → 403 IMPORT_ATTEMPT_FORBIDDEN
  → created_at older than 365 days → 404 IMPORT_ATTEMPT_NOT_FOUND
  → status not VALIDATED or MAPPING_REQUIRED → already confirmed or not confirmable
  → VerifyToken: signature, purpose, schema 1.0, expiry, actor
  → sha256(token string) = validation_token_sha256
  → sha256(normalized_template) = canonical_payload_sha256 = token.payload_hash
  → mapping source set equals snapshot
  → every source has a live catalog target
  → resolved count = required_mapping_count
  → target_type_id matches D8
  → claim row_version
  → UpsertTypeVersion
```

`file_sha256` is computed only at validate. Confirm does not receive the original bytes, so it does not recompute that hash. Swapping a token from a second validate of the same file fails `IMPORT_ATTEMPT_TOKEN_MISMATCH` because `issued_at` differs and the token hash differs.

Flag off: skip this lookup. Existing confirm checks still run.

### Error matrix

| Code | HTTP | When |
|---|---|---|
| `IMPORT_ATTEMPT_REQUIRED` | 400 | Flag on and confirm omits `import_attempt_id`. Not used when the flag is off. |
| `IMPORT_ATTEMPT_NOT_FOUND` | 404 | Unknown id, or id outside the 365-day read window. |
| `IMPORT_ATTEMPT_FORBIDDEN` | 403 | Id exists, actor or company does not match. Distinct from `PERMISSION_DENIED` (missing CMS permission). |
| `IMPORT_ATTEMPT_ALREADY_CONFIRMED` | 409 | Status `CONFIRMED`, or a concurrent claim already took the row. |
| `IMPORT_ATTEMPT_PAYLOAD_MISMATCH` | 422 | Canonical hash does not match the row or the token `payload_hash`. |
| `IMPORT_ATTEMPT_TOKEN_MISMATCH` | 422 | Token hash does not match the row, while a signature check may still pass for a different issue. |
| `IMPORT_ATTEMPT_MAPPING_INCOMPLETE` | 400 | Source set mismatch, blank target, or resolved count &lt; required count. |
| `IMPORT_ATTEMPT_STALE_REFERENCE` | 409 | Target code missing from `workflow_template_departments`, or `retired_at` is set. |
| `INVALID_IMPORT_TOKEN` | 422 | Bad signature, expired, wrong purpose, wrong token schema, token actor ≠ caller. Existing code. |
| `TARGET_TYPE_CONFLICT` | 409 | `target_type_id` already exists. HTTP stays 409 so current clients that key off status still see a conflict. The code string changes from a generic `STATE_CONFLICT` for this case. |
| `FEATURE_DISABLED` | 404 | History route, flag off. |
| `PERMISSION_DENIED` | 403 | Missing CMS permission. |
| `SERVICE_UNAVAILABLE` | 503 | Flag on and the validate history insert fails. |
| `INVALID_REQUEST` | 400 | Malformed JSON, empty name, name mismatch, bad filters. |

Frontend `mapImportApiError` today treats every HTTP 409 as “pick another type id”. Implementation must branch `IMPORT_ATTEMPT_ALREADY_CONFIRMED` away from that copy.

---

## 8. Confirm security invariants

When the flag is on, before `UpsertTypeVersion`:

1. Attempt exists inside the retention window.
2. `company_id` matches the caller.
3. `actor_user_id` matches the caller.
4. Status is `VALIDATED` or `MAPPING_REQUIRED`.
5. Token is valid, purpose `template_import`, unexpired.
6. Token hash matches the row.
7. Canonical hash of `normalized_template` matches the row and the token.
8. `target_type_id` matches policy D8.
9. Mapping source ids are exactly the snapshot. No added or dropped sources.
10. Every source has a non-empty target that is in the current global catalog and is not retired.
11. Resolved count equals `required_mapping_count`.
12. Display groups are re-checked as they are today.
13. `row_version` claim succeeds. Replay and the concurrent loser do not create a second Draft.

Do not trust `can_confirm` or a mapping list that the client invents. Do not require `activation_ready`.

Flag off: items 1–7 and 9–11 and 13 are skipped. Items that already exist in `ConfirmTemplateImport` (token, canonical hash of the body, catalog, type id, name) still run.

---

## 9. Deadline and periodicity contract

| Case | Runtime |
|---|---|
| Periodic, no `deadline_rule`, `deadline_days > 0` | Normalize to `T+{days}`. Pass deadline-rule check. |
| Periodic, raw `deadline_rule`, `deadline_days > 0` | Days win. Stored and token-bound value is `T+{days}`. |
| Periodic, no positive `deadline_days` | `APPLICABILITY_DEADLINE_DAYS_REQUIRED` when applicability rules are present without positive days. |
| Periodic `periodicity` | Only `daily`, `weekly`, `monthly`, `quarterly`, `yearly`. `event_based` or `ad_hoc` → `INVALID_PERIODICITY`. |
| Irregular, missing `deadline_rule` | `DEADLINE_RULE_REQUIRED`. Do not derive from `deadline_days`. |
| Irregular, `deadline_rule` set, `periodicity` omitted | Pass the periodicity check. |
| Irregular, `periodicity` `event_based` or `ad_hoc`, rule set | Pass. Legacy payloads may do this. The guide does not recommend the field. |
| Irregular, any other `periodicity` | Fail. This check is additive in implementation so it matches the schema. It does not loosen the five-value periodic rule or the irregular deadline rule. |

Guide text must say the overwrite rule in one sentence: for periodic templates, `deadline_days` greater than zero replaces any authored `deadline_rule`.

---

## 10. Schema parity plan

Do not edit schema files in the planning step. Implementation phase edits both:

- `cobo_iam_services/docs/schema/template-import-v1.schema.json`
- `cobo_web_design/docs/schema/template-import-v1.schema.json`

And the embedded copy served by the schema endpoint, which must be the IAM file after the edit.

Remove the single periodicity enum of seven values. Use draft-07 `if` / `then`:

- `template_category = periodic` → `periodicity` required, enum of five values. `deadline_rule` not required.
- `template_category = irregular` → `deadline_rule` required, `minLength` 1. `periodicity` not required. If present, enum `event_based`, `ad_hoc`.

`additionalProperties: false` stays. Root `schema_version` const `"1.0"` stays.

Parity test: load both schema files and compare the conditional enums and required arrays. The import screen must keep posting the file without a client schema gate that rejects a periodic file which omits `deadline_rule`. A regression test posts or parses that fixture through the current screen path and expects the request to be sent, not blocked in the browser.

Existing Go tests to keep: `TestDeriveImportCompatibilityDeadlineRule_OverridesRaw`, `TestDeriveImportCompatibilityDeadlineRule_MissingRuleWithDays`, `TestValidateImportTemplate_PeriodicDeadlineRuleOptionalWithDays`, `TestValidateImportTemplate_IrregularStillRequiresDeadlineRule`.

New fixtures are in section 17.

---

## 11. Migration safety

Not created and not run in this planning step. Not applied to the DEV smoke schema.

When implementation starts:

1. List `migrations/*.up.sql` and `run_dev_migrations.sh`.
2. Confirm no parallel file took the next number.
3. Use highest + 1 that day. The note “0147 is current” is not permission to write `0148` without that listing.
4. Up: `CREATE TABLE IF NOT EXISTS` only, `utf8mb4` / `utf8mb4_unicode_ci`, `DATETIME(3)`, InnoDB. No backfill. No purge statement.
5. Down: `DROP TABLE IF EXISTS cms_template_import_attempts` only. No manual `DELETE` by age. No drop of any other table, index, or procedure.
6. Static test fails if the down file drops anything else.
7. Isolated MySQL 8: up, insert, up again, down, up. Do not load `.env`. Do not point at the shared DEV database.
8. Append the up file to the runner only after the number is chosen.

---

## 12. API contract matrix

Authn: existing bearer inspection. Do not log the header.

| Route | Method | Permission | Flag | Success |
|---|---|---|---|---|
| `POST /api/v1/platform/cms/templates/import/validate` | multipart `file` | `requireCMSTemplateWrite` | Off: no id, no write. On: id required in the body that returns | HTTP 200 raw DTO, or 503 if the on-mode insert fails |
| `POST /api/v1/platform/cms/templates/import/confirm` | JSON | `requireCMSTemplateWrite` | Off: id optional and ignored. On: id required | HTTP 201 existing Draft DTO |
| `GET /api/v1/platform/cms/templates/import-history` | query | `platform.cms.view` and non-empty `company_id` | Off: 404 `FEATURE_DISABLED` | `{ data.items, meta.limit, meta.next_cursor }`. Retention filter. `limit` default 20, max 100. Filters: `status`, basename `filename`, exact `file_sha256`, `from`, `to` RFC3339. Cursor is base64 `(created_at, id)` descending inside the company and the retention window |
| `GET /api/v1/platform/cms/templates/import-history/{id}` | | Same as list | Off: 404 `FEATURE_DISABLED` | `{ data }` allowlist. Outside retention or other company handled per section 7 |
| `GET .../import/example` | | write | none | Unchanged attachment |
| `GET .../import/guide` | | write | none | `text/markdown; charset=utf-8`, filename `cobo-template-import-guide-v1.0.md`, `nosniff`, `Cache-Control: private, no-store` |
| `GET .../import/schema` | | write | none | `application/json`, filename `cobo-template-import-v1.schema.json`, same cache headers, not wrapped in `{ data }` |

List fields: `id`, `filename`, `file_sha256_fingerprint`, `status`, the four booleans, `mapping_required`, three counts, `target_type_id`, `created_type_id`, `confirm_error_code`, `created_at`, `updated_at`.

Detail adds full file hash, canonical hash, `schema_version`, `error_codes`, `mapping_summary`, actor ids, `file_size_bytes`, `validated_at`, `confirmed_at`. It does not add the token hash.

Audit: none on GET, guide, schema, example, or validate. Confirm success still emits `disclosure.type.import` and may add `import_attempt_id` to metadata. No token and no template JSON in that metadata.

---

## 13. Permission matrix

| Surface | Rule |
|---|---|
| `/cms/templates` | `platform.cms.view` |
| Validate, confirm, example, guide, schema | `platform.cms.view` and (`cms.template.write` or `disclosure_type.manage`) |
| History list/detail | `platform.cms.view` and caller `company_id`. Write is not required to read |
| Confirm of another actor or company | `IMPORT_ATTEMPT_FORBIDDEN` |
| `Lịch sử bản ghi CMS` | Unchanged `cms.record.read` |

Empty `company_id` on a history read: 403 `PERMISSION_DENIED`, no query.

---

## 14. Frontend routes and UI states

Route: `/cms/templates?tab=import-history`.

Label: `Lịch sử import file`.

Do not change `Lịch sử bản ghi CMS`.

| State | UI |
|---|---|
| Loading | `Đang tải lịch sử import file.` `aria-busy` |
| Flag off | `Tính năng chưa được bật.` No table. Not the empty copy |
| Empty, flag on, inside retention | `Chưa có lần import nào trong bộ lọc này.` |
| Forbidden | Amber panel, own test id |
| Network / 5xx | Message plus `Thử lại` |
| Success | Time in `Asia/Ho_Chi_Minh`, filename, status text, parse/domain, mapping counts, activation text that is not a confirm button, link `?type_id=` only when `created_type_id` is set |

Filters reset the cursor. `Tải thêm` uses `next_cursor`. Detail uses the detail endpoint. No token, token hash, or raw JSON. No re-import button.

Import screen downloads, all three, write-gated, independent of the flag:

1. `Tải file mẫu JSON`
2. `Tải hướng dẫn JSON`
3. `Tải JSON Schema`

They do not read the in-memory token or the selected file. The screen must not pre-reject a periodic file that omits `deadline_rule`.

---

## 15. JSON guide content

File: `internal/disclosure/app/artifacts/cobo-template-import-guide-v1.0.md`, embedded. Vietnamese. Version `1.0`.

State: one root object; `schema_version` `"1.0"`; unknown fields rejected; `name` and `template_category` required; periodic periodicity is the five values only; `event_based` and `ad_hoc` are not valid for periodic; irregular authors set `deadline_rule` and should omit `periodicity`; if a legacy irregular file includes `periodicity`, only `event_based` or `ad_hoc`; periodic `deadline_days > 0` overwrites `deadline_rule` with `T+{days}`; irregular empty rule fails; company classes `listed`, `large_public`, `non_large_public`; portable department `code` and `name`; `stage` and `processing_days >= 1` when a step exists; missing workflow is an activation blocker, not a confirm blocker; `type_id` pattern; dates `YYYY-MM-DD`; `NEXT_SLOT` does not need a slot; do not publish from this flow; do not trust `activation_ready` as confirm.

Include one valid periodic example that can omit `deadline_rule` when days are set, and one invalid wrapper sample (`INVALID_JSON_PAYLOAD`).

Include the stable error codes from section 7 and the domain codes the validator already returns. Do not include secrets or sample tokens.

Retention for operators is section 6 and section 21, not this author-facing file.

---

## 16. Ordered implementation tasks

### Phase 0 — Final contract freeze

## Task 0: Freeze this plan

### Goal
Keep D1–D14 as the contract. Do not reopen retention, attempt id, or periodicity.

### Dependencies
None. This file is the freeze.

### Files likely touched
- `cobo_iam_services/docs/ai-cache/cms-template-import-history-and-json-guide-implementation-plan-2026-09-27.md`

### Contract changes
None beyond this document.

### Acceptance criteria
- Open decision count is zero.
- Flag-off and flag-on behavior are both specified.
- Schema rules are conditional, not a seven-value enum.

### Tests
Review only.

### Verification commands
None. Documentation.

### Rollback
Revert this file.

### Risk
An implementer uses an older revision that still lists open questions.

### Phase 1 — Guide download

## Task 1: Embed the guide

### Goal
Add the Markdown artifact that matches section 15 and section 9.

### Dependencies
Task 0 accepted.

### Files likely touched
- `internal/disclosure/app/artifacts/cobo-template-import-guide-v1.0.md`
- `internal/disclosure/app/template_import_guide.go`
- `internal/disclosure/app/template_import_guide_test.go`

### Contract changes
None at runtime until Task 2.

### Acceptance criteria
- States the five periodic values and that `event_based` is not valid for periodic.
- States irregular `deadline_rule` and does not recommend irregular `periodicity`.
- States the days-overwrite rule.
- Contains a valid periodic sample and an invalid wrapper sample.
- Contains no token.

### Tests
String assertions on the embed. Parse the valid sample inside the markdown as one JSON object.

### Verification commands
- `go test ./internal/disclosure/app/ -count=1 -run TemplateImportGuide`

### Rollback
Delete the new files.

### Risk
Guide text copied from the frontend schema’s old required array.

## Task 2: Guide and schema endpoints

### Goal
Serve the guide and, after Task 8 has aligned the schema, the IAM schema. Endpoints may land before Task 8 if the schema handler is tested against the embedded bytes that Task 8 updates. Prefer landing Task 8’s schema bytes before enabling the schema route in review.

### Dependencies
Task 1. Schema route content depends on Task 8.

### Files likely touched
- `import_template_handler.go`
- `handler.go`
- `import_template_handler_test.go`
- `internal/disclosure/app/artifacts/cobo-template-import-v1.schema.json`

### Contract changes
Section 12 guide and schema rows. No flag. No DB.

### Acceptance criteria
- Guide filename and `text/markdown; charset=utf-8`.
- 403 body is an error envelope, not the file.
- Schema download is not wrapped in `{ data }`.

### Tests
Headers, 403, body equals embed.

### Verification commands
- `go test ./internal/disclosure/transport/http/ -count=1 -run TemplateImport`

### Rollback
Remove the routes.

### Risk
Serving the old frontend schema. The parity test in Task 8 must fail that.

## Task 3: Download buttons

### Goal
Three download actions on the import screen.

### Dependencies
Task 2.

### Files likely touched
- `cobo_web_design/src/features/cms-core/templates/import/api/importTemplateApi.ts`
- `ImportExampleDownload.tsx` or a sibling
- `ImportTemplateScreen.tsx`
- Vitest files beside them

### Contract changes
None on validate/confirm.

### Acceptance criteria
- Example button unchanged.
- Guide and schema set the server filenames.
- Download calls do not attach the token or the selected file.

### Tests
Path and `download` attribute. 403 toast.

### Verification commands
- `npx vitest run src/features/cms-core/templates/import/components/ImportExampleDownload.test.tsx src/features/cms-core/templates/import/ImportTemplateScreen.test.tsx`

### Rollback
Remove the two new buttons.

### Risk
A client-side schema check that rejects periodic files. Task 8 covers that regression.

### Checkpoint 1

GO only when all four are true:

- Guide response PASS: HTTP 200, `text/markdown; charset=utf-8`, filename `cobo-template-import-guide-v1.0.md`, body matches the embed.
- Schema and validator rules are consistent with sections 9 and 10. At this checkpoint that means the guide, the canonical example, and the existing Go validator agree: periodic frequencies are the five values, periodic `event_based` is invalid, irregular `deadline_rule` is required, and periodic `deadline_days > 0` overwrites a raw `deadline_rule`. The example JSON used by the guide must parse and pass that validator path. The two schema files are rewritten in Phase 5; Checkpoint 4 then proves the files match each other. Checkpoint 1 does not treat the old frontend schema (unconditional `deadline_rule`, flat seven-value enum) as the contract.
- No database write from guide, example, or schema download.
- No runtime migration file is applied. Task 4 has not started.

NO-GO if a migration file was added, if the guide tells periodic authors that `event_based` is valid, if the guide says raw `deadline_rule` wins over `deadline_days`, or if the guide endpoint inserts a history row.

### Phase 2 — Import attempt persistence

## Task 4: Migration after ledger rediscovery

### Goal
Create the table in section 6 under the next free number.

### Dependencies
Checkpoint 1. Same-day directory listing.

### Files likely touched
- `migrations/<next>_cms_template_import_attempts.up.sql`
- `migrations/<next>_cms_template_import_attempts.down.sql`
- `migrations/<next>_cms_template_import_attempts_test.go`
- `migrations/run_dev_migrations.sh`

### Contract changes
Physical table only. Includes `validation_token_sha256` and `row_version`. No purge SQL.

### Acceptance criteria
- Number was chosen after listing, not copied from this plan.
- Down drops only the new table.
- Isolated MySQL 8 up, rerun, and down pass, or the task stops with `BLOCKED:` and does not use the DEV smoke database.

### Tests
Static down-scope test. Isolated MySQL test gated by an explicit non-DEV DSN.

### Verification commands
- `go test ./migrations/ -count=1 -run CmsTemplateImportAttempts`

### Rollback
Down on the isolated schema only.

### Risk
Choosing 0148 because this document mentioned 0147.

## Task 5: Repository, flag helper, validate and confirm

### Goal
Implement sections 4–8.

### Dependencies
Task 4.

### Files likely touched
- `template_import_flags.go`
- `template_import_attempt.go`
- `template_import_service.go`
- `template_import_confirm.go`
- `template_import_contracts.go`
- `internal/platform/errors/errors.go`
- mysql and in-memory attempt repositories
- import handler and tests
- frontend confirm type: send `import_attempt_id` when the flag-on response includes it

### Contract changes
D3, D4, D6, D7, section 7 error codes. `TARGET_TYPE_CONFLICT` at HTTP 409.

### Acceptance criteria
- Flag off: zero history writes; confirm without an id still succeeds under current rules.
- Flag on: validate failure, mapping required, and success rows exist; response always has `import_attempt_id` or HTTP 503.
- Flag on: missing id is `IMPORT_ATTEMPT_REQUIRED`.
- Binding failures use the matrix in section 7.
- One Draft under replay and under two concurrent confirms.
- Injected commit failure leaves the row not `CONFIRMED`.
- Audit event still fires on success.
- No raw token or raw JSON column is written.

### Tests
Section 17 persistence and concurrency rows. Frontend test that the id is sent and not rendered.

### Verification commands
- `go test ./internal/disclosure/app/ ./internal/disclosure/transport/http/ ./internal/disclosure/infra/inmemory/ -count=1 -run 'TemplateImport|ConfirmTemplate|ImportAttempt'`

### Rollback
Turn the flag off. The optional id is ignored again.

### Risk
Returning HTTP 200 from validate when the insert failed. That violates D3.

### Checkpoint 2

GO when isolated MySQL up, rerun, and down passed; lifecycle, rollback-before-commit, replay, and concurrent confirm tests passed; redaction test shows no token and no payload column.

NO-GO if the migration ran on the shared DEV schema, if the down file deletes by age or drops another object, or if two confirms created two drafts.

### Phase 3 — History API

## Task 6: List and detail

### Goal
Section 12 read routes, retention filter, flag-off 404.

### Dependencies
Checkpoint 2.

### Files likely touched
- `template_import_history.go`
- `import_history_handler.go`
- `handler.go`
- handler tests

### Contract changes
`next_cursor`, allowlists, 365-day filter.

### Acceptance criteria
- View-only `platform.cms.view` can read.
- Missing that permission is 403 with no items.
- Flag off is 404 `FEATURE_DISABLED` with no env string.
- A row older than 365 days is absent.
- Detail JSON keys are the allowlist. Token hash is absent.

### Tests
Pagination, filters, retention, redaction, cross-actor 403, unknown id 404.

### Verification commands
- `go test ./internal/disclosure/transport/http/ -count=1 -run ImportHistory`

### Rollback
Unregister the GET routes.

### Risk
`SELECT *` into JSON. Use an explicit DTO.

### Phase 4 — Frontend history UI

## Task 7: History screen

### Goal
`?tab=import-history` with the states in section 14.

### Dependencies
Task 6.

### Files likely touched
- `TemplatesFeatureScreen.tsx`
- `TemplatesFeatureScreen.import.test.tsx`
- `import/api/importHistoryApi.ts`
- `import/ImportHistoryScreen.tsx`
- `import/ImportHistoryDetail.tsx`
- their tests

### Contract changes
UI only. Do not edit `TemplateRecordHistoryTab.tsx` or the editor history label.

### Acceptance criteria
- Disabled panel text is `Tính năng chưa được bật` and is not the empty state.
- Both labels exist in tests: `Lịch sử import file` and `Lịch sử bản ghi CMS`.
- Confirmed rows link to `?type_id=`.
- Fixture token text is not in the document.

### Tests
Loading, empty, error+retry, forbidden, disabled, filters, cursor, detail.

### Verification commands
- `npx vitest run src/features/cms-core/templates/TemplatesFeatureScreen.import.test.tsx src/features/cms-core/templates/import/ImportHistoryScreen.test.tsx`
- `npm run build`

### Rollback
Remove the tab and screen.

### Risk
`resolveCmsTemplatesTab` sending `import-history` back to the template list. The route test must see the history test id.

### Checkpoint 3

GO when Task 7 tests and `npm run build` pass, record-history label is unchanged, and disabled is distinct from empty.

NO-GO if the history screen renders raw JSON or a token.

### Phase 5 — Schema parity

## Task 8: Conditional schema and validator parity

### Goal
Apply section 10 in both schema files, the embedded schema, and the irregular periodicity check.

### Dependencies
Task 1 guide text. Can proceed in parallel with Tasks 4–7 after Checkpoint 1, but must finish before DEV QA.

### Files likely touched
- both `docs/schema/template-import-v1.schema.json` files
- embedded schema artifact
- `template_import_validator.go` (additive irregular periodicity check only)
- `deadline_rule_import_compat_test.go` or a new fixture test
- `types.contract.test.ts`
- guide sample if it still shows a seven-value enum

### Contract changes
D9–D11. Periodic `deadline_rule` not unconditionally required. No shared seven-value enum.

### Acceptance criteria
- The two schema files agree on the conditional enums.
- Periodic `event_based` fails validation.
- Irregular without `periodicity` passes when `deadline_rule` is set.
- Irregular `event_based` passes when `deadline_rule` is set.
- Irregular `yearly` fails.
- Irregular missing `deadline_rule` fails.
- Periodic missing `deadline_rule` with valid `deadline_days` passes and normalizes to `T+{days}`.
- Periodic missing both fails.
- The import UI does not block the valid periodic fixture before the request.

### Tests
The fixture list above, plus a schema parity test across the two repos, plus the UI non-blocking test.

### Verification commands
- `go test ./internal/disclosure/app/ -count=1 -run 'TemplateImport|DeadlineRule|Periodicity'`
- `npx vitest run src/features/cms-core/templates/import/types.contract.test.ts src/features/cms-core/templates/import/ImportTemplateScreen.test.tsx`

### Rollback
Revert the schema files and the additive validator branch.

### Risk
Loosening the periodic five-value rule while adding the irregular branch. The existing periodic tests must stay in the same package run.

### Checkpoint 4 — schema parity before DEV

GO when Task 8 fixtures pass and the downloaded schema matches the IAM file.

NO-GO if either schema still requires `deadline_rule` for every category, or still offers one enum of seven values to every category.

### Phase 6 — DEV QA

## Task 9: DEV matrix

### Goal
Execute section 18 only after Checkpoint 5 approval. This plan does not authorize the run.

### Dependencies
Checkpoints 1–4 and a written approval naming the migration number and the database.

### Files likely touched
- a QA note under `docs/ai-cache/` after the run. Not created now.

### Contract changes
None.

### Acceptance criteria
- Flag-off and flag-on matrices recorded.
- No publish, no activate, no email.
- Flag returned to off at the end unless product asks to leave it on.

### Tests
Automated suite green before the manual matrix.

### Verification commands
Recorded in the QA note. Not run from planning.

### Rollback
Unset the flag. Drop the table only with a separate approval, using the down file.

### Risk
A real DEV Draft. Use a disposable `type_id` and leave it inactive.

---

## 17. Test matrix

### Flag off

- Validate and confirm do not call the history repository.
- Confirm without `import_attempt_id` matches current success and error statuses.
- Confirm with an id still ignores it.
- History list and detail: 404 `FEATURE_DISABLED`, no rows, no env name.
- Guide still 200.

### Flag on

- Malformed JSON, domain error, schema-version 422: row `VALIDATION_FAILED`, original HTTP status kept, `import_attempt_id` present.
- Unresolved mapping: `MAPPING_REQUIRED`.
- Auto-matched only: `VALIDATED` even if API `can_confirm` is false. Confirm with the snapshotted live target succeeds. `resolved == required`.
- Insert failure: HTTP 503, no confirmable body.
- Missing id: `IMPORT_ATTEMPT_REQUIRED`.
- Unknown id: `IMPORT_ATTEMPT_NOT_FOUND`.
- Other actor and other company: `IMPORT_ATTEMPT_FORBIDDEN`.
- Payload edit: `IMPORT_ATTEMPT_PAYLOAD_MISMATCH`.
- Token from a second validate of the same bytes: `IMPORT_ATTEMPT_TOKEN_MISMATCH`.
- Incomplete or extra mapping source: `IMPORT_ATTEMPT_MAPPING_INCOMPLETE`, no Draft.
- Missing or retired department code: `IMPORT_ATTEMPT_STALE_REFERENCE`. Do not expect a per-company department error; the catalog is global.
- New unused `target_type_id`: success. Existing id: `TARGET_TYPE_CONFLICT`, one type no created.
- Expired token: `INVALID_IMPORT_TOKEN`, no Draft.
- Replay and concurrent confirm: one Draft, loser `IMPORT_ATTEMPT_ALREADY_CONFIRMED`.
- Upsert error: row not `CONFIRMED`.
- Audit failure after commit: row can be `CONFIRMED`, HTTP 201.
- History update failure after commit: HTTP 201, row not `CONFIRMED`, one audit event.
- List/detail: no token, no token hash, no raw JSON. Rows older than 365 days hidden.

### Periodicity and deadline

- `periodic` / `yearly` / `deadline_days` 20 / no `deadline_rule` → pass, value `T+20`.
- `periodic` / `event_based` → fail `INVALID_PERIODICITY`.
- `irregular` / no `periodicity` / `deadline_rule` set → pass.
- `irregular` / `event_based` / `deadline_rule` set → pass.
- `irregular` / `daily` → fail.
- `irregular` missing `deadline_rule` → fail.
- `periodic` missing rule and missing positive days → fail.
- `periodic` raw `T+99` with days 20 → `T+20`.
- Guide sample normalizes the same way.
- Both schema files: same conditional enums.

### Frontend

- Import history route.
- Record history label unchanged.
- Three downloads.
- Disabled, empty, forbidden, error, filters, cursor, detail.
- Valid periodic JSON is not rejected in the browser before POST.

### Migration

- Down file scope. Isolated up, rerun, down. Not the DEV smoke schema.

---

## 18. DEV QA plan

Not started by this document.

After Checkpoint 5:

1. Rediscover the migration number. Prove up/rerun/down on isolated MySQL 8.
2. Apply to DEV only with written approval. Do not use the full `docs/deploy-dev-guide.md` redeploy unless the operator asks. First boot keeps the flag unset.
3. Flag off: validate/confirm without an id still works; history is 404 `FEATURE_DISABLED`; the screen says the feature is off; guide download returns the filename and `text/markdown`.
4. Flag on, then:
   - valid periodic JSON → `VALIDATED` or `MAPPING_REQUIRED`;
   - wrapper text → `VALIDATION_FAILED`;
   - unresolved departments → `MAPPING_REQUIRED`, confirm blocked until targets exist;
   - mappings complete → one inactive Draft, row `CONFIRMED`;
   - replay → 409, still one Draft;
   - missing permission → 403, no foreign rows;
   - filters and cursor stay in one company and inside 365 days;
   - guide headers match section 12;
   - no publish, no activate, no email.
5. Unset the flag when the window ends unless product asks to leave it on.

---

## 19. Rollback plan

| Layer | Action |
|---|---|
| Flag | Unset and restart. Writes stop. History reads return 404 `FEATURE_DISABLED`. |
| API | Revert service and handlers. Flag-off confirm remains the compatibility path. |
| Frontend | Revert the tab and the two new buttons. |
| Schema docs | Revert Task 8 if the conditional schema must be undone. |
| Table | Leave it. Drop only with approval, using the down file. Down does not age-delete rows. |
| QA Draft | Leave inactive. Do not publish it to clean up. |
| Audit | Do not delete `disclosure.type.import` events as rollback. |

---

## 20. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Flag off looks like “nobody imported” | 404 `FEATURE_DISABLED` and the disabled panel, not an empty table |
| Flag on breaks old confirm clients | Required id only when the flag is on. Off ignores the field. Roll back by unsetting the flag |
| Validate returns success without an id | D3: insert failure is HTTP 503 |
| Token or raw JSON stored | No columns; redaction test; detail allowlist excludes the token hash |
| Second Draft | `row_version` claim; replay test; concurrent test |
| `CONFIRMED` without a committed template | Status update after commit; injected failure test |
| History 403 reveals that an id exists in another company | Ids are random UUIDs. Accepted under D6. Do not add a sequential id |
| Storage growth for 365 days without a purge | Section 21. Indexes stay on `(company_id, created_at, id)`. Reads filter the window so old rows are not served |
| Seven-value enum shipped again | Parity test compares both schema files |
| Frontend schema blocks periodic omit-rule files | UI regression in Task 8. Upload path stays multipart, not Ajv-first |
| `activation_ready` used as confirm | Unchanged screen rule; guide sentence |
| Migration number guessed | Task 4 lists the directory the same day |
| Record history reused | Separate tab and label tests |
| Department “other company” test invented | Catalog is global. Stale or retired code is the real test |

---

## 21. Follow-up operational tasks

Not in this implementation phase.

## Task F1: Purge import attempts older than 365 days

### Goal
Delete rows with `created_at` older than 365 days on a schedule, matching the read filter. No archive table.

### Dependencies
This feature shipped with the flag default off or on, and a security owner named.

### Files likely touched
A later worker or SQL job. Not `cmd/worker` in the current slice. Not this plan’s migration.

### Contract changes
Physical delete only. API behavior stays “older than 365 days is invisible”.

### Acceptance criteria
- Job is idempotent.
- It does not delete rows younger than 365 days.
- It does not drop the table.
- It logs counts, not filenames or hashes, unless security asks for hashes.

### Tests
Fixture rows at 364 days and 366 days. Only the older row is removed.

### Verification commands
Chosen when the job is designed. Isolated MySQL only for the first run.

### Rollback
Disable the job schedule. Reads already hide old rows.

### Risk
Clock skew deleting a row that is still inside the product window. Use the same cutoff definition as the read query.

Owner: security/operations, not the guide slice. Until this task ships, operators accept that bytes older than 365 days can remain on disk while API clients cannot read them.

---

## 22. GO / NO-GO gates

### GO for implementation

All of the following are true in this revision:

- `OPEN_DECISIONS=0`
- API contract in sections 7 and 12 is frozen
- Schema rules in sections 9 and 10 are frozen
- Retention is 365 days, with no purge in this phase
- Flag-off compatibility is specified and does not change current import

### NO-GO

- A periodic schema still requires `deadline_rule` unconditionally
- Attempt binding omits token hash or canonical hash
- History can return a raw token or raw payload
- A migration number is written without re-reading the ledger
- Flag-off validate/confirm changes current success behavior
- The import tab replaces or relabels `Lịch sử bản ghi CMS`

| Gate | GO | NO-GO |
|---|---|---|
| Start Task 1 | This revision accepted | Any D1–D14 reopened |
| Checkpoint 1 | Guide response PASS; guide, example, and validator agree on sections 9–10; zero DB writes; no migration applied | Guide contradicts the frozen periodicity or deadline rules, a history row is inserted, or a migration file exists |
| Start Task 4 | Directory listing that day | Number copied from the “0147” note alone |
| Checkpoint 2 | Isolated up/rerun/down, replay, concurrency, no raw token | DEV smoke schema migrated |
| Checkpoint 3 | History UI tests, labels distinct, disabled ≠ empty | Token rendered |
| Checkpoint 4 | Task 8 fixtures and schema parity | Flat seven-value enum remains |
| Checkpoint 5 — before DEV QA | Checkpoints 1–4 green and written approval | Browser confirm or flag on without that approval |
| Checkpoint 6 — before merge | A–C code green, flag default off, pre-merge review has no critical token or cross-company gap | Production migrate or flag enabled in production |

This planning update: **GO to review the plan for implementation. NO-GO to write code, schema files, or migrations in the same step.**

---

## 23. Docs consulted

- `cobo_iam_services/docs/ai-cache/README.md`
- `cobo_iam_services/docs/ai-cache/cms-template-import-history-and-json-guide-solution-2026-09-27.md`
- Prior revisions of this plan
- `docs/schema/template-import-v1.schema.json` in both repos
- `docs/deploy-dev-guide.md` (not executed)
- Import service, validator, normalizer, `deadline_rule_compatibility.go`, confirm, token, example handler, `ListTemplateDepartments`, `platform/errors`, `workflowdept/flags.go`, `workflowstepcomments` `FEATURE_DISABLED`
- Frontend import screen, import API, record history tab, editor view model, `templateValidation.ts`, `types.contract.test.ts`
- `migrations/run_dev_migrations.sh` through `0147`

## 24. Cache metadata

| Field | Value |
|---|---|
| Path | `cobo_iam_services/docs/ai-cache/cms-template-import-history-and-json-guide-implementation-plan-2026-09-27.md` |
| Task type | Implementation plan, decisions closed |
| Decisions | Retention 365 days; Option A attempt id; conditional periodicity |
| Implemented | Documentation only |
| Branch | `fixbug/test-9` |
| Open decisions | 0 |
| Verification | Docs and prior source reads. No build, no migration, no DEV |

```text
PLAN_STATUS=READY_FOR_IMPLEMENTATION_REVIEW
IMPLEMENTATION_STARTED=false
SOURCE_CODE_CHANGED=false
SCHEMA_RUNTIME_CHANGED=false
MIGRATION_CREATED=false
MIGRATION_APPLIED=false
DEV_TOUCHED=false
PRODUCTION_TOUCHED=false
FRONTEND_CODE_CHANGED=false
OPEN_DECISIONS=0
```
