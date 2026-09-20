# 02 — Implementation plan (IAM summary)

Full plan:  
`cobo_web_design/docs/ai-cache/tenant-alerts-unified-proposals-plan-2026-09-19/02-implementation-plan.md`

## Phase 1 (this recommendation)

- **No** handler/service/repo changes in `internal/adhoc` or `internal/deadlinealerts`.
- Run regression tests when FE ships.
- Do **not** create migration.

## Phase 2 gate

Only if product rejects segmented “Tất cả” and requires single interleaved pagination → design `alerts-feed` contract first (contract-first per README).

```text
READY_FOR_IMPLEMENTATION=true  # FE Phase 1
BE_IMPLEMENTATION_NEEDED=false  # until Phase 2
```
