# 09 — Deadline Alert projection

Alert ≠ new record. Periodic AlertFrom = OpenAt (OpenAt<=TodayHCM) + Draft + submitted_at NULL.

```text
PROJECTED_NEW_DISCLOSURE_RECORDS≈52 (if snapshot OK)
PROJECTED_NEW_ALERT_ELIGIBLE_RECORDS=28
  OPENAT_NOT_YET (record but no alert)=24  # yearly OpenAt 2026-09-01; monthly day=31 OpenAt 2026-08-31
PROJECTED_UPCOMING_ALERTS=4   # target DAILY × 4 companies (Due≈2026-08-29..31)
PROJECTED_DUE_SOON_ALERTS=0   # approx; depends exact working-day Due
PROJECTED_OVERDUE_ALERTS=24   # monthly/quarterly current-slot late catch-up
PROJECTED_IMMEDIATE_OVERDUE_RECORDS=24
PROJECTED_ALERT_COMPANIES_AFFECTED=4
```

Target daily:

```text
T=OpenAt=2026-08-25; deadline_days=5 working → Due ~2026-08-29..31
Alert expected=true; status≈UPCOMING
```

Full matrix: `82-impact-matrix.tsv` (52 rows).
