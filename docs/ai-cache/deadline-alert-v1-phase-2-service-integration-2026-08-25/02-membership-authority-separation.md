# 02 — Membership authority separation

```text
MEMBERSHIP_AUTHORITY=SQL_REPOSITORY (Phase 1 ListRows)
SERVICE_RESPONSIBILITY=DUE_RESOLUTION_STATUS_DERIVATION_CONFIRMATION_PRESENTATION
NO_DUPLICATE_MEMBERSHIP_FILTER_IN_GO=true
GO_SERVICE_MUST_NOT_REDEFINE_MEMBERSHIP=true
```

| Concern | Layer |
|---------|--------|
| Active template / company / access SQL | Repository |
| Draft + submitted_at IS NULL | Repository |
| Periodic OpenAt / legacy cycle_start | Repository |
| Irregular (no OpenAt gate) | Repository |
| Due resolve | Service |
| UPCOMING / DUE_SOON / OVERDUE | Service |
| PENDING_CONFIRM / DONE presentation | Service |
| Request filters / pagination | Service |

```text
SERVICE_PERIODICITY_AWARENESS_ADDED=false
OPEN_AT_RECHECK_IN_SERVICE=false
SUBMITTED_AT_MEMBERSHIP_RECHECK_IN_SERVICE=false
CONFIRMED_DRAFT_CAN_REMAIN_ACTIVE_OBLIGATION=true
  (DONE = alert ack ≠ Company Submit)
```
