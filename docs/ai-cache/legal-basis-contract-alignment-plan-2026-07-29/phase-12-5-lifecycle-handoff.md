# Phase 12.5 — Legal Basis lifecycle handoff

## 1. Executive summary

Phase 12.5 fixes the **new version** wipe/ID gap: when creating an independent `disclosure_type_versions` row, structured `legal_bases` are deep-copied from the prior version (or regenerated from provided payload) with **new UUIDs**, order/metadata preserved, projection refreshed. Same-draft preserve, publish/activate/archive (non-mutating), and company flat public APIs remain. Clone / global→company **paths do not exist** — documented N/A, not invented. Verdict: **PASS_WITH_NON_BLOCKING_GAPS**.

## 2. Source baseline

- Repo: `cobo_iam_services` @ `25351bd` → impl commit below
- Branch: `recovery/lost-changes-audit-20260717-153324`
- Go: go.mod `1.23.0` / runtime `go1.24.9`
- FE Phase 12.4: `4c4fe37`

## 3. Contract references

`02`–`05`, `07`, `10`, `12`–`18`, OD-1/OD-3, Phase 12.2–12.4 handoffs.

## 4. Source discovery

`phase-12-5-source-discovery.md`

## 5. Lifecycle ownership matrix

`phase-12-5-lifecycle-matrix.md`

## 6. No-op / minimal-fix gate

User: **minimal_fix** + **N/A for missing clone/copy**. Fix class **C** new-version only.

## 7. Same-version update

Unchanged: `PreserveLegalBases && overwriteDraft` reloads JSON; client IDs preserved on provided write. Tests cover edit/reorder/preserve.

## 8–10. Publish / Activate / Archive

Status/pointer only — no LB rewrite. Activate covered in integration test.

## 11. New version

`PrepareLegalBasesForNewVersion` in mysql + inmemory when `!overwriteDraft` and prior version ≥1.

## 12–13. Clone / Global→company

**N/A** — no code paths; not invented.

## 14. Company edit

Public flat create/update unchanged; UPDATE does not wipe `legal_bases_json`.

## 15. ID semantics

Same draft: preserve. New version: regenerate via `idgen.UUIDv7Generator`.

## 16. Deep-copy

`DeepCopyLegalBases` — new slice; value structs; isolation tests PASS.

## 17. Projection

Reuse `ProjectLegalBasesToLegacy` (OD-7). No duplicated algorithm.

## 18. Legacy-only

Preserve path with empty structured → copy flat only; no metadata parse.

## 19. Divergence / malformed

Drop empty items on target copy only; source DB not auto-repaired.

## 20. Transactionality

Logic inside existing Upsert transaction / in-memory lock — no new distributed tx.

## 21. Observability

Events: `legal_basis_lifecycle_ids_regenerated`, `legal_basis_lifecycle_preserved`, `legal_basis_lifecycle_malformed_items_dropped`, `legal_basis_lifecycle_divergence_normalized_in_target` (hashes only).

## 22. Files changed

See `phase-12-5-diff-audit.md`.

## 23–24. Tests

Unit + inmemory integration PASS. See JSON evidence files.

## 25. Full test / vet / build

- `go test ./internal/disclosure/...` PASS
- `go build ./...` PASS / `go vet ./...` PASS
- `go test ./...` pre-existing fails: companyaccess, httpserver, notification — **unrelated**

## 26. Diff/scope

PASS — no FE/migration/Docker/company DTO expansion.

## 27. Commits

- Implementation (BE): `0c6dcca` — `fix(legal-basis): preserve data across template lifecycle`
- Evidence (BE): `bf3f6ae` — `docs(legal-basis): add Phase 12.5 lifecycle evidence`
- Evidence (FE plan folder): `6265bf9` — same message

## 28. Remaining Phase 12.6 gaps

Backfill / dry-run migration for Group A–E — **out of 12.5**.

## 29. Evidence paths

`docs/ai-cache/legal-basis-contract-alignment-plan-2026-07-29/phase-12-5-*` (canonical under `cobo_web_design`; discovery mirrored under BE).

## 30. Verdict

**PASS_WITH_NON_BLOCKING_GAPS**

## 31. Awaiting user confirmation

Stop before Phase 12.6.
