# 00 — Source reconciliation (Phase 1)

```text
TASK=Deadline Alert V1 Phase 1 — SQL membership + repository tests
DEADLINE_ALERT_V1_CONTRACT_LOCKED=true
APPLICATION_SOURCE_CHANGED=true (repository only)
```

## Actual symbols

```text
ACTUAL_REPOSITORY_PATH=internal/deadlinealerts/infra/mysql/repository.go
ACTUAL_LIST_FUNCTION=ListRows
ACTUAL_RECORD_ALIAS=dr (disclosure_records)
ACTUAL_PERIODIC_CYCLE_TABLE=periodic_cycles
ACTUAL_PERIODIC_JOIN_KEY=periodic_cycles.record_id = dr.record_id
MEMBERSHIP_HELPER=list_rows_membership.go (listRowsV1ObligationMembershipSQL)
```

## Current SQL (before Phase 1)

```text
CURRENT_SQL_STATUS_FILTER=LOWER(TRIM(dr.status)) <> 'draft'
CURRENT_SUBMITTED_FILTER=none
CURRENT_TEMPLATE_ACTIVE_FILTER=ListRowsActiveTemplateSQLJoin (INNER JOIN active_version_no > 0)
CURRENT_COMPANY_FILTER=dr.company_id = ?
CURRENT_ACCESS_FILTER=BuildListRowsScopeSQL(scope) appended
CURRENT_PERIODIC_JOIN=none
CURRENT_ORDER_BY=dr.created_at DESC
CURRENT_COUNT_BEHAVIOR=no separate count query; Go pages after full ListRows + filters
COUNT_QUERY_CHANGE=NOT_APPLICABLE
```

## Query shape

```text
BEFORE=FROM disclosure_records + workflow LEFT JOIN + active template INNER JOIN + confirmations LEFT JOIN
STRATEGY=MINIMAL_QUERY_DELTA
AFTER_PERIODIC=EXISTS / NOT EXISTS on periodic_cycles (no JOIN, no DISTINCT)
```

## TodayHCM

```text
TODAY_HCM_SOURCE=businessDateHCM(now) inside mysql Repository (Asia/Ho_Chi_Minh)
DB_SESSION_TIMEZONE_ASSUMED=false
INTERFACE_CHANGED=false
WithNow Option for deterministic tests
```

## Irregular vs periodic

```text
PERIODIC_RECORD_DETECTION=EXISTS periodic_cycles WHERE record_id = dr.record_id
IRREGULAR_RECORD_DETECTION=NOT EXISTS periodic_cycles for record_id
```

## Intentional intermediate state

```text
GO_DRAFT_FILTER_STILL_PRESENT=true (service.go isDraftRecordStatus skip)
PHASE_1_USER_VISIBLE_FEATURE_COMPLETE=false
```

## Helpers left unchanged (out of ListRows membership)

```text
listTaskAssigneeRecords / listCurrentStepMeta still use status <> draft
REASON=Phase 1 boundary; Draft rows may lack step/assignee enrichment until Phase 2 review
```
