# 04 — Periodic API matrix

Source: `api-summary.json` + recheck by record_id.

| Scenario | Expected | Actual | Gate |
|----------|----------|--------|------|
| A No occurrence | no alert for missing record | DEV empty cycles baseline; Periodicity unit covers AF | TEST_ONLY_PROVEN / NOT_APPLICABLE for live AF |
| B Pre-OpenAt Draft | absent | …f22c absent | PASS |
| C OpenAt reached + future due | present UPCOMING | …f413 UPCOMING | PASS |
| D Overdue unsubmitted | present OVERDUE | …f2cb OVERDUE | PASS |
| Due today | DUE_SOON | …f36b DUE_SOON | PASS |
| F Irregular Draft | present (no OpenAt) | …f4c1 OVERDUE | PASS |
| Duplicate rows | 0 | 0 | PASS |
| HTTP | 200 | 200 | PASS |
| Unexpected 5xx | 0 | 0 | PASS |

```text
API_RETURNS_ACTIONABLE_PERIODIC_DRAFT=PASS
PERIODIC_UNSUBMITTED_OVERDUE=PASS
PRE_OPENAT_PERIODIC_HIDDEN=PASS
IRREGULAR_ALERT_REGRESSION=PASS
DUPLICATE_ALERT_ROWS=0
UNEXPECTED_5XX=0
MATERIALIZATION_LOOKAHEAD_IS_ALERT_POLICY=false
```

First-run orphan …65ea (Draft, **no** cycle) appears as irregular UPCOMING — not a Pre-OpenAt failure.
