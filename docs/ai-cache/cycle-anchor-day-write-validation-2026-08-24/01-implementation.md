# 01 — Implementation

```text
CHANGE_TYPE=SERVER_CONTRACT_HARDENING
PRODUCT_BEHAVIOR_CHANGE=false  # only rejects previously-invalid writes
```

## Files

| File | Change |
|------|--------|
| `internal/disclosure/app/effective_schedule.go` | `CycleAnchorDayMin/Max`, `ValidateCycleAnchorDay` |
| `internal/disclosure/app/service.go` | call validator on CMS config update, CMS upsert, company prefs |
| `internal/disclosure/app/contracts.go` | comment clarify write vs clamp |
| `internal/disclosure/app/cycle_anchor_day_validation_test.go` | unit + service tests |

## Non-changes

ClampDayOfMonth, deadline formula, OpenAt, worker, FE, DB schema, migrations.
