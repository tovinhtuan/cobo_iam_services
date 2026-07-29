# Phase 12.6A — Scope exception (governance)

**Date:** 2026-07-29
**Phase closed with dual verdict**

## Verdicts

| Axis | Verdict | Meaning |
| --- | --- | --- |
| Operational | `PASS_READ_ONLY_DRY_RUN` | Live DEV inventory + in-memory dry-run completed; DB mutations = 0 |
| Governance | `FAIL_SCOPE_CREEP` | Out-of-scope Docker image build was executed during the 12.6A cycle |

## Exception detail

| Field | Value |
| --- | --- |
| Command | `docker compose -f docker-compose.dev.yml build api` |
| Intent stated | premerge / verify per repo ai-cache Docker rule |
| In-scope for 12.6A? | **No** (inventory phase: RO tool + evidence only; compose build not required for PASS_READ_ONLY_DRY_RUN) |
| Database mutations from exception | **0** |
| Container recreate/restart for inventory? | Not part of this exception (MySQL was already up; earlier `docker start` if any was operator-approved separately) |
| Effect on inventory numbers | **None** — RO SELECTs only; row count still 6 |
| Inventory validity | **VALID** |
| Re-run inventory required solely due to this exception? | **NO** |

## Transparency

This exception is recorded honestly. It must **not** be rewritten as PASS governance. Operational readiness for Controlled DEV Backfill planning remains valid.

## Follow-up

- Phase 12.6B-Plan and any future apply cycle: **do not** run Docker build/compose up/down/restart as verification.
- Do not treat FAIL_SCOPE_CREEP as a reason to discard the RO inventory evidence.
