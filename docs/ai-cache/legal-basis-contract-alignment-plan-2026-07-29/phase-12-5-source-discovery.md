# Phase 12.5 — Source discovery (Legal Basis lifecycle)

**Date:** 2026-07-29  
**Repo:** `cobo_iam_services`  
**Baseline:** branch `recovery/lost-changes-audit-20260717-153324`, HEAD `25351bd`  
**Go:** go.mod `1.23.0` / runtime `go1.24.9`  
**Dirty tree:** clean  

**User gate:** Clone / Global→company endpoints **do not exist** → document **N/A no-op**; do **not** invent endpoints. Minimal fix = new-version deep-copy + ID regen (+ tests).

## Operations found

| # | Operation | Entry | Exists? |
| --- | --- | --- | --- |
| 1 | Create draft / first version | `PUT .../disclosure-types/{id}` → `UpsertTypeVersion` | Yes |
| 2 | Update same open draft | Same PUT overwrite branch | Yes |
| 3 | Publish | Company `TransitionCompanyTemplateLifecycle(publish)` — `review_status` only | Yes (no LB mutate) |
| 4 | Activate | `ActivateTypeVersion` — pointer/`is_released` only | Yes (no LB mutate) |
| 5 | Archive | CMS archive / company archive — status only | Yes (no LB mutate) |
| 6 | Create new version | Same PUT when no open draft → `MAX(version)+1` INSERT | Yes |
| 7 | Clone template | — | **No** |
| 8 | Global → company copy | — | **No** |
| 9 | Company create/edit | Portal create/update — **flat `legal_basis` only** | Yes |

## Domain / persistence

- DTO: `LegalBasisDTO` (`contracts.go`)
- Columns: `legal_basis` TEXT + `legal_bases_json` JSON
- Compat: `legal_basis_compat.go` (12.2) — Preserve on same-draft overwrite only
- ID fill blanks: `ValidateLegalBasesForWrite` + `idgen.Generator.NewUUID()`
- No force-regen helper for version fork yet

## Critical gap

`PreserveLegalBases && overwriteDraft` only. On **new version INSERT**, omit → marshal `[]` → **wipes** previous structured JSON.

## Shared mappers

`ResolveLegalBasisWrite` used for all upserts. No clone/copy mapper.

## Company public request

Still flat-only. Update does not touch `legal_bases_json` (leaves as-is). Create never sets JSON.

## Tests today

`legal_basis_compat_test.go` unit; no lifecycle ID/preserve/new-version integration tests for LB.
