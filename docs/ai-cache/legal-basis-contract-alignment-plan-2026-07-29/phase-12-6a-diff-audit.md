# Phase 12.6A — Diff audit

## Allowed (this phase)

| Path | Reason |
| --- | --- |
| `cmd/legal-basis-inventory/` | Read-only inventory CLI (no --apply) |
| `internal/disclosure/app/legal_basis_inventory/` | Classify/dry-run helpers + tests |
| `docs/ai-cache/.../phase-12-6a-*` | Evidence |

## Forbidden — unchanged

Migration apply, backfill executor, FE, Docker/deploy, runtime API behavior, company DTO.

## Verdict

Tooling scope **PASS**. Overall phase verdict **BLOCKED_READ_ONLY_ACCESS** (no DEV inventory run).
