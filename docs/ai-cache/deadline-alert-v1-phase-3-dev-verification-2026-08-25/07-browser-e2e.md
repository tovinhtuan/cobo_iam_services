# 07 — Browser E2E

```text
ROUTE=/app/deadlines (Cảnh báo về thời hạn)
ACTOR=admin.dn@example.com @ c_001
FE_SOURCE_CHANGED=false
```

Screenshots: `screenshots/01-deadlines-list.png`, `03-deadlines-by-id.png`, `05-draft-detail.png`

## Results

```text
BROWSER_RENDERS_ACTIONABLE_DRAFT=PASS
  (body contains due 2026-08-20 / 2026-08-25 / 2026-09-10; network payload includes overdue record_id)
BROWSER_PRE_OPENAT_HIDDEN=PASS
BROWSER_POST_SUBMIT_ALERT_REMOVED=PASS
DRAFT_ALERT_NAVIGATION=PASS
  (direct /app/disclosures/{draft_id} opens; card click on list did not change URL — detail route OK)
CONFIRMATION_BROWSER_REGRESSION=NOT_REQUIRED_TEST_COVERED_LOCALLY
```

```text
CROSS_COMPANY_LEAK=0
  user@example.com → select c_002 → GET deadline-alerts HTTP 403 (no c_001 QA ids leaked)
CROSS_COMPANY_ACCESS=PASS (authorized c_001 200; unauthorized/other company no leak)
```
