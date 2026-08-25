# 00 — Source reconciliation (Phase 2)

```text
TASK=Deadline Alert V1 Phase 2 — service integration
PHASE_1_SQL_MEMBERSHIP_LOCKED=true
```

## Actual symbols

```text
ACTUAL_SERVICE_PATH=internal/deadlinealerts/app/service.go
ACTUAL_SERVICE_SYMBOL=ListDeadlineAlerts
REPOSITORY_CALL_SITE=s.repo.ListRows(ctx, companyID, accessScope) ~L42
CURRENT_GO_DRAFT_FILTER=isDraftRecordStatus(row.RecordStatus) → continue  (REMOVED in Phase 2)
CURRENT_DUE_RESOLVER=resolveDueDateAndStatus
  planned_date → AdHocDeadlineDate → dueDateFromTypeConfig
CURRENT_STATUS_DERIVER=
  ConfirmedAt → DONE
  isTerminalRecordStatus → PENDING_CONFIRM
  else remainingDays → UPCOMING|DUE_SOON|OVERDUE
CURRENT_CONFIRMATION_LOGIC=ConfirmedAt on AlertRow (from deadline_alert_confirmations JOIN)
```

## Go membership filters audited

```text
GO_MEMBERSHIP_FILTERS=
1. isDraftRecordStatus skip — REMOVED (was duplicate of Phase 1 SQL)
2. accessScope.AllowsRow — KEPT (access, also SQL-scoped)
3. dueDate == "" skip — KEPT (renderability, not obligation membership)
4. status/query/date/dept/displayGroup request filters — KEPT

DRAFT_SPECIFIC_FILTERS=
- isDraftRecordStatus (removed)

OTHER_STATUS_ALLOWLISTS=
- none for membership
- isTerminalRecordStatus only for PENDING_CONFIRM presentation
```

## Count / pagination

```text
SERVICE_COUNT_LIST_PARITY_BEFORE=total=len(enriched) after Draft skip; page slice of enriched
SERVICE_COUNT_LIST_PARITY_AFTER=same architecture; Draft no longer skipped → natural count includes Draft
SERVICE_SORTING_CHANGED=false (repository ORDER BY; service preserves list order)
```

## Inmemory

```text
INMEMORY_CHANGE_REQUIRED=false
(inmemory ListRows returns nil; service tests use stubRepo)
```

## Interface / API

```text
REPOSITORY_INTERFACE_CHANGED=false
API_HANDLER_CHANGED=false
API_SCHEMA_CHANGED=false
```
