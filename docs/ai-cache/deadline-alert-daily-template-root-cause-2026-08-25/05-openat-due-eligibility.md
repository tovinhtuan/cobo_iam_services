# 05 — OpenAt / Due / repository eligibility

Upstream cycle/record absent → alert membership **NOT_REACHED**.

```text
ALERT_FROM=null
ALERT_WINDOW_STARTED=NOT_REACHED
RESOLVED_DUE=null
RESOLVED_DUE_SOURCE=null
CHECK_13_DUE_RESOLVED=NOT_REACHED
EXPECTED_REPOSITORY_ELIGIBLE=false (no Draft row to select)
REPOSITORY_MEMBERSHIP_DB_PROOF=NOT_REACHED
```

Truth table (conceptual for this template):

| Predicate              | Value        |
| ---------------------- | ------------ |
| status Draft           | no record    |
| submitted_at NULL      | no record    |
| active template        | true         |
| company/access         | N/A          |
| periodic cycle exists  | false        |
| AlertFrom <= TodayHCM  | N/A          |

Deadline Alert SQL/service correctly have nothing to return — not a Phase 1/2 membership bug for this case.