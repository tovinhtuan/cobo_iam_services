# 03 — Business contract drift review

| # | Question | Verdict |
|---|----------|---------|
| A | Alert still per occurrence? | PASS |
| B | Requires disclosure_record? | PASS |
| C | Periodic AlertFrom=OpenAt? | PASS |
| D | +7d worker window ignored for alert? | PASS |
| E | Draft eligible? | PASS |
| F | Submitted excluded? | PASS |
| G | Internal workflow excluded? | PASS |
| H | ApplicableFrom indirect only? | PASS |
| I | Reminder separate? | PASS |
| J | No dedicated alert table? | PASS |

```text
CONTRACT_DRIFT_COUNT=0
DEADLINE_ALERT_V1_BUSINESS_CONTRACT_VERIFIED=true
```

Summary:

```text
Periodic obligation visible when:
  occurrence exists + Draft + submitted_at NULL + OpenAt/cycle_start reached + due resolvable.
Disappears when Company Submit succeeds.
Internal review delay does not create Company OVERDUE.
```
