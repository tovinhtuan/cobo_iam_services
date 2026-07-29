# Phase 12.2 — Source discovery

**Date:** 2026-07-29  
**Baseline:** `b164b64` / `recovery/lost-changes-audit-20260717-153324`  
**Go:** go.mod `1.23.0` (runtime go1.24.9)  
**Working tree:** clean at start

## Contract references (SoT)

- `02`–`05`, `07`–`10`, `12`–`18`, `phase-12-1a-results.json`, `plan-results.json`
- Priority: Phase 12.1A final docs over prompt summary

## Prompt vs docs (resolved, non-blocking)

| Topic | Prompt | Final docs | Resolution |
| --- | --- | --- | --- |
| null vs empty strings | `string\|null` | persist `""` | **Docs win** — empty string |
| Error code | VALIDATION_ERROR example | VALIDATION_ERROR | Use `INVALID_REQUEST` + `Details.field` / `field_errors` to match existing envelope; also set `Details.field` path. Document: project machine code remains `INVALID_REQUEST` (existing); path is machine-readable. **OR** add `VALIDATION_ERROR` code if we can without breaking clients — prefer Details.field + INVALID_REQUEST to avoid inventing new top-level code that FE doesn't know. **Decision:** use existing `newValidationError` → `INVALID_REQUEST` + `field_errors` map keys as paths (`legal_bases[2].link`). Not BLOCKED — contract path semantics preserved. |
| null vs omitted array | distinguish | Go `*[]T`: null≡omitted | Document: `null` ≡ omitted (preserve); `[]` = clear |

No `BLOCKED_CONTRACT_CONFLICT`.

## Persistence

| Column | Table | Notes |
| --- | --- | --- |
| `legal_basis` | `disclosure_type_versions` | TEXT NULL |
| `legal_bases_json` | `disclosure_type_versions` | JSON (migration 0019) |

## DTO / models

| Type | Fields | File |
| --- | --- | --- |
| `LegalBasisDTO` | id,title,code,authority,issue_date,summary,link | `contracts.go` |
| `UpsertTypeVersionRequest` | `LegalBasis` string + `LegalBases []LegalBasisDTO` | `contracts.go` |
| `DisclosureTypeDTO` | both | `contracts.go` |
| `CreateCompanyTemplateRequest` / `UpdateCompanyTemplateRequest` | **`legal_basis` only** (deferred structured) | `contracts.go` |

## Existing helpers

| Helper | Behavior | Gap |
| --- | --- | --- |
| `sanitizeLegalBases` | keep title∨summary; trim; no limits/URL/date/dup/projection | incomplete vs 12.1A |
| `ApplyTemplateFlatBlockSync` | syncs flat ↔ block `legal_basis` | keep; projection must set flat before sync |

## Feature-flag framework

`internal/platform/config` + `ServiceOption` pattern (e.g. `WithDeadlineEngineV2Shadow`). Wire new bools from env.

## Observability

No dedicated metrics bus for legal basis. Use structured `log` / `fmt` style consistent with `deadline_engine_shadow` — stable event codes, no legal text. Limitation: log-only if no prometheus counter exists.

## Endpoint matrix

| Endpoint | Method | Request | Response | Current legal_basis | Current legal_bases | Phase 12.2 impact |
| --- | --- | --- | --- | --- | --- | --- |
| `/api/v1/admin/disclosure-types/{type_id}` | PUT | `UpsertTypeVersionRequest` | upsert meta | persist as sent; block sync | sanitize + persist JSON | **Write precedence + validation + presence** |
| `/api/v1/admin/disclosure-types/{type_id}/versions/{n}` | GET | — | `DisclosureTypeDTO` | raw DB | raw JSON | **Read precedence** |
| `/api/v1/disclosure-types/{type_id}` | GET | — | `DisclosureTypeDTO` | raw DB | raw JSON | **Read precedence** |
| `/api/v1/disclosure-types` list | GET | — | list items | usually no full bases | check | **No change unless exposes fields** |
| Activate version | POST | version | — | copies via activate path | copies JSON as stored | shared read only; regen IDs → Phase 12.5 gap |
| Company create/update | POST/PATCH | flat only | write resp | flat persist | none on request | **Keep flat; read synthesize if detail used** |
| Clone / new version | service | — | — | deep copy rows | deep copy JSON | **Gap → 12.5** if ID regen missing |

## Write presence truth table (Upsert PUT)

| `legal_bases` JSON | `legal_basis` | Flag OFF | Flag ON |
| --- | --- | --- | --- |
| omitted / null | any | Preserve existing bases on draft overwrite; new insert `[]`; persist flat as today + block sync | Legacy path: persist flat; **wrap** one OD-2 item into bases; emit legacy-write |
| `[]` | any | Persist empty bases + flat | Clear bases; derive projection `""`; ignore client flat |
| `[{…}]` | any / diverge | Light sanitize persist both (compat; no force projection) | Strict validate; ignore client flat; derive projection; persist both |

## Read algorithm (all detail paths)

Apply `ApplyLegalBasisReadCompat(dto)` after load:

1. NormalizeForRead (drop invalid empty items; metric)
2. If ≥1 → bases=normalized; flat_response=Project (even if DB flat diverges); emit divergence if mismatch; **no DB write**
3. Else if fallback ON && flat≠"" → synthesize OD-2; response flat = normalized flat
4. Else → `[]` + empty flat

## Lifecycle ownership

Phase 12.2: shared mapper foundation only. ID regen on clone/new version/global→company → **Phase 12.5 gap** if not already correct.

## Company lock

Do **not** add `legal_bases` to company request DTOs.
