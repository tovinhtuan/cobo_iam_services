# 01 — Service pipeline before / after

## BEFORE

```text
ListRows (Phase 1 membership)
  → isDraftRecordStatus → continue   // discarded actionable Draft
  → AllowsRow
  → resolveDueDateAndStatus
  → skip if due empty
  → request filters
  → enrich DTO
  → page/total from enriched
```

## AFTER

```text
ListRows (Phase 1 membership — unchanged)
  → AllowsRow
  → resolveDueDateAndStatus
  → skip if due empty
  → request filters
  → enrich DTO (confirmation/status preserved)
  → page/total from enriched
```

```text
GO_DRAFT_MEMBERSHIP_FILTER_REMOVED=true
DUE_RESOLUTION_CHANGED=false
DEADLINE_STATUS_LOGIC_CHANGED=false
CONFIRMATION_LOGIC_CHANGED=false
```
