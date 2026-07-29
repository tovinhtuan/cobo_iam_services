# Phase 12.6B — Backfill design (Group A wrap)

**Status:** DESIGN LOCKED (not implemented)
**Environment:** DEV only
**Records:** exact 6 from `phase-12-6b-record-allowlist.json`

## Input → output (per record)

| | Before (Group A) | After (expected Group C) |
| --- | --- | --- |
| `legal_basis` | non-empty flat (hash = allowlist) | OD-7 projection = `"Cơ sở pháp lý"` (hash = `06a6705c2f2481b5`) |
| `legal_bases_json` | NULL / no valid items | JSON array length 1 |
| Item `id` | n/a | new `idgen.UUIDv7Generator` UUID |
| Item `title` | n/a | `"Cơ sở pháp lý"` |
| Item `summary` | n/a | **exact** original flat text (byte-preserving UTF-8) |
| Item `code`,`authority`,`issue_date`,`link` | n/a | `""` |
| `updated_by` | prior | `system:legal-basis-backfill-12.6b` |
| `activated_at` | preserve | **unchanged** |

## Hard rules

1. Do **not** persist display id `*-lb-legacy-1`.
2. Do **not** parse metadata from free text / split / truncate.
3. Do **not** reconstruct summary from post-projection (title-only).
4. Serialize via `[]LegalBasisDTO` + `json.Marshal` + `ValidateLegalBasesForWrite` — no hand-built JSON strings.
5. Flat after wrap **must** equal `ProjectLegalBasesToLegacy(items)` (user-locked; differs from runtime `ResolveLegalBasisWrite` KEEP-flat path).
6. Algorithm version stamp in evidence: `v1-wrap-legacy-flat`.

## Pre-apply freshness (mandatory)

1. RO inventory re-run.
2. Exact 6 allowlist keys exist; each still Group A.
3. Flat hash match; structured still empty.
4. No new malformed / overflow / Group D.
5. Schema: `legal_basis` + `legal_bases_json` present; if `is_released` newly appears → STOP (`STALE_DRY_RUN` / schema watch).

Any mismatch → **STOP** (`STALE_DRY_RUN`); no mutation.

## Feature flags

Do **not** enable `VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED` or backend structured-write env changes in this apply window. Flag rollout = Phase 12.7+.
