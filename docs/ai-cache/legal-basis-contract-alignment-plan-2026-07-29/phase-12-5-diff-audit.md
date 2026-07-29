# Phase 12.5 — Diff audit

## Allowed (touched)

| Path | Reason |
| --- | --- |
| `internal/disclosure/app/legal_basis_lifecycle.go` | Deep-copy + ID regen helpers |
| `internal/disclosure/app/legal_basis_lifecycle_test.go` | Unit tests |
| `internal/disclosure/infra/mysql/repository.go` | New-version LB prepare |
| `internal/disclosure/infra/inmemory/repository.go` | Same semantics |
| `internal/disclosure/infra/inmemory/legal_basis_lifecycle_test.go` | Integration tests |
| `docs/ai-cache/legal-basis-contract-alignment-plan-2026-07-29/phase-12-5-*` | Evidence |

## Forbidden — unchanged

- FE CMS / tenant
- Migration / schema
- Docker / deploy
- Company public request DTO (`legal_bases[]` not added)
- `source_legal_basis_id` / `is_legacy_projection`
- Clone / global→company **new endpoints** (explicitly not invented)
- Unrelated RBAC/workflow

## Verdict

**PASS** — no FAIL_SCOPE_CREEP.
