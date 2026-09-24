# Global CMS Record — frozen API/permission contract (T1)

```text
date: 2026-09-24
status: frozen for implementation
plan: cms-template-record-history-implementation-plan-2026-09-24.md (v3)
```

## Permissions (server-enforced)

| Code | Action |
|------|--------|
| `cms.record.read` | List/get global records + company children summary |
| `cms.record.write` | Create/update **Draft** only |
| `cms.record.publish` | Publish (`Draft→Published`) and Archive |
| `cms.record.materialize` | Preview/run materialization |

Gate: also require `platform.cms.view`.

**Not used for global:** `cms.record.submit`, `cms.record.approve`, `disclosure.approve`.

Grant tier: `system_only` (same as `cms.template.*`). Seed: grant to roles that already have `platform.cms.view`.

## Status

Global: `Draft` | `Published` | `Archived`  
Company (materialized from Global CMS Record): storage + API = **`NotStarted`** (canonical).  
Legacy Portal `CreateRecord`: remains **`Draft`**.  
Submit accepts `Draft` **or** `NotStarted` → `PendingReview`.  
Company Workflow Queue lists only company `PendingReview`/`Submitted` — never Global Records / `NotStarted`.

### company_counts buckets

`total`, `not_started` (Draft **or** NotStarted), `in_progress`, `pending_review`, `approved`, `published`, `completed`, `failed`.

## cycle_key

Regex (HCM business period, not deadline):

- `daily:YYYY-MM-DD`
- `weekly:YYYY-Www` (ISO week)
- `monthly:YYYY-MM`
- `quarterly:YYYY-Qn` (n=1..4)
- `yearly:YYYY`

Event/ad-hoc: out of phase 1 (reject create).

**Note:** `cycle_key` frequency should match template `frequency_unit` so `applicable_from` / `applicable_to` compare slots correctly.

## Endpoints

| Method | Path | Perm |
|--------|------|------|
| GET | `/api/v1/platform/cms/templates/{template_id}/records` | read |
| POST | `/api/v1/platform/cms/templates/{template_id}/records` | write |
| GET | `/api/v1/platform/cms/records/{record_id}` | read |
| PUT | `/api/v1/platform/cms/records/{record_id}` | write (Draft only) |
| POST | `/api/v1/platform/cms/records/{record_id}/publish` | publish |
| POST | `/api/v1/platform/cms/records/{record_id}/archive` | publish |
| GET | `/api/v1/platform/cms/records/{record_id}/company-records` | read |
| POST | `/api/v1/platform/cms/records/{record_id}/materialize` | materialize |

### Publish idempotency

If already `Published` → **200** with current DTO (no-op).  
If `Archived` → **409**.

### Archive vs unique

On archive, `cycle_key` is rewritten to `{original}#archived:{id}` so `(template_id, cycle_key)` unique allows a new Draft for the same business cycle.

### Materialize body

```json
{ "mode": "full"|"incremental", "company_ids": [], "dry_run": false }
```

Empty `company_ids` → resolve via shared eligibility resolver (below).  
Non-empty → re-evaluate each id; non-eligible skipped with reason (no trust of client list / no platform bypass).

### Eligibility (shared preview + actual)

A company is eligible when:

```text
company.active
AND subscription status IN (ACTIVE, TRIAL) within effective window
AND applicability_rules match company profile
AND company_type_preferences.auto_create_enabled != false  (missing row = true)
AND applicable_from / applicable_to allow the cycle slot (Asia/Ho_Chi_Minh)
AND not already linked (cms_record_id, company_id)
```

Stable skip reason codes: `COMPANY_INACTIVE`, `ENTITLEMENT_MISSING`, `APPLICABILITY_NOT_MATCHED`, `AUTO_CREATE_DISABLED`, `NOT_YET_APPLICABLE`, `NO_LONGER_APPLICABLE`, `ALREADY_MATERIALIZED`, `ELIGIBLE`.

### Materialize result item

Includes `outcome` (`created|exists|skipped|failed`), `reason_code`, optional `error_message` / `company_record_id`.

## Schema follow-up

- `0142`: tables + `disclosure_records.cms_record_id` + permissions.
- `0143`: widen global/materialize IDs to `VARCHAR(64)` for prefixed UUIDv7 (`cms_rec_<uuid>`).

## Legacy

`/api/v1/platform/cms/entries*` remains company-scoped `disclosure_records`. Unchanged semantics.
