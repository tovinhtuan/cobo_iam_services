# 05 — Due-date source alignment (gate)

## Deadlinealerts terminal due

`resolveDueDateAndStatus` when ConfirmedAt != nil OR terminal record:

```text
due = firstNonEmpty(PlannedDate, AdHocDeadlineDate)
```

(Empty → today fallback for **status display only**.)

## Completion KPI approach

1. List `DONE` (+ reuse `PENDING_CONFIRM` already fetched) via **same** `ListDeadlineAlerts` → `DueDate` already resolved by deadlinealerts.
2. Join `disclosure_records.completed_at` by `record_id` (read-only).
3. Exclude rows with empty due or missing `completed_at` (no today-fallback in sample — product requires valid due_at/completed_at).

## Verdict

**ALIGNED** — completion due uses deadlinealerts-resolved `DueDate`, not a parallel calculator. No calculator/write-path changes.

Not `BLOCKED_DUE_DATE_SOURCE_ALIGNMENT`.