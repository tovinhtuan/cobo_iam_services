# Phase 12.6B-Plan — Source discovery

**Type:** planning / docs only
**Baseline BE HEAD (at plan write):** `ddf3316` (dirty tree may include uncommitted 12.6A tool)
**FE plan mirror branch:** `recovery/lost-changes-audit-20260717-153324`
**12.5 ancestor:** `0c6dcca` ⊆ HEAD — YES

## Contracts reused (not rewritten)

- `02-adr-source-of-truth.md` … `18-implementation-readiness.md`
- OD-1…OD-7 LOCKED (`14`, `15`)
- Group A wrap + OD-7 projection (`08`, `17`)
- Phase 12.6A evidence (`phase-12-6a-*`) — operational PASS / governance FAIL_SCOPE_CREEP

## Persistence identity

| Field | Source |
| --- | --- |
| PK | `(disclosure_type_versions.type_id, version_no)` |
| Scope | `disclosure_types.company_id` NULL → GLOBAL |
| Flat | `legal_basis` TEXT |
| Structured | `legal_bases_json` JSON |
| Actor column | `updated_by` VARCHAR — backfill will set system actor |
| Activation | `activated_at` — **must not change** on backfill |
| `is_released` | Missing on DEV — out of scope (no 0122) |

## Runtime helpers to reuse in future apply (design only)

- `ProjectLegalBasesToLegacy` — OD-7
- `ValidateLegalBasesForWrite` + `idgen.UUIDv7Generator`
- Inventory package `legal_basis_inventory` — RO classify / freshness
- JSON marshal of `[]LegalBasisDTO` — **no hand-concat**

## Explicit non-goals this phase

- No `--apply` implementation or execution
- No Docker / migrate / deploy
- No CMS/tenant/API runtime edits
- No production
