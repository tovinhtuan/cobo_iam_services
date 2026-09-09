# Pointer — Tenant deadline summary October past-date audit

Canonical evidence pack:

`cobo_web_design/docs/ai-cache/tenant-deadline-summary-october-past-date-audit-2026-09-09/`

BE source anchors:
- `internal/disclosure/app/service.go` — `GetTypeDetail` → `CalculateDeadlineSummary`
- `internal/disclosure/app/deadline_calculator.go` — `computeCycleStart` / `ResolveLogicalSlot(now)` (no ApplicableFrom)
- `internal/disclosure/app/periodic.go` — `seedPeriodicCycles` ApplicableFrom gate (current slot only)
- `internal/disclosure/app/applicability_state.go` — UPCOMING when current slot &lt; AF

```text
ROOT_CAUSE=OTHER (DeadlineSummary ignores ApplicableFrom)
AUDIT_RESULT=PO_DECISION_REQUIRED
NO_IMPLEMENTATION
```
