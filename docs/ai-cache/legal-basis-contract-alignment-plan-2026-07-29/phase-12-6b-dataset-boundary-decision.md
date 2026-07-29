# Phase 12.6B — Dataset boundary decision

**Decision status:** **LOCKED_ALL_6**
**Date:** 2026-07-29
**Environment:** DEV (`cobo_iam`)
**Owner confirmation:** User selected ALL_6 in Plan Mode Q&A.

## Options analyzed

| Option | Description | Pros | Cons |
| --- | --- | --- | --- |
| **A. All 6 versions** | Every Group A `(type_id, version_no)` from 12.6A | Uniform contract; no historical leftovers; 6==auto-eligible | Slightly wider blast radius than active-only |
| **B. Active only** | `version_no == active_version_no` | Smaller | Historical versions (if any later) stay flat-only; split contract |
| **C. API-read path only** | Versions currently returned by API list/detail | Matches UX | Ambiguous for version-history endpoints; harder to prove exhaustiveness |
| **D. Active first, historical later** | Two windows | Safer sequencing on large DEVs | Extra ops cost; on this DEV all 6 are already active |

## DEV facts (RO recheck)

- Total versions = **6**; all Group A; B=C=D=E=0
- Each row: `version_no=1`, `active_version_no=1`, `company_id` empty (GLOBAL), `status=active`
- `legal_bases_json` IS NULL; no malformed/overflow/Group D
- `is_released` column **absent** — not used for filtering; migration 0122 **not** in this backfill

## Recommendation → LOCK

**LOCKED_ALL_6** — backfill exact allowlist of 6 Group A records.

### Rationale

1. All 6 auto-eligible WRAP_LEGACY_FLAT; zero anomalies.
2. On this DEV, ALL_6 coincides with active-only — locking ALL_6 still prevents split semantics if extra historical versions appear before apply (freshness stop).
3. Version-detail / catalog still serve `disclosure_type_versions` rows; leaving flat-only on some versions while others wrap creates Group A vs C divergence across history.
4. Rollback complexity stays bounded (exactly 6 snapshot rows).
5. Missing `is_released` does **not** block ALL_6: primary key identity is `(type_id, version_no)`.

## Rejected for this DEV window

- `LOCKED_ACTIVE_ONLY` — redundant today; weaker for future historical rows without a second plan.
- `BLOCKED_OWNER_DECISION` — owner already chose ALL_6.
- `BLOCKED_SCHEMA_CONFLICT` — not triggered; `legal_basis` + `legal_bases_json` present.

## Freshness implication

If a pre-apply inventory shows total≠6, any non-A row in the six keys, or extra versions for these types, stop with `STALE_DRY_RUN` / re-open boundary.
