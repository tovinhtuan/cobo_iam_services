# 09 — Alerts

DB eligibility proxy (new Draft+OpenAt):

```text
open_reached=28 pre_open=24 overdue_ish=24 upcoming_ish=4
ACTUAL_NEW_ALERTS≈28 (global projection)
ACTUAL_NEW_OVERDUE_ALERTS≈24
ALERT_DELTA_GUARD=PASS
OVERDUE_DELTA_GUARD=PASS
EXPECTED_OVERDUE_CATCHUP=true
```

c_001 API GET /company/deadline-alerts HTTP 200:

```text
total=17 (company-scoped)
STATUS: UPCOMING=4 OVERDUE=11 DUE_SOON=2
TARGET record present UPCOMING
```
