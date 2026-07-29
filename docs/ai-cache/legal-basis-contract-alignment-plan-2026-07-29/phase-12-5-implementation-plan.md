# Phase 12.5 — Implementation plan

## Decision

**MINIMAL_FIX** (user):

1. Do **not** add Clone or Global→company endpoints (N/A).
2. Fix **new version** path so `legal_bases` are not wiped and IDs are regenerated on independent versions.
3. Keep same-draft preserve behavior (already A).
4. Add unit + in-memory integration tests for ID/data/order/projection/isolation.
5. Evidence in plan folder; stop before 12.6.

## Code changes

### App helpers (`legal_basis_lifecycle.go`)

- `DeepCopyLegalBases` — new slice of value structs
- `RegenerateLegalBasisIDs` — new UUID per item via `idgen.Generator`
- `PrepareLegalBasesForNewVersion(sourceBases, sourceFlat, providedBases, preserve, idg)`:
  - If preserve: normalize/drop empty → deep-copy → regen IDs → project flat (fallback source flat if project fails) → emit `legal_basis_lifecycle_ids_regenerated`
  - Else if provided non-empty: deep-copy → regen IDs (projection already set by Resolve when flag ON)
  - Else explicit clear `[]`: leave empty
  - Legacy-only source (no valid structured): copy flat only; structured `[]` — **no metadata parse**

### MySQL + in-memory `UpsertTypeVersion`

When `!overwriteDraft` and prior version exists (`maxVersion >= 1`):

- If `PreserveLegalBases`: load prior `legal_bases_json` (+ flat) → `PrepareLegalBasesForNewVersion`
- Else if client provided non-empty structured: regen IDs on copy
- Marshall resulting slice

When `overwriteDraft`: unchanged preserve/client merge.

Repos: use `idgen.UUIDv7Generator{}` inside repos (repos lack injected Generator today) **or** pass through request. Prefer app helper + local `UUIDv7Generator` in repo call site to avoid constructor churn.

## Non-goals

CMS/FE, migration, Docker, company public structured DTO, `source_legal_basis_id`, invent clone/copy APIs.
