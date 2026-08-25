# 03 — Repository test matrix

Authority: `list_rows_membership_test.go` + `list_rows_active_template_test.go`

| Contract flag | Coverage |
|---------------|----------|
| PERIODIC_DRAFT_OPENAT_PAST_INCLUDED | matrix + SQL source |
| PERIODIC_DRAFT_OPENAT_TODAY_INCLUDED | matrix |
| PERIODIC_DRAFT_OPENAT_FUTURE_EXCLUDED | matrix |
| LEGACY_PERIODIC_NULL_OPENAT_CYCLE_START_PAST_INCLUDED | matrix |
| LEGACY_PERIODIC_NULL_OPENAT_CYCLE_START_FUTURE_EXCLUDED | matrix |
| IRREGULAR_DRAFT_UNSUBMITTED_INCLUDED | matrix |
| DRAFT_SUBMITTED_EXCLUDED | matrix |
| NON_DRAFT_EXCLUDED | matrix (+ submitted_null case) |
| IRREGULAR_POST_SUBMIT_EXCLUDED | matrix |
| MALFORMED_PERIODIC_FAIL_SAFE | matrix |
| ACTIVE_TEMPLATE_FILTER_PRESERVED | list_rows_active_template_test + wiring test |
| COMPANY_SCOPE_PRESERVED | wiring test |
| ACCESS_SCOPE_PRESERVED | BuildListRowsScopeSQL still wired |
| TESTS_USE_FIXED_BUSINESS_DATE | today=2026-08-25; businessDateHCM fixed clock |

```text
LIST_ROWS_DIRECT_SQL_LIVE_MYSQL=false (no sqlmock in module)
MEMBERSHIP_EVALUATOR=listRowsV1MembershipEligible mirrors SQL for deterministic matrix
SQL_SHAPE_ASSERTED=source inspection of ListRows + listRowsV1ObligationMembershipSQL
```

```text
REPOSITORY_DUE_BUSINESS_LOGIC_ADDED=false
```
