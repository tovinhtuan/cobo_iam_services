# 08 — Migration / backfill plan

**Status:** LOCKED (Phase 12.1A) — see also `17-migration-decision-matrix.md`
**Timing:** OD-4 Hybrid — dual-read → structured write → dry-run → A/B/C/E; **D manual only**

## Groups

| Group | Detection | Auto mutate? | Action | Canonical result |
| --- | --- | --- | --- | --- |
| **A** | flat non-empty AND bases empty | **Yes** (after dry-run) | One item: title=`Cơ sở pháp lý`, summary=full text, others `""`; generate id | bases len≥1 |
| **B** | flat empty AND bases non-empty | **Yes** | Derive flat=OD-7 projection; sync block | Consistent |
| **C** | both present AND flat == projection | **Yes** | Keep structured; normalize projection/block | Already OK |
| **D** | both present AND diverge | **No** | Report only; runtime structured wins; manual BA | No silent overwrite |
| **E** | both empty | No-op | — | Empty |

## Phase 12.6A note (2026-07-29) — CLOSED

Read-only inventory CLI: `cmd/legal-basis-inventory` (no `--apply`).
**Operational:** `PASS_READ_ONLY_DRY_RUN` (DEV Docker config + RO txn + SQL allowlist; mutations=0).
**Governance:** `FAIL_SCOPE_CREEP` — see `phase-12-6a-scope-exception.md` (`docker compose … build api`).
Inventory VALID; re-run not required solely for that exception.

## Phase 12.6B-Plan note (2026-07-29)

Controlled DEV backfill **plan only** — see `phase-12-6b-*`.
**Boundary:** `LOCKED_ALL_6` · exact allowlist · snapshot/CAS/all-or-nothing txn designed.
**Verdict:** `BACKFILL_PLAN_READY` — **no mutation yet**; apply requires explicit user phrase + Approvals 3–4.
Do **not** run migration `0122` as part of 12.6B.

- **No** regex parsing of Thông tư/NĐ codes from blob.
- Idempotent: re-run skips already-backfilled A (non-empty bases).
- Dry-run = report only (type_id, version_no, group, lengths, hashes).
- Batch for this DEV window: **exact 6** (not wildcard 100).
- Snapshot before mutate (secure path outside git).
- **DEV first**, never production in Phase 12.

## Rollback

- Pre-backfill snapshot of `(type_id, version_no, legal_basis, legal_bases_json)`.
- Rollback = restore from snapshot (exact allowlist CAS).
- Do not use docker/compose for migration / verify builds in apply window.

## Sequence (LOCKED)

1. Ship dual-read
2. Enable structured write
3. Dry-run
4. Review Group D
5. Backfill A/B/C/E
6. Manual decision for D

## Rejected alternative

Permanent no-backfill-only is allowed as product choice later, but **default plan includes** A/B/C/E after dry-run (OD-4).

## Phase 12.6B-I (2026-07-29)

Guarded backfill tooling implemented (`legal_basis_backfill` + cmds). Verdict **TOOL_READY_FOR_CONTROLLED_EXECUTION**. DEV apply **not** executed; SQL DEV wiring deferred to 12.6B-E.
